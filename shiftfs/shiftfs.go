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

package shiftfs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/nestybox/sysbox-libs/linuxUtils"
	"github.com/nestybox/sysbox-libs/mount"
	"github.com/nestybox/sysbox-libs/utils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	setxid "gopkg.in/hlandau/service.v1/daemon/setuid"
)

const SHIFTFS_MAGIC int64 = 0x6a656a62

// Describes a shiftfs mount point
type MountPoint struct {
	Source   string
	Readonly bool
}

// Mark performs a shiftfs mark-mount for path on the given markPath
// (e.g., Mark("/a/b", "/c/d") causes "b" to be mounted on "d" and
// "d" to have a shiftfs mark).
func Mark(path, markPath string) error {
	if err := unix.Mount(path, markPath, "shiftfs", 0, "mark"); err != nil {
		return fmt.Errorf("failed to mark shiftfs on %s at %s: %v", path, markPath, err)
	}
	return nil
}

// Mount performs a shiftfs mount on the given path; the path must have a
// shiftfs mark on it already (e.g., Mount("/c/d", "/x/y") requires that
// "d" have a shiftfs mark on it and causes "d" to be mounted on "y" and
// "y" to have a shiftfs mount).
func Mount(path, mntPath string) error {
	if err := unix.Mount(path, mntPath, "shiftfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount shiftfs on %s at %s: %v", path, mntPath, err)
	}
	return nil
}

// Unmount perform a shiftfs unmount on the given path. The path must have
// a shiftfs mark or mount on it.
func Unmount(path string) error {
	if err := unix.Unmount(path, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("failed to unmount %s: %v", path, err)
	}
	return nil
}

// Returns a boolean indicating if the given path has a shiftfs mount
// on it (mark or actual mount).
func Mounted(path string, mounts []*mount.Info) (bool, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}

	return mount.MountedWithFs(realPath, "shiftfs", mounts)
}

// ShiftfsSupported checks if shiftfs is supported on the host.
func ShiftfsSupported(dir string) (bool, error) {
	logrus.Debugf("Running shiftfs check on host.")
	return runShiftfsCheckOnHost(dir, false)
}

// ShiftfsSupported checks if shiftfs-on-overlayfs is supported on the host.
func ShiftfsSupportedOnOverlayfs(dir string) (bool, error) {
	logrus.Debugf("Running shiftfs-on-overlayfs check on host.")
	return runShiftfsCheckOnHost(dir, true)
}

