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

//go:build !linux || !idmapped_mnt || !cgo
// +build !linux !idmapped_mnt !cgo

package idMap

import (
	"fmt"
)

func IDMapMount(usernsPath, mountPath string, unmountFirst bool) error {
	return fmt.Errorf("idmapped mount unsupported in this Sysbox build.")
}

func IDMapMountSupported(dir string) (bool, error) {
	return false, nil
}

func IDMapMountSupportedOnOverlayfs(dir string) (bool, error) {
	return false, nil
}

func IDMapMountSupportedOnPath(path string) (bool, error) {
	return false, nil
}
