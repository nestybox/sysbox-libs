//
// Copyright 2019-2021 Nestybox, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Utilities for shifting user and group IDs on the file system using chown
// (e.g., shifting uids:gids from range [0:65536] to range [165536:231071]).

package idShiftUtils

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/joshlf/go-acl"
	aclLib "github.com/joshlf/go-acl"
	"github.com/karrick/godirwalk"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	mapset "github.com/deckarep/golang-set"
)

type IDShiftType int

const (
	NoShift IDShiftType = iota
	Shiftfs
	IDMappedMount
	IDMappedMountOrShiftfs
	Chown
)

type aclType int

const (
	aclTypeAccess aclType = iota
	aclTypeDefault
)

type IDMapping struct {
	ContainerID uint32
	HostID      uint32
	Size        uint32
}

// checkACLSupport attempts to set an extended ACL attribute on a file to check ACL support.
func checkACLSupport(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    // Try setting an extended attribute specific to ACLs
    err = unix.Fsetxattr(int(file.Fd()), "system.posix_acl_access", []byte{}, 0)

    // ENOTSUP means ACL is not supported; any other error indicates something
    // else went wrong, so we assume ACLs are supported
    return err != unix.ENOTSUP
}

// shiftAclType shifts the ACL type user and group IDs by the given offset
func shiftAclType(aclT aclType, path string, uidOffset, gidOffset int32) error {
	var facl aclLib.ACL
	var err error

	// Read the ACL
	if aclT == aclTypeDefault {
		facl, err = acl.GetDefault(path)
	} else {
		facl, err = acl.Get(path)
	}

	if err != nil {
		return fmt.Errorf("failed to get ACL for %s: %s", path, err)
	}

	// Shift the user and group ACLs (if any)
	newACL := aclLib.ACL{}
	aclShifted := false

	for _, e := range facl {

		// ACL_USER id shifting
		if e.Tag == aclLib.TagUser {
			uid, err := strconv.ParseUint(e.Qualifier, 10, 32)
			if err != nil {
				logrus.Warnf("failed to convert ACL qualifier for %v: %s", e, err)
				continue
			}

			targetUid := uint64(int32(uid) + uidOffset)
			e.Qualifier = strconv.FormatUint(targetUid, 10)
			aclShifted = true
		}

		// ACL_GROUP id shifting
		if e.Tag == aclLib.TagGroup {
			gid, err := strconv.ParseUint(e.Qualifier, 10, 32)
			if err != nil {
				logrus.Warnf("failed to convert ACL qualifier %v: %s", e, err)
				continue
			}

			targetGid := uint64(int32(gid) + gidOffset)
			e.Qualifier = strconv.FormatUint(targetGid, 10)
			aclShifted = true
		}

		newACL = append(newACL, e)
	}

	// Write back the modified ACL
	if aclShifted {
		if aclT == aclTypeDefault {
			err = acl.SetDefault(path, newACL)
		} else {
			err = acl.Set(path, newACL)
		}
		if err != nil {
			return fmt.Errorf("failed to set ACL %v for %s: %s", newACL, path, err)
		}
	}

	return nil
}

// Shifts the ACL user and group IDs by the given offset, both for access and default ACLs
func shiftAclIds(path string, isDir bool, uidOffset, gidOffset int32) error {

	// Access list
	err := shiftAclType(aclTypeAccess, path, uidOffset, gidOffset)
	if err != nil {
		return err
	}

	// Default list (for directories only)
	if isDir {
		err = shiftAclType(aclTypeDefault, path, uidOffset, gidOffset)
		if err != nil {
			return err
		}
	}

	return nil
}

// "Shifts" ownership of user and group IDs on the given directory and files and directories
// below it by the given offset, using chown.
func ShiftIdsWithChown(baseDir string, uidOffset, gidOffset int32) error {

	aclSupported := checkACLSupport(baseDir)

	hardLinks := []uint64{}
	err := godirwalk.Walk(baseDir, &godirwalk.Options{
		Callback: func(path string, de *godirwalk.Dirent) error {

			// When doing the chown, we don't follow symlinks as we want to change
			// the ownership of the symlinks themselves. We will chown the
			// symlink's target during the godirwalk (unless the symlink is
			// dangling in which case there is nothing to be done).

			fi, err := os.Lstat(path)
			if err != nil {
				return err
			}

			st, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("failed to convert to syscall.Stat_t")
			}

			// If a file has multiple hardlinks, change its ownership once
			if st.Nlink >= 2 {
				for _, linkInode := range hardLinks {
					if linkInode == st.Ino {
						return nil
					}
				}

				hardLinks = append(hardLinks, st.Ino)
			}

			targetUid := int32(st.Uid) + uidOffset
			targetGid := int32(st.Gid) + gidOffset

			err = unix.Lchown(path, int(targetUid), int(targetGid))
			if err != nil {
				return fmt.Errorf("chown %s to %d:%d failed: %s", path, targetUid, targetGid, err)
			}

			// chown will turn-off the set-user-ID and set-group-ID bits on files,
			// so we need to restore them.
			fMode := fi.Mode()
			setuid := fMode&os.ModeSetuid == os.ModeSetuid
			setgid := fMode&os.ModeSetgid == os.ModeSetgid

			if fMode.IsRegular() && (setuid || setgid) {
				if err := os.Chmod(path, fMode); err != nil {
					return fmt.Errorf("chmod %s to %s failed: %s", path, fMode, err)
				}
			}

			// Chowning the file is not sufficient; we also need to shift user and group IDs in
			// the Linux access control list (ACL) for the file
			if fMode&os.ModeSymlink == 0 && aclSupported {
				if err := shiftAclIds(path, fi.IsDir(), uidOffset, gidOffset); err != nil {
					return fmt.Errorf("failed to shift ACL for %s: %s", path, err)
				}
			}

			return nil
		},

		ErrorCallback: func(path string, err error) godirwalk.ErrorAction {

			fi, err := os.Lstat(path)
			if err != nil {
				return godirwalk.Halt
			}

			// Ignore errors due to chown on dangling symlinks (they often occur in container image layers)
			if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
				return godirwalk.SkipNode
			}

			return godirwalk.Halt
		},

		Unsorted: true, // Speeds up the directory tree walk
	})

	return err
}

// Returns the lists of user and group IDs for all files and directories at or
// below the given path.
func GetDirIDs(baseDir string) ([]uint32, []uint32, error) {

	uidSet := mapset.NewSet()
	gidSet := mapset.NewSet()

	err := godirwalk.Walk(baseDir, &godirwalk.Options{
		Callback: func(path string, de *godirwalk.Dirent) error {

			fi, err := os.Lstat(path)
			if err != nil {
				return err
			}

			st, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("failed to convert to syscall.Stat_t")
			}

			uidSet.Add(st.Uid)
			gidSet.Add(st.Gid)

			return nil
		},

		Unsorted: true, // Speeds up the directory tree walk
	})

	if err != nil {
		return nil, nil, err
	}

	uidList := []uint32{}
	for _, id := range uidSet.ToSlice() {
		val := id.(uint32)
		uidList = append(uidList, val)
	}

	gidList := []uint32{}
	for _, id := range gidSet.ToSlice() {
		val := id.(uint32)
		gidList = append(gidList, val)
	}

	return uidList, gidList, nil
}
