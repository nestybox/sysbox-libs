//
// Copyright 2020 - 2023 Nestybox, Inc.
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

package linuxUtils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/afero"
	"golang.org/x/sys/unix"
)

// Afero FS for unit-testing purposes.
var appFs = afero.NewOsFs()

// Obtain system's linux distribution.
func GetDistro() (string, error) {

	distro, err := GetDistroPath("/")
	if err != nil {
		return "", err
	}

	return distro, nil
}

// Parse os-release lines looking for 'ID' field. Originally borrowed from
// acobaugh/osrelease lib and adjusted to extract only the os-release "ID"
// field.
func parseLineDistroId(line string) string {

	// Skip empty lines.
	if len(line) == 0 {
		return ""
	}

	// Skip comments.
	if line[0] == '#' {
		return ""
	}

	// Try to split string at the first '='.
	splitString := strings.SplitN(line, "=", 2)
	if len(splitString) != 2 {
		return ""
	}

	// Trim white space from key. Return here if we are not dealing
	// with an "ID" field.
	key := splitString[0]
	key = strings.Trim(key, " ")
	if key != "ID" {
		return ""
	}

	// Trim white space from value.
	value := splitString[1]
	value = strings.Trim(value, " ")

	// Handle double quotes.
	if strings.ContainsAny(value, `"`) {
		first := string(value[0:1])
		last := string(value[len(value)-1:])

		if first == last && strings.ContainsAny(first, `"'`) {
			value = strings.TrimPrefix(value, `'`)
			value = strings.TrimPrefix(value, `"`)
			value = strings.TrimSuffix(value, `'`)
			value = strings.TrimSuffix(value, `"`)
		}
	}

	// Expand anything else that could be escaped.
	value = strings.Replace(value, `\"`, `"`, -1)
	value = strings.Replace(value, `\$`, `$`, -1)
	value = strings.Replace(value, `\\`, `\`, -1)
	value = strings.Replace(value, "\\`", "`", -1)

	return value
}

// Obtain system's linux distribution in the passed rootfs.
func GetDistroPath(rootfs string) (string, error) {

	var (
		data []byte
		err  error
	)

	// As per os-release(5) man page both of the following paths should be taken
	// into account to find 'os-release' file.
	var osRelPaths = []string{
		filepath.Join(rootfs, "/etc/os-release"),
		filepath.Join(rootfs, "/usr/lib/os-release"),
	}

	for _, file := range osRelPaths {
		data, err = afero.ReadFile(appFs, file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")

		// Iterate through os-release lines looking for 'ID' content.
		for _, line := range lines {
			distro := parseLineDistroId(line)
			if distro != "" {
				return distro, nil
			}
		}
	}

	return "", err
}

// GetKernelRelease returns the kernel release (e.g., "4.18")
func GetKernelRelease() (string, error) {

	var utsname unix.Utsname

	if err := unix.Uname(&utsname); err != nil {
		return "", fmt.Errorf("uname: %v", err)
	}

	n := bytes.IndexByte(utsname.Release[:], 0)

	return string(utsname.Release[:n]), nil
}

// Compares the given kernel version versus the current kernel version. Returns
// 0 if versions are equal, 1 if the current kernel has higher version than the
// given one, -1 otherwise.
func KernelCurrentVersionCmp(k1Major, k1Minor int) (int, error) {

	rel, err := GetKernelRelease()
	if err != nil {
		return 0, err
	}

	splits := strings.SplitN(rel, ".", -1)
	if len(splits) < 2 {
		return 0, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	k2Major, err := strconv.Atoi(splits[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	k2Minor, err := strconv.Atoi(splits[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	if k2Major > k1Major {
		return 1, nil
	} else if k2Major == k1Major {
		if k2Minor > k1Minor {
			return 1, nil
		} else if k2Minor == k1Minor {
			return 0, nil
		}
	}

	return -1, nil
}

// Parses the kernel release string (obtained from GetKernelRelease()) and returns
// the major and minor numbers.
func ParseKernelRelease(rel string) (int, int, error) {
	var (
		major, minor int
		err          error
	)

	splits := strings.SplitN(rel, ".", -1)
	if len(splits) < 2 {
		return -1, -1, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	major, err = strconv.Atoi(splits[0])
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	minor, err = strconv.Atoi(splits[1])
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse kernel release %v", rel)
	}

	return major, minor, nil
}

// Obtain location of kernel-headers for a given linux distro.
func GetLinuxHeaderPath(distro string) (string, error) {

	var path string

	kernelRel, err := GetKernelRelease()
	if err != nil {
		return "", err
	}

	if distro == "redhat" || distro == "centos" || distro == "rocky" || distro == "almalinux" || distro == "fedora" || distro == "amzn" {
		path = filepath.Join("/usr/src/kernels", kernelRel)
	} else if distro == "arch" || distro == "flatcar" {
		path = filepath.Join("/lib/modules", kernelRel, "build")
	} else {
		// All other distros appear to be following the "/usr/src/linux-headers-rel"
		// naming convention.
		kernelHdr := "linux-headers-" + kernelRel
		path = filepath.Join("/usr/src", kernelHdr)
	}

	return path, nil
}

// KernelModSupported returns nil if the given module is loaded in the kernel.
func KernelModSupported(mod string) (bool, error) {

	// Load the module
	exec.Command("modprobe", mod).Run()

	// Check if the module is in the kernel
	filename := "/proc/modules"

	f, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), mod) {
			return true, nil
		}
	}
	if err := s.Err(); err != nil {
		return false, fmt.Errorf("failed to read %s: %s", filename, err)
	}

	return false, nil
}

// CreateUsernsProcess forks the current process into a new Linux
// user-namespace, using the given the ID mapping (common to both uid and
// gid). Returns the pid of the new process and a "kill" function (so that the
// caller can kill the child when desired). The new process executes the given
// function.
//
// NOTE: adapted from github.com/containers/storage/drivers/overlay
func CreateUsernsProcess(idMap *specs.LinuxIDMapping, execFunc func(), cwd string, newMountNs bool) (int, func(), error) {

	currCwd, err := os.Getwd()
	if err != nil {
		return 0, nil, err
	}

	if err := os.Chdir(cwd); err != nil {
		return 0, nil, err
	}
	defer os.Chdir(currCwd)

	flags := unix.CLONE_NEWUSER | uintptr(unix.SIGCHLD)
	if newMountNs {
		flags = flags | unix.CLONE_NEWNS
	}

	pid, _, err2 := syscall.Syscall6(uintptr(unix.SYS_CLONE), flags, 0, 0, 0, 0, 0)
	if err2 != 0 {
		return -1, nil, err2
	}

	if pid == 0 {
		// We are in the child; if our parent dies, ask the kernel to kill us
		unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0)

		// Wait for the parent to do the user-ns uid & gid mappings (and timeout in 3 secs)
		readIDMapFile := func(fname string) (*specs.LinuxIDMapping, error) {
			data, err := os.ReadFile(fname)
			if err != nil {
				return nil, err
			}
			fields := strings.Fields(string(data))
			if len(fields) < 3 {
				return nil, errors.New("invalid mapping")
			}
			containerID, _ := strconv.Atoi(fields[0])
			hostID, _ := strconv.Atoi(fields[1])
			size, _ := strconv.Atoi(fields[2])

			return &specs.LinuxIDMapping{
				ContainerID: uint32(containerID),
				HostID:      uint32(hostID),
				Size:        uint32(size),
			}, nil
		}

		mapFiles := []string{"uid_map", "gid_map"}
		foundMapping := false

		for _, f := range mapFiles {
			for i := 0; i < 30; i++ {
				m, err := readIDMapFile(fmt.Sprintf("/proc/self/%s", f))
				if err != nil {
					continue
				}
				if m.ContainerID == idMap.ContainerID &&
					m.HostID == idMap.HostID &&
					m.Size == idMap.Size {
					foundMapping = true
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !foundMapping {
				os.Exit(1)
			}
		}

		// Now execute the function we were given
		execFunc()
	}

	childKillFunc := func() {
		unix.Kill(int(pid), unix.SIGKILL)
	}

	// Write the user-ns mappings (the child is waiting for them)
	writeMapping := func(fname string, idmap *specs.LinuxIDMapping) error {
		mapping := fmt.Sprintf("%d %d %d\n", idmap.ContainerID, idmap.HostID, idmap.Size)
		return ioutil.WriteFile(fmt.Sprintf("/proc/%d/%s", pid, fname), []byte(mapping), 0600)
	}

	if err := writeMapping("uid_map", idMap); err != nil {
		childKillFunc()
		return -1, nil, err
	}

	if err := writeMapping("gid_map", idMap); err != nil {
		childKillFunc()
		return -1, nil, err
	}

	return int(pid), childKillFunc, nil
}