// runShiftfsCheckOnHost runs a quick test on the host to check if shiftfs is
// supported. dir is the path where the test will run, and checkOnOverlayfs
// indicates if the test should check shiftfs-on-overlayfs.
func runShiftfsCheckOnHost(dir string, checkOnOverlayfs bool) (bool, error) {
	usernsUid := 165536

	shiftfsModPresent, err := linuxUtils.KernelModSupported("shiftfs")
	if err != nil {
		return false, err
	}

	if !shiftfsModPresent {
		return false, nil
	}

	logrus.Debugf("- shiftfs check: found shiftfs module.")

	fsName, err := utils.GetFsName(dir)
	if err != nil {
		return false, err
	}

	if fsName == "overlayfs" || fsName == "tmpfs" {
		return false, fmt.Errorf("test dir (%s) must not be on overlayfs or tmpfs", dir)
	}

	tmpDir, err := os.MkdirTemp(dir, "sysbox-shiftfs-check")
	if err != nil {
		return false, err
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	if err := os.Chmod(tmpDir, 0755); err != nil {
		return false, err
	}

	testDir := filepath.Join(tmpDir, "test")
	if err := os.Mkdir(testDir, 0755); err != nil {
		return false, err
	}

	if err := os.Chown(testDir, usernsUid, usernsUid); err != nil {
		return false, err
	}

	logrus.Debugf("- shiftfs check: test dir = %s (%s)", testDir, fsName)

	if checkOnOverlayfs {
		lowerDir := filepath.Join(tmpDir, "lower")
		upperDir := filepath.Join(tmpDir, "upper")
		workDir := filepath.Join(tmpDir, "work")

		dirs := []string{lowerDir, upperDir, workDir}
		for _, dir := range dirs {
			if err := os.Mkdir(dir, 0755); err != nil {
				return false, err
			}
		}

		opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
		flags := uintptr(0)
		if err := unix.Mount("overlay", testDir, "overlay", flags, opts); err != nil {
			return false, err
		}
		defer unix.Unmount(testDir, unix.MNT_DETACH)

		logrus.Debugf("- shiftfs check: mounted overlayfs on %s", testDir)
	}

	// Create the shiftfs mark on the test dir
	if err := Mark(testDir, testDir); err != nil {
		return false, err
	}
	defer Unmount(testDir)

	logrus.Debugf("- shiftfs check: marked shiftfs on %s", testDir)

	// Since shiftfs only makes sense within a user-ns, we will fork a child
	// process into a new user-ns and have it mount shiftfs and verify it
	// works. execFunc is the function the child will execute.
	execFunc := func() {
		logrus.Debugf("- shiftfs check: execFunc: running")

		logrus.Debugf("- shiftfs check: execFunc: lock OS thread")
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Make ourselves root within the user ns
		logrus.Debugf("- shiftfs check: execFunc: setresuid")
		if err := setxid.Setresuid(0, 0, 0); err != nil {
			logrus.Debugf("- shiftfs check: execFunc: failed: %v", err)
			os.Exit(1)
		}
		logrus.Debugf("- shiftfs check: execFunc: setresgid")
		if err := setxid.Setresgid(0, 0, 0); err != nil {
			logrus.Debugf("- shiftfs check: execFunc: failed: %v", err)
			os.Exit(1)
		}

		logrus.Debugf("- shiftfs check: execFunc: mounting shiftfs on %s", testDir)
		if err := Mount(testDir, testDir); err != nil {
			logrus.Debugf("- shiftfs check: execFunc: failed: %v", err)
			os.Exit(2)
		}

		testfile := filepath.Join(testDir, "testfile")
		testfile2 := filepath.Join(testDir, "testfile2")

		logrus.Debugf("- shiftfs check: execFunc: creating file %s", testfile)
		_, err := os.Create(testfile)
		if err != nil {
			logrus.Debugf("- shiftfs check: execFunc: failed: %v", err)
			os.Exit(3)
		}

		// This operation will fail with EOVERFLOW if shiftfs is buggy in the kernel
		logrus.Debugf("- shiftfs check: execFunc: renaming file %s to %s", testfile, testfile2)
		if err := os.Rename(testfile, testfile2); err != nil {
			logrus.Debugf("- shiftfs check: execFunc: failed: %v", err)
			os.Remove(testfile)
			os.Exit(4)
		}

		logrus.Debugf("- shiftfs check: execFunc: removing file %s", testfile2)
		os.Remove(testfile2)

		logrus.Debugf("- shiftfs check: execFunc: success")
		os.Exit(0)
	}

	// Fork the child process into a new user-ns (and mount-ns too)
	idmap := &specs.LinuxIDMapping{
		ContainerID: 0,
		HostID:      uint32(usernsUid),
		Size:        65536,
	}

	pid, _, err := linuxUtils.CreateUsernsProcess(idmap, execFunc, testDir, true, false)
	if err != nil {
		return false, err
	}

	logrus.Debugf("- shiftfs check: spawning child process (%d) into user-ns", pid)

	// Wait for the child process to exit
	var wstatus syscall.WaitStatus
	var rusage syscall.Rusage

	_, err = syscall.Wait4(pid, &wstatus, 0, &rusage)
	if err != nil {
		return false, err
	}

	if !wstatus.Exited() {
		logrus.Debugf("- shiftfs check: child process (%d) did not exit normally", pid)
		return false, fmt.Errorf("child process did not exit normally")
	}

	exitStatus := wstatus.ExitStatus()

	if exitStatus != 0 {
		logrus.Debugf("- shiftfs check: child process failed (exit status = %d)", exitStatus)
		return false, nil
	}

	logrus.Debugf("- shiftfs check: passed")
	return true, nil
}
