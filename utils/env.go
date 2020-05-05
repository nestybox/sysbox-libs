package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetEnvVarInfo returns the name and value of the given environment variable
func GetEnvVarInfo(v string) (string, string, error) {
	tokens := strings.Split(v, "=")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("invalid variable %s", v)
	}
	return tokens[0], tokens[1], nil
}

// CmdExists check if the given command is available on the host
func CmdExists(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
