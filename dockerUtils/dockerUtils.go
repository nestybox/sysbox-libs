//
// Copyright: (C) 2019 - 2020 Nestybox Inc.  All rights reserved.
//

package dockerUtils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nestybox/sysbox-libs/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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
func DockerConnect(timeout time.Duration) (*Docker, error) {

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithTimeout(timeout),
		client.WithAPIVersionNegotiation(),
	)

	if err != nil {
		return nil, newDockerErr(DockerConnErr, fmt.Sprintf("failed to connect to Docker API: %v", err))
	}

	info, err := cli.Info(context.Background())
	if err != nil {
		err2 := cli.Close()
		if err2 != nil {
			return nil, newDockerErr(DockerInfoErr, fmt.Sprintf("failed to retrieve Docker info (%v) and disconnect from Docker API (%v)", err, err2))
		}
		return nil, newDockerErr(DockerInfoErr, fmt.Sprintf("failed to retrieve Docker info", err))
	}

	return &Docker{
		cli:      cli,
		dataRoot: info.DockerRootDir,
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

	filter := filters.NewArgs()
	filter.Add("id", containerID)

	containers, err := d.cli.ContainerList(context.Background(), types.ContainerListOptions{
		All:     true, // required since container may not yet be running
		Filters: filter,
	})

	if err != nil {
		return "", newDockerErr(DockerContInfoErr, err.Error())
	}

	if len(containers) == 0 {
		return "", newDockerErr(DockerContInfoErr, fmt.Sprintf("container %s found", containerID))
	} else if len(containers) > 1 {
		return "", newDockerErr(DockerContInfoErr, fmt.Sprintf("more than one container matches ID %s: %v", containerID, containers))
	}

	return containers[0].ImageID, nil
}

// ContainerGetInfo returns info for the given container. Must be called
// after the container is created.
func (d *Docker) ContainerGetInfo(containerID string) (*ContainerInfo, error) {

	info, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, err
	}

	rootfs := ""
	if info.GraphDriver.Name == "overlay2" {
		rootfs = info.GraphDriver.Data["MergedDir"]
	}

	return &ContainerInfo{
		Rootfs:     rootfs,
		AutoRemove: info.HostConfig.AutoRemove,
	}, nil
}

// ContainerIsDocker returns true if the given container ID corresponds to a
// Docker container. It does this by first trying to query Docker for the
// container. If this doesn't work, it uses a heuristic based on the container's
// rootfs.
func ContainerIsDocker(id, rootfs string) (bool, error) {

	timeout := time.Duration(500 * time.Millisecond)

	docker, err := DockerConnect(timeout)
	if err == nil {
		defer docker.Disconnect()
		_, err := docker.ContainerGetImageID(id)
		return (err == nil), nil
	}

	// The connection to Docker can fail when containers are restarted
	// automatically after reboot (i.e., containers originally launched with
	// "--restart"); Docker won't respond until those are up. See Sysbox issue
	// #184. In this case we determine if the container is a Docker container by
	// examining the container's rootfs.

	return isDockerRootfs(rootfs)
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
			if !utils.StringSliceContains(filenames, dockerDir) {
				isDocker = false
			}
		}

		if isDocker {
			return true, nil
		}
	}

	return false, nil
}
