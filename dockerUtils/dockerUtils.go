//
// Copyright: (C) 2019 - 2020 Nestybox Inc.  All rights reserved.
//

package dockerUtils

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ContainerInfo struct {
	AutoRemove bool
}

type Docker struct {
	cli      *client.Client
	dataRoot string
}

// DockerConnect establishes a session with the Docker daemon.
func DockerConnect() (*Docker, error) {

	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to Docker API: %v", err)
	}

	info, err := cli.Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve Docker info: %v", err)
	}

	return &Docker{
		cli:      cli,
		dataRoot: info.DockerRootDir,
	}, nil
}

func (d *Docker) Disconnect() error {
	err := d.cli.Close()
	if err != nil {
		return fmt.Errorf("Failed to disconnect from Docker API: %v", err)
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

	cli := d.cli

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
		All:     true, // required since container may not yet be running
		Filters: filter,
	})

	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("not found")
	} else if len(containers) > 1 {
		return "", fmt.Errorf("more than one container matches ID %s: %v", containerID, containers)
	}

	return containers[0].ImageID, nil
}

// ContainerIsDocker returns true if the given container ID corresponds to a
// Docker container.
func (d *Docker) ContainerIsDocker(id string) bool {
	_, err := d.ContainerGetImageID(id)
	return err == nil
}

// ContainerGetInfo returns info for the given container. Must be called
// after the container is created.
func (d *Docker) ContainerGetInfo(containerID string) (*ContainerInfo, error) {

	info, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, err
	}

	return &ContainerInfo{
		AutoRemove: info.HostConfig.AutoRemove,
	}, nil
}
