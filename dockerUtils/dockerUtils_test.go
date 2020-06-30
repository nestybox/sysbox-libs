//
// Copyright: (C) 2019 - 2020 Nestybox Inc.  All rights reserved.
//

package dockerUtils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGetContainer(t *testing.T) {
	docker, err := DockerConnect()
	if err != nil {
		t.Fatalf("DockerConnect() failed: %v", err)
	}

	dataRoot := docker.GetDataRoot()
	if dataRoot != "/var/lib/docker" {
		t.Errorf("docker.GetDataRoot(): want /var/lib/docker; got %s", dataRoot)
	}

	id, err := testStartContainer(false)
	if err != nil {
		t.Fatalf("Failed to start test container: %v", err)
	}

	ci, err := docker.ContainerGetInfo(id)
	if err != nil {
		t.Errorf("ContainerGetInfo(%s) failed: %v", id, err)
	}

	if ci.AutoRemove != false {
		t.Errorf("Container autoRemove mismatch: want false, got true")
	}

	if !docker.ContainerIsDocker(id) {
		t.Errorf("ContainerIsDocker(%s) failed", id)
	}

	if err := testStopContainer(id, true); err != nil {
		t.Errorf("Failed to stop test container: %v", err)
	}
}

func TestGetContainerAutoRemove(t *testing.T) {
	docker, err := DockerConnect()
	if err != nil {
		t.Fatalf("DockerConnect() failed: %v", err)
	}

	id, err := testStartContainer(true)
	if err != nil {
		t.Fatalf("Failed to start test container: %v", err)
	}

	ci, err := docker.ContainerGetInfo(id)
	if err != nil {
		t.Errorf("ContainerGetInfo(%s) failed: %v", id, err)
	}

	if ci.AutoRemove != true {
		t.Errorf("Container autoRemove mismatch: want true, got false")
	}

	if err := testStopContainer(id, false); err != nil {
		t.Errorf("Failed to stop test container: %v", err)
	}
}

func testStartContainer(autoRemove bool) (string, error) {
	var cmd *exec.Cmd
	var stdout, stderr bytes.Buffer

	if autoRemove {
		cmd = exec.Command("docker", "run", "-d", "--rm", "alpine", "tail", "-f", "/dev/null")
	} else {
		cmd = exec.Command("docker", "run", "-d", "alpine", "tail", "-f", "/dev/null")
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to start test container: %s %s\n", stdout.String(), stderr.String())
	}

	id := strings.TrimSuffix(stdout.String(), "\n")
	return id, nil
}

func testStopContainer(id string, remove bool) error {
	var cmd *exec.Cmd
	var stdout, stderr bytes.Buffer

	if remove {
		cmd = exec.Command("docker", "rm", "-f", id)
	} else {
		cmd = exec.Command("docker", "stop", "-t0", id)
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stop test container: %s %s\n", stdout.String(), stderr.String())
	}

	return nil
}
