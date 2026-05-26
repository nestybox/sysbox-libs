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

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nestybox/sysbox-libs/linuxUtils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	logrus "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// The following are filesystems and host directories where we never ID-map
// mount as it causes functional problems (i.e., the kernel does not yet support
// ID-mapped mounts over them).
//
// TODO: remove this blacklist and instead run experiments on each fs

var idMapMountFsBlackList = []int64{
	unix.OVERLAYFS_SUPER_MAGIC, // can't id-map on top of an overlayfs mount
	0x65735546,                 // unix.FUSE_SUPER_MAGIC
	0x6a656a63,                 // FAKEOWNER (Docker Desktop's Linux VM only)
}

var idMapMountDevBlackList = []string{"/dev/null"}

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
	fdTree, err := unix.OpenTree(-1, mountPath, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH|unix.AT_RECURSIVE)
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
	err = unix.MoveMount(fdTree, "", -1, mountPath, unix.MOVE_MOUNT_F_EMPTY_PATH)
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

	return runIDMapMountCheckOnHost(dir, ovfsCheckNone)
}

// OverlayfsOnIDMapUpperSupported checks whether the kernel supports id-mapped
// mounts on the overlayfs upperdir and workdir.
//
// dir must be a writable path on the same filesystem that will host real
// container upper layers (e.g. the containerd/docker snapshotter root).
//
// Returns (true, nil) if a throwaway overlayfs mount with id-mapped
// upperdir+workdir succeeds; (false, nil) for supported negative results
// (kernel too old, EINVAL/EOPNOTSUPP from mount); (false, err) on
// unexpected I/O errors.
func OverlayfsOnIDMapUpperSupported(dir string) (bool, error) {

	// Id-mapped overlayfs upperdir requires Linux kernel >= 5.19.
	// (Same floor as the existing lower-layer idmap support.)
	kernelOK, err := checkKernelVersion(5, 19)
	if err != nil {
		return false, err
	}
	if !kernelOK {
		return false, nil
	}

	return runIDMapMountCheckOnHost(dir, ovfsCheckUpper)
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

	return runIDMapMountCheckOnHost(dir, ovfsCheckLower)
}

// ovfsCheckMode controls which overlayfs id-map scenario runIDMapMountCheckOnHost probes.
type ovfsCheckMode int

const (
	ovfsCheckNone  ovfsCheckMode = iota // plain idmap bind-mount probe (no overlayfs)
	ovfsCheckLower                      // overlayfs with id-mapped lowerdir
	ovfsCheckUpper                      // overlayfs with id-mapped upperdir + workdir
)

// runIDMapMountCheckOnHost runs a quick test on the host to check if ID-mapping is
// supported. dir is the path where the test will run. mode controls what scenario
// is probed: plain idmap, idmap on overlayfs lower layers, or idmap on overlayfs
// upper+workdir layers.
func runIDMapMountCheckOnHost(dir string, mode ovfsCheckMode) (bool, error) {
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

	if mode == ovfsCheckLower || mode == ovfsCheckUpper {
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
		for i := 0; i < 3600; i++ {
			time.Sleep(1 * time.Second)
		}
	}

	idmap := &specs.LinuxIDMapping{
		ContainerID: 0,
		HostID:      0,
		Size:        1,
	}

	pid, childKill, err := linuxUtils.CreateUsernsProcess(idmap, execFunc, testDir, false, false)
	if err != nil {
		return false, err
	}

	defer func() {
		var wstatus syscall.WaitStatus
		var rusage syscall.Rusage
		childKill()
		syscall.Wait4(pid, &wstatus, 0, &rusage)
	}()

	// Create the ID mapped mount associated with the child process user-ns
	usernsPath := fmt.Sprintf("/proc/%d/ns/user", pid)

	switch mode {
	case ovfsCheckNone:
		idMapDir = testDir
		if err := IDMapMount(usernsPath, idMapDir, false); err != nil {
			return false, errors.Wrap(err, "create mapped mount")
		}
		defer unix.Unmount(idMapDir, unix.MNT_DETACH)
		return true, nil

	case ovfsCheckLower:
		idMapDir = lowerDir
		// IDMapMount failure here is propagated as a hard error rather than
		// "not supported". By the time we reach this probe the caller has already
		// confirmed IDMapMountSupported() returned true, so any failure on a
		// plain bind-mount clone indicates a genuine I/O or permission problem
		// rather than a kernel feature gap.
		if err := IDMapMount(usernsPath, idMapDir, false); err != nil {
			return false, errors.Wrap(err, "create mapped mount on lower")
		}
		defer unix.Unmount(idMapDir, unix.MNT_DETACH)

		opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
		if err := unix.Mount("overlay", testDir, "overlay", uintptr(0), opts); err != nil {
			return false, err
		}
		unix.Unmount(testDir, unix.MNT_DETACH)
		return true, nil

	case ovfsCheckUpper:
		// Id-map both upperdir and workdir against the same userns.
		// The kernel enforces that they share the same mnt_userns at
		// overlayfs mount time; mismatches produce EINVAL.
		//
		// Only EINVAL/EOPNOTSUPP/ENOTSUP indicate the kernel does not support
		// idmapped mounts on this filesystem type — those are treated as
		// "not supported" (false, nil). All other errors (EPERM, EBUSY, EIO,
		// …) are genuine failures and are propagated to the caller.
		if err := IDMapMount(usernsPath, upperDir, false); err != nil {
			if stderrors.Is(err, unix.EINVAL) ||
				stderrors.Is(err, unix.EOPNOTSUPP) ||
				stderrors.Is(err, unix.ENOTSUP) {
				logrus.Debugf("ovfsCheckUpper: upperdir idmap not supported on this fs: %v", err)
				return false, nil
			}
			return false, err
		}
		defer unix.Unmount(upperDir, unix.MNT_DETACH)

		if err := IDMapMount(usernsPath, workDir, false); err != nil {
			if stderrors.Is(err, unix.EINVAL) ||
				stderrors.Is(err, unix.EOPNOTSUPP) ||
				stderrors.Is(err, unix.ENOTSUP) {
				logrus.Debugf("ovfsCheckUpper: workdir idmap not supported on this fs: %v", err)
				return false, nil
			}
			return false, err
		}
		defer unix.Unmount(workDir, unix.MNT_DETACH)

		opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
		if err := unix.Mount("overlay", testDir, "overlay", uintptr(0), opts); err != nil {
			// Kernel rejected overlayfs with id-mapped upper/workdir.
			// Treat EINVAL, EOPNOTSUPP, ENOTSUP as "not supported";
			// propagate genuine I/O errors.
			if stderrors.Is(err, unix.EINVAL) ||
				stderrors.Is(err, unix.EOPNOTSUPP) ||
				stderrors.Is(err, unix.ENOTSUP) {
				return false, nil
			}
			return false, err
		}
		unix.Unmount(testDir, unix.MNT_DETACH)
		return true, nil
	}

	return false, fmt.Errorf("unknown ovfsCheckMode %d", mode)
}

// Checkf if the dir at the given path can be ID-mapped based on the underlying filesystem.
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

	// ID-mapped mounts on tmpfs supported since kernel 6.3
	// Ref: https://lore.kernel.org/lkml/20230217080552.1628786-1-brauner@kernel.org/

	if fs.Type == unix.TMPFS_MAGIC {
		cmp, err := linuxUtils.KernelCurrentVersionCmp(6, 3)
		if err != nil {
			return false, fmt.Errorf("failed to compare kernel version: %v", err)
		}
		if cmp < 0 {
			return false, nil
		}
	}

	return true, nil
}
