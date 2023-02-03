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

// NOTE:
//
// Run test with "go test -tags idmapped_mnt" when running on a host with kernel
// >= 5.12. Otherwise the test will use the idMapMount_unsupported.go file.

package idMap

import (
	"os"
	"testing"
)

func TestIDMapMountSupported(t *testing.T) {

	kernelOK, err := checkKernelVersion(5, 12)
	if err != nil {
		t.Fatal(err)
	}

	if kernelOK {
		dir := "/var/lib/sysbox"

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		supported, err := IDMapMountSupported(dir)
		if err != nil {
			t.Fatalf("IDMapMountSupported() failed with error: %s", err)
		}

		if supported {
			t.Logf("ID-mapping supported on this host.")
		} else {
			t.Logf("ID-mapping not supported on this host.")
		}
	}
}

func TestIDMapMountSupportedOnOverlayfs(t *testing.T) {

	kernelOK, err := checkKernelVersion(5, 19)
	if err != nil {
		t.Fatal(err)
	}

	if kernelOK {
		dir := "/var/lib/sysbox"

		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		supported, err := IDMapMountSupportedOnOverlayfs(dir)
		if err != nil {
			t.Fatalf("IDMapMountSupportedOnOverlayfs() failed with error: %s", err)
		}

		if supported {
			t.Logf("ID-mapping-on-overlayfs supported on this host.")
		} else {
			t.Logf("ID-mapping-on-overlayfs not supported on this host.")
		}
	}
}
