//
// Copyright 2019-2022 Nestybox, Inc.
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

//go:build linux && idmapped_mnt && cgo
// +build linux,idmapped_mnt,cgo

package idShiftUtils

// #define _GNU_SOURCE
// #include <errno.h>
// #include <fcntl.h>
// #include <getopt.h>
// #include <linux/mount.h>
// #include <linux/types.h>
// #include <stdbool.h>
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// #include <sys/syscall.h>
// #include <unistd.h>
//
// static inline int
// open_tree(int dirfd, const char *filename, unsigned int flags)
// {
//     return syscall(SYS_open_tree, dirfd, filename, flags);
// }
//
// static inline int
// move_mount(int from_dirfd, const char *from_pathname,
//            int to_dirfd, const char *to_pathname, unsigned int flags)
// {
//     return syscall(SYS_move_mount, from_dirfd, from_pathname,
//                    to_dirfd, to_pathname, flags);
// }
import "C"

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The following are filesystems and host directories where we never ID-map
// mount as it causes functional problems (i.e., the kernel does not yet support
// ID-mapped mounts over them).

var idMapMountFsBlackList = []int64{
	unix.OVERLAYFS_SUPER_MAGIC,
	unix.TMPFS_MAGIC,
	unix.BTRFS_SUPER_MAGIC,
}

var idMapMountDevBlackList = []string{"/dev/null"}

// Opens a mount tree (wrapper for open_tree() syscall).
func openTree(dirFd int, path string, flags uint) (int, error) {
	cPath := C.CString(path)

	fdTree, err := C.open_tree(C.int(dirFd), cPath, C.uint(flags))
	if err != nil {
		return -1, err
	}

	C.free(unsafe.Pointer(cPath))
	return int(fdTree), nil
}

// Moves a mount (wrapper for move_mount() syscall).
func moveMount(fromDirFd int, fromPath string, toDirFd int, toPath string, flags uint) error {
	cFromPath := C.CString(fromPath)
	cToPath := C.CString(toPath)

	_, err := C.move_mount(C.int(fromDirFd), cFromPath, C.int(toDirFd), cToPath, C.uint(flags))
	if err != nil {
		return err
	}

	C.free(unsafe.Pointer(cFromPath))
	C.free(unsafe.Pointer(cToPath))

	return nil
}

// ID-maps the given mountpoint, using the given userns ID mappings; both paths must be absolute.
func IDMapMount(usernsPath, mountPath string) error {

	// open the usernsPath
	usernsFd, err := os.Open(usernsPath)
	if err != nil {
		return fmt.Errorf("Failed to open %s: %s", usernsPath, err)
	}
	defer usernsFd.Close()

	// If mountPath is procfd based, read the magic link
	if strings.HasPrefix(mountPath, "/proc/self/fd/") {
		mountPath, err = os.Readlink(mountPath)
		if err != nil {
			return fmt.Errorf("Failed to read link %s: %s", mountPath, err)
		}
	}

	// clone the given mount
	fdTree, err := openTree(-1, mountPath,
		uint(C.OPEN_TREE_CLONE|C.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH|unix.AT_RECURSIVE))

	if err != nil {
		return fmt.Errorf("Failed to open mount at %s: %s", mountPath, err)
	}

	// Set the ID-mapped mount attribute on the clone
	// TODO: add propagation type? (use the original mountpoints propagation)?

	mountAttr := &unix.MountAttr{
		Attr_set:  unix.MOUNT_ATTR_IDMAP,
		Userns_fd: uint64(usernsFd.Fd()),
	}

	err = unix.MountSetattr(int(fdTree), "", unix.AT_EMPTY_PATH|unix.AT_RECURSIVE, mountAttr)
	if err != nil {
		return fmt.Errorf("Failed to set mount attr: %s", err)
	}

	// Unmount the original mountPath mount to prevent redundant / stacked mounting
	err = unix.Unmount(mountPath, unix.MNT_DETACH)
	if err != nil {
		return fmt.Errorf("Failed to unmount %s: %s", mountPath, err)
	}

	// Attach the clone to the to mount point
	err = moveMount(fdTree, "", -1, mountPath, C.MOVE_MOUNT_F_EMPTY_PATH)
	if err != nil {
		return fmt.Errorf("Failed to move mount: %s", err)
	}

	unix.Close(fdTree)
	return nil
}

func IDMapMountSupportedOnPath(path string) (bool, error) {
	var fs unix.Statfs_t

	for _, m := range idMapMountDevBlackList {
		if path == m {
			return false, nil
		}
	}

	err := unix.Statfs(path, &fs)
	if err != nil {
		return false, err
	}

	for _, name := range idMapMountFsBlackList {
		if fs.Type == name {
			return false, nil
		}
	}

	return true, nil
}
