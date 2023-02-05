//
// Copyright 2019-2023 Nestybox, Inc.
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

package idMap

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
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nestybox/sysbox-libs/linuxUtils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// The following are filesystems and host directories where we never ID-map
// mount as it causes functional problems (i.e., the kernel does not yet support
// ID-mapped mounts over them).

var idMapMountFsBlackList = []int64{
	unix.TMPFS_MAGIC,
	unix.BTRFS_SUPER_MAGIC,
	0x65735546, // unix.FUSE_SUPER_MAGIC
	0x6a656a63, // FAKEOWNER (Docker Desktop's Linux VM only)
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
func IDMapMount(usernsPath, mountPath string, unmountFirst bool) error {

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
	} else {
		mountPath, err = filepath.EvalSymlinks(mountPath)
		if err != nil {
			return fmt.Errorf("Failed to eval symlink on %s: %s", mountPath, err)
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
	if unmountFirst {
		err = unix.Unmount(mountPath, unix.MNT_DETACH)
		if err != nil {
			return fmt.Errorf("Failed to unmount %s: %s", mountPath, err)
		}
	}

	// Attach the clone to the to mount point
	err = moveMount(fdTree, "", -1, mountPath, C.MOVE_MOUNT_F_EMPTY_PATH)
	if err != nil {
		return fmt.Errorf("Failed to move mount: %s", err)
	}

	unix.Close(fdTree)
	return nil
}

// IDMapMountSupported checks if ID-mapping is supported on the host.
func IDMapMountSupported(dir string) (bool, error) {

	// ID-Mapped mounts requires Linux kernel >= 5.12
	kernelOK, err := checkKernelVersion(5, 12)
	if err != nil {
		return false, err
	}

	if !kernelOK {
		return false, nil
	}

	return runIDMapMountCheckOnHost(dir, false)
}

// OverlayfsOnIDMapMountSupported checks if overlayfs over ID-mapped lower
// layers is supported on the host.
func OverlayfsOnIDMapMountSupported(dir string) (bool, error) {

	// overlayfs on ID-mapped lower layers requires Linux kernel >= 5.19
	kernelOK, err := checkKernelVersion(5, 19)
	if err != nil {
		return false, err
	}

	if !kernelOK {
		return false, nil
	}

	return runIDMapMountCheckOnHost(dir, true)
}

// runIDMapMountCheckOnHost runs a quick test on the host to check if ID-mapping is
// supported. dir is the path where the test will run. If checkOnOverlayfs
// is true, the test checks if overlayfs supports ID-mapped lower layers.
func runIDMapMountCheckOnHost(dir string, checkOnOverlayfs bool) (bool, error) {
	var (
		lowerDir, upperDir, workDir, idMapDir string
	)

	tmpDir, err := os.MkdirTemp(dir, "sysbox-ovfs-check")
	if err != nil {
		return false, err
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	testDir := filepath.Join(tmpDir, "merged")
	if err := os.Mkdir(testDir, 0700); err != nil {
		return false, err
	}

	if checkOnOverlayfs {
		lowerDir = filepath.Join(tmpDir, "lower")
		upperDir = filepath.Join(tmpDir, "upper")
		workDir = filepath.Join(tmpDir, "work")

		dirs := []string{lowerDir, upperDir, workDir}
		for _, dir := range dirs {
			if err := os.Mkdir(dir, 0700); err != nil {
				return false, err
			}
		}
	}

	// Create a userns process that simply pauses until killed
	execFunc := func() {
		for {
			syscall.Syscall6(uintptr(unix.SYS_PAUSE), 0, 0, 0, 0, 0, 0)
		}
	}

	idmap := &specs.LinuxIDMapping{
		ContainerID: 0,
		HostID:      0,
		Size:        1,
	}

	pid, cleanupFunc, err := linuxUtils.CreateUsernsProcess(idmap, execFunc, testDir, false)
	if err != nil {
		return false, err
	}
	defer cleanupFunc()

	// Create the ID mapped mount associated with the child process user-ns
	usernsPath := fmt.Sprintf("/proc/%d/ns/user", pid)

	if checkOnOverlayfs {
		idMapDir = lowerDir
	} else {
		idMapDir = testDir
	}

	if err := IDMapMount(usernsPath, idMapDir, false); err != nil {
		return false, errors.Wrap(err, "create mapped mount")
	}
	defer unix.Unmount(idMapDir, unix.MNT_DETACH)

	if checkOnOverlayfs {
		opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
		flags := uintptr(0)
		if err := unix.Mount("overlay", testDir, "overlay", flags, opts); err != nil {
			return false, err
		}
		unix.Unmount(testDir, unix.MNT_DETACH)
		return true, nil
	}

	return true, nil
}

// Checkf if the dir at the given path can be ID-mapped based on the underlying
// filesystem.
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
