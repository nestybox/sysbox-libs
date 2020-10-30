//
// Copyright 2020 Nestybox, Inc.
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
	"bytes"
	"fmt"
	"path/filepath"
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
			elems := strings.Split(string(line), "=")
			if len(elems) == 1 {
				continue
			}

			if elems[0] == "ID" {
				return elems[1], nil
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

// Obtain location of kernel-headers for a given linux distro.
func GetLinuxHeaderPath(distro string) (string, error) {

	var path string

	kernelRel, err := GetKernelRelease()
	if err != nil {
		return "", err
	}

	if distro == "redhat" || distro == "centos" || distro == "fedora" {
		path = filepath.Join("/usr/src/kernels", kernelRel)
	} else {
		// All other distros appear to be following the "/usr/src/linux-headers-rel"
		// naming convention.
		kernelHdr := "linux-headers-" + kernelRel
		path = filepath.Join("/usr/src", kernelHdr)
	}

	return path, nil
}
