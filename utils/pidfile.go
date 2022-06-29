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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// CreatePidFile writes a sysbox pid to a file. If the file already exists,
// and its pid matches a current sysbox process, then an error is returned.
func CreatePidFile(process string, pidFile string) error {

	pid, err := readPidFile(pidFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil {
		if isProcessRunning(process, pid) {
			return fmt.Errorf("%s process is running as pid %d", process, pid)
		}
	}

	pidStr := fmt.Sprintf("%d\n", os.Getpid())
	if err := ioutil.WriteFile(pidFile, []byte(pidStr), 0400); err != nil {
		return fmt.Errorf("failed to write %s pid to file %s: %s", process, pidFile, err)
	}

	return nil
}

func DestroyPidFile(pidFile string) error {
	return os.RemoveAll(pidFile)
}

func readPidFile(pidFile string) (int, error) {

	bs, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(string(bs)))
}

func isProcessRunning(process string, pid int) bool {

	target, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return false
	}

	base := filepath.Base(target)

	if process != base {
		logrus.Infof("pid %d is not associated to process %s", pid, process)
		return false
	}

	return true
}
