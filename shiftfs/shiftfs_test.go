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
	"github.com/nestybox/sysbox-libs/linuxUtils"
	"os"
	"testing"
)

func TestShiftfsSupported(t *testing.T) {

	kernelSupportsShiftfs, err := linuxUtils.KernelModSupported("shiftfs")
	if err != nil {
		t.Fatal(err)
	}

	if kernelSupportsShiftfs {
		dir := "/var/lib/sysbox"

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("ShiftfsSupported() failed with error: %s", err)
		}

		supported, err := ShiftfsSupported(dir)
		if err != nil {
			t.Fatalf("ShiftfsSupported() failed with error: %s", err)
		}

		if !supported {
			t.Logf("shiftfs not supported on this host.")
		}
	}
}

func TestShiftfsSupportedOnOverlayfs(t *testing.T) {

	kernelSupportsShiftfs, err := linuxUtils.KernelModSupported("shiftfs")
	if err != nil {
		t.Fatal(err)
	}

	if kernelSupportsShiftfs {
		dir := "/var/lib/sysbox"

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("ShiftfsSupportedOnOverlayfs() failed with error: %s", err)
		}

		supported, err := ShiftfsSupportedOnOverlayfs(dir)
		if err != nil {
			t.Fatalf("ShiftfsSupportedOnOverlayfs() failed with error: %s", err)
		}

		if !supported {
			t.Logf("shiftfs-on-overlayfs not supported on this host.")
		}
	}
}
