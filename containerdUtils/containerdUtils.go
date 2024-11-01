//
// Copyright: (C) 2024 Nestybox Inc.  All rights reserved.
//
package containerdUtils

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Location of containerd config file
// (see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md)
var (
	configPath = []string{
		"/etc/containerd/containerd.toml",
		"/etc/containerd/config.toml",
		"/usr/local/etc/containerd/config.toml",
	}

	defaultDataRoot = "/var/lib/containerd"
)

type containerdConfig struct {
	Root string `toml:"Root"`
}

// GetDataRoot returns the containerd data root directory, as read from
// the containerd config file.
func GetDataRoot() (string, error) {
	for _, path := range configPath {
		dataRoot, err := parseDataRoot(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("failed to open file %s: %w", path, err)
		}
		return dataRoot, nil
	}
	return defaultDataRoot, nil
}

func parseDataRoot(path string) (string, error) {
	var config containerdConfig

	// open the config file; if it does not exist, move on to the next one.
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// parse the "root"
	if _, err := toml.NewDecoder(f).Decode(&config); err != nil {
		return "", fmt.Errorf("could not decode %s: %w", path, err)
	}

	// if no "root" present, assume it's the default
	if config.Root == "" {
		return defaultDataRoot, nil
	}

	return config.Root, nil
}
