//
// Copyright: (C) 2019 - 2020 Nestybox Inc.  All rights reserved.
//

package dockerUtils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

// Set to true during testing only
var testMode = false

type ErrCode int

const (
	DockerConnErr ErrCode = iota
	DockerDiscErr
	DockerInfoErr
	DockerContInfoErr
	DockerOtherErr
)

type DockerErr struct {
	Code ErrCode
	msg  string
}

func newDockerErr(code ErrCode, msg string) *DockerErr {
	return &DockerErr{
		Code: code,
		msg:  msg,
	}
}

func (e *DockerErr) Error() string {
	return e.msg
}

type ContainerInfo struct {
	Rootfs     string
	AutoRemove bool
}

type Docker struct {
	cli      *client.Client
	dataRoot string
}

// DockerConnect establishes a session with the Docker daemon.
func DockerConnect() (*Docker, error) {
	// Profiling shows Docker takes on average ~10ms to respond to a single
	// client; with up to 1000 concurrent clients, it takes ~400ms to respond on
	// average (see the TestDockerConnectDelay() test in dockerUtils_test.go).
	// Thus we set the timeout to 1 sec; if it doesn't respond in this time, it
	// likely means Docker is not present.
	cli, err := client.New(client.FromEnv, client.WithTimeout(1*time.Second))
	if err != nil {
		return nil, newDockerErr(DockerConnErr, fmt.Sprintf("failed to connect to Docker API: %v", err))
	}

	// Get the docker data root dir (usually /var/lib/docker)
	res, err := cli.Info(context.Background(), client.InfoOptions{})
	if err != nil {
		err2 := cli.Close()
		if err2 != nil {
			return nil, newDockerErr(DockerInfoErr, fmt.Sprintf("failed to retrieve Docker info (%v) and disconnect from Docker API (%v)", err, err2))
		}
		return nil, newDockerErr(DockerInfoErr, fmt.Sprintf("failed to retrieve Docker info: %v", err))
	}

	return &Docker{
		cli:      cli,
		dataRoot: res.Info.DockerRootDir,
	}, nil
}

func (d *Docker) Disconnect() error {
	err := d.cli.Close()
	if err != nil {
		return newDockerErr(DockerDiscErr, fmt.Sprintf("failed to disconnect from Docker API: %v", err))
	}
	return nil
}

// GetDataRoot returns the Docker daemon's data-root dir (usually "/var/lib/docker/").
func (d *Docker) GetDataRoot() string {
	return d.dataRoot
}

// ContainerGetImageID returns the image ID of the given container; may be
// called during container creation.
func (d *Docker) ContainerGetImageID(containerID string) (string, error) {
	res, err := d.cli.ContainerList(context.Background(), client.ContainerListOptions{
		All:     true, // required since container may not yet be running
		Filters: make(client.Filters).Add("id", containerID),
	})
	if err != nil {
		return "", newDockerErr(DockerContInfoErr, err.Error())
	}

	containers := res.Items
	if len(containers) == 0 {
		return "", newDockerErr(DockerContInfoErr, fmt.Sprintf("container %s not found", containerID))
	} else if len(containers) > 1 {
		return "", newDockerErr(DockerContInfoErr, fmt.Sprintf("more than one container matches ID %s: %v", containerID, containers))
	}

	return containers[0].ImageID, nil
}

// ContainerGetInfo returns info for the given container. Must be called
// after the container is created.
func (d *Docker) ContainerGetInfo(containerID string) (*ContainerInfo, error) {
	res, err := d.cli.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	info := res.Container

	rootfs := ""
	if info.GraphDriver != nil && info.GraphDriver.Name == "overlay2" {
		rootfs = info.GraphDriver.Data["MergedDir"]
	}

	return &ContainerInfo{
		Rootfs:     rootfs,
		AutoRemove: info.HostConfig.AutoRemove,
	}, nil
}

// ListVolumesAt lists Docker volumes with the given host mount point (which implies
// volumes using the "local" driver only).
func (d *Docker) ListVolumesAt(mountPoint string) ([]volume.Volume, error) {
	res, err := d.cli.VolumeList(context.Background(), client.VolumeListOptions{
		Filters: make(client.Filters).Add("driver", "local"),
	})
	if err != nil {
		return nil, err
	}

	// Filter volumes by mount point
	var filteredVolumes []volume.Volume
	for _, vol := range res.Items {
		if vol.Mountpoint == mountPoint {
			filteredVolumes = append(filteredVolumes, vol)
			break
		}
	}

	return filteredVolumes, nil
}

// ContainerIsDocker returns true if the given container ID corresponds to a
// Docker container. It first checks the container's rootfs, which is cheap and
// local; only if that is inconclusive does it query the Docker daemon.
func ContainerIsDocker(id, rootfs string) (bool, error) {

	// Prefer the rootfs heuristic: it avoids a Docker API query that can block
	// for seconds while Docker is unresponsive, which happens exactly when
	// containers are restored after a reboot (containers launched with
	// "--restart"; see Sysbox issue #184).
	isDocker, err := isDockerRootfs(rootfs)
	if err == nil && isDocker {
		return true, nil
	}

	// The rootfs check was negative or errored; ask Docker directly.
	docker, derr := DockerConnect()
	if derr == nil {
		defer docker.Disconnect()
		_, gerr := docker.ContainerGetImageID(id)
		return (gerr == nil), nil
	}

	// Docker is unreachable; fall back to the rootfs heuristic result.
	return isDocker, err
}

// isDockerRootfs determines if the given a container rootfs is for a Docker container.
func isDockerRootfs(rootfs string) (bool, error) {

	// Check if the container rootfs is under Docker's default data root
	// (when in test-mode, we skip this so as to do the deeper check below)
	if !testMode {
		if strings.Contains(rootfs, "/var/lib/docker") {
			return true, nil
		}
	}

	// Check the parent dirs of the rootfs (up to 5 levels) and look for the
	// `image, network, swarm, and containers` directories that are part of the
	// Docker data root.

	dockerDirs := []string{"image", "network", "containers", "swarm"}

	searchLevels := 5
	maxFilesPerDir := 30 // the docker data root dir has typically 10->20 subdirs in it
	path := rootfs

	for i := 0; i < searchLevels; i++ {
		path = filepath.Dir(path)

		dir, err := os.Open(path)
		if err != nil {
			return false, newDockerErr(DockerOtherErr, fmt.Sprintf("failed to open %s: %s\n", path, err))
		}

		filenames, err := dir.Readdirnames(maxFilesPerDir)
		if err != nil {
			return false, newDockerErr(DockerOtherErr, fmt.Sprintf("failed to read directories under %s: %s\n", path, err))
		}

		isDocker := true
		for _, dockerDir := range dockerDirs {
			if !slices.Contains(filenames, dockerDir) {
				isDocker = false
			}
		}

		if isDocker {
			return true, nil
		}
	}

	return false, nil
}
