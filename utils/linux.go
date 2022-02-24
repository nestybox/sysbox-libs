//
// Copyright 2020 - 2022 Nestybox, Inc.
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

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

	if distro == "redhat" || distro == "centos" || distro == "rocky" || distro == "almalinux" || distro == "fedora" {
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

func KernelSupportsIDMappedMounts() (bool, error) {
	var major, minor int

	rel, err := GetKernelRelease()
	if err != nil {
		return false, err
	}

	major, minor, err = ParseKernelRelease(rel)
	if err != nil {
		return false, err
	}

	// ID-Mapped mounts requires Linux kernel >= 5.12

	if major < 5 {
		return false, nil
	} else if major == 5 && minor < 12 {
		return false, nil
	} else {
		return true, nil
	}
}

// KernelModSupported returns nil if the given module is loaded in the kernel.
func KernelModSupported(mod string) (bool, error) {

	// Load the module
	exec.Command("modprobe", mod).Run()

	// Check if the module is in the kernel
	f, err := os.Open("/proc/modules")
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

	return false, nil
}
