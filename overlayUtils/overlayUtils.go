//
// Copyright 2023 Nestybox, Inc.
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

// Utilities for dealing with Linux's overlay fs

package overlayUtils

import (
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set"
	"github.com/nestybox/sysbox-libs/mount"
	"golang.org/x/sys/unix"
)

type MountOpts struct {
	Opts      string
	Flags     int
	PropFlags int
}

// GetMountOpt returns the mount options string, mount flags, and mount
// propagation flags of the overlayfs mount at the given path.
func GetMountOpt(mi *mount.Info) *MountOpts {

	currMntOpts := mapset.NewSet()
	for _, opt := range strings.Split(mi.Opts, ",") {
		currMntOpts.Add(opt)
	}

	currVfsOpts := mapset.NewSet()
	for _, opt := range strings.Split(mi.VfsOpts, ",") {
		currVfsOpts.Add(opt)
	}

	// The vfs opts reported by mountinfo are a combination of per superblock
	// mount opts and the overlayfs-specific data; we need to separate these so
	// we can do the mount properly.
	properMntOpts := mapset.NewSetFromSlice([]interface{}{
		"ro", "rw", "nodev", "noexec", "nosuid", "noatime", "nodiratime", "relatime", "strictatime", "sync",
	})

	newMntOpts := currVfsOpts.Intersect(properMntOpts)
	newVfsOpts := currVfsOpts.Difference(properMntOpts)

	// Convert the mount options to the mount flags
	newMntOptsString := []string{}
	for _, opt := range newMntOpts.ToSlice() {
		newMntOptsString = append(newMntOptsString, fmt.Sprintf("%s", opt))
	}
	mntFlags := mount.OptionsToFlags(newMntOptsString)

	// Convert the vfs option set to the mount data string
	newVfsOptsString := ""
	for i, opt := range newVfsOpts.ToSlice() {
		if i != 0 {
			newVfsOptsString += ","
		}
		newVfsOptsString += fmt.Sprintf("%s", opt)
	}

	// Get the mount propagation flags
	propFlags := 0

	if strings.Contains(mi.Optional, "shared") {
		propFlags |= unix.MS_SHARED
	} else if strings.Contains(mi.Optional, "master") {
		propFlags |= unix.MS_SLAVE
	} else if strings.Contains(mi.Optional, "unbindable") {
		propFlags |= unix.MS_UNBINDABLE
	} else {
		propFlags |= unix.MS_PRIVATE
	}

	mntOpts := &MountOpts{
		Opts:      newVfsOptsString,
		Flags:     mntFlags,
		PropFlags: propFlags,
	}

	return mntOpts
}

func GetLowerLayers(mntOpts *MountOpts) []string {
	lowerStr := ""
	opts := strings.Split(mntOpts.Opts, ",")
	for _, opt := range opts {
		if strings.HasPrefix(opt, "lowerdir=") {
			lowerStr = strings.TrimPrefix(opt, "lowerdir=")
			break
		}
	}

	return strings.Split(lowerStr, ":")
}

func GetUpperLayer(mntOpts *MountOpts) string {
	opts := strings.Split(mntOpts.Opts, ",")
	for _, opt := range opts {
		if strings.HasPrefix(opt, "upperdir=") {
			return strings.TrimPrefix(opt, "upperdir=")
		}
	}
	return ""
}
