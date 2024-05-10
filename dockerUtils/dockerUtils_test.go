//
// Copyright: (C) 2019 - 2020 Nestybox Inc.  All rights reserved.
//

package dockerUtils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
)

func TestGetContainer(t *testing.T) {

	testMode = true
	defer func() { testMode = false }()

	docker, err := DockerConnect()
	if err != nil {
		t.Fatalf("DockerConnect() failed: %v", err)
	}
	defer docker.Disconnect()

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

	isDocker, err := ContainerIsDocker(id, ci.Rootfs)
	if err != nil {
		t.Errorf("ContainerIsDocker(%s, %s) failed: %v", id, ci.Rootfs, err)
	}
	if !isDocker {
		t.Errorf("ContainerIsDocker(%s, %s) returned false; expecting true", id, ci.Rootfs)
	}

	isDockerRootfs, err := isDockerRootfs(ci.Rootfs)
	if err != nil {
		t.Errorf("isDockerRootfs(%s) failed: %v", ci.Rootfs, err)
	}
	if !isDockerRootfs {
		t.Errorf("isDockerRootfs(%s) returned false; expecting true", ci.Rootfs)
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

func TestListVolumesAt(t *testing.T) {

	docker, err := DockerConnect()
	if err != nil {
		t.Fatalf("DockerConnect() failed: %v", err)
	}
	defer docker.Disconnect()

	// Prepare by creating a volume to test against
	volName := "testvolume"
	ctx := context.Background()
	_, err = docker.cli.VolumeCreate(ctx, volume.CreateOptions{Name: volName, Driver: "local"})
	assert.NoError(t, err, "should be able to create a volume")

	// Clean up after test
	defer func() {
		err := docker.cli.VolumeRemove(ctx, volName, true)
		assert.NoError(t, err, "should be able to remove the volume")
	}()

	// Test the function
	mountPoint := filepath.Join("/var/lib/docker/volumes/", volName, "_data")
	volumes, err := docker.ListVolumesAt(mountPoint)
	assert.NoError(t, err, "should not have an error listing volumes")
	assert.True(t, len(volumes) > 0, "should find at least one volume")
	found := false
	for _, vol := range volumes {
		if vol.Name == volName && vol.Mountpoint == mountPoint {
			found = true
			break
		}
	}
	assert.True(t, found, "should find the test volume in the filtered list")
}

func TestDockerConnectDelay(t *testing.T) {
	var wg sync.WaitGroup

	numWorkers := 1000
	maxDelay := 500 * time.Millisecond
	delayCh := make(chan time.Duration, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go dockerConnectWorker(&wg, delayCh)
	}

	wg.Wait()

	sum := 0 * time.Second
	for i := 0; i < numWorkers; i++ {
		sum += <-delayCh
	}
	avg := sum / time.Duration(numWorkers)

	if avg > time.Duration(maxDelay) {
		t.Fatalf("DockerConnect() delay failed: want <= %v, got %v", maxDelay, avg)
	}

	t.Logf("DockerConnect() delay for %d concurrent clients = %v (average)\n", numWorkers, avg)
}

// test helpers

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

func dockerConnectWorker(wg *sync.WaitGroup, delayCh chan time.Duration) {
	start := time.Now()
	_, err := DockerConnect()
	delay := time.Since(start)

	if err != nil {
		fmt.Printf("error connecting to docker (delay = %v): %v\n", delay, err)
	}

	delayCh <- delay
	wg.Done()
}
