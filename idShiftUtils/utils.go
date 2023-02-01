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

package idShiftUtils

import (
	"fmt"
	"io/ioutil"
	"syscall"

	"golang.org/x/sys/unix"
)

// createUsernsProcess forks the current process into a new user-ns, using the
// given the ID mappings. Returns the pid of the new process and a "kill"
// function (so that the caller can kill the child when desired). The new process
// executes the given function.
//
// NOTE: adapted from github.com/containers/storage/drivers/overlay
func createUsernsProcess(uidMaps []IDMapping, gidMaps []IDMapping, execFunc func()) (int, func(), error) {

	pid, _, err := syscall.Syscall6(uintptr(unix.SYS_CLONE), unix.CLONE_NEWUSER|uintptr(unix.SIGCHLD), 0, 0, 0, 0, 0)
	if err != 0 {
		return -1, nil, err
	}

	if pid == 0 {
		// If the parent dies, the child gets SIGKILL
		unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0)
		execFunc()
	}

	cleanupFunc := func() {
		unix.Kill(int(pid), unix.SIGKILL)
		unix.Wait4(int(pid), nil, 0, nil)
	}

	writeMappings := func(fname string, idmap []IDMapping) error {
		mappings := ""
		for _, m := range idmap {
			mappings = mappings + fmt.Sprintf("%d %d %d\n", m.ContainerID, m.HostID, m.Size)
		}
		return ioutil.WriteFile(fmt.Sprintf("/proc/%d/%s", pid, fname), []byte(mappings), 0600)
	}

	if err := writeMappings("uid_map", uidMaps); err != nil {
		cleanupFunc()
		return -1, nil, err
	}

	if err := writeMappings("gid_map", gidMaps); err != nil {
		cleanupFunc()
		return -1, nil, err
	}

	return int(pid), cleanupFunc, nil
}
