package containerdUtils

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestGetDataRoot(t *testing.T) {
	tests := []struct {
		name           string
		configPath     string
		configContent  string
		expectedRoot   string
		expectError    bool
	}{
		{
			name: "Config with root entry",
			configPath: "/etc/containerd/containerd.toml",
			configContent: `
version = 2

root = "/var/lib/desktop-containerd/daemon"
state = "/run/containerd"

oom_score = 0
imports = ["/etc/containerd/runtime_*.toml", "./debug.toml"]

[grpc]
  address = "/run/containerd/containerd.sock"
  uid = 0
  gid = 0

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "k8s.gcr.io/pause:3.2"
  [plugins."io.containerd.snapshotter.v1.overlayfs"]
    root_path = "/var/lib/containerd/snapshotter"
`,
			expectedRoot: "/var/lib/desktop-containerd/daemon",
			expectError:  false,
		},
		{
			name: "Config without root entry",
			configPath: "/etc/containerd/config.toml",
			configContent: `
version = 2

state = "/run/containerd"
oom_score = 0
imports = ["/etc/containerd/runtime_*.toml", "./debug.toml"]

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "k8s.gcr.io/pause:3.2"
  [plugins."io.containerd.snapshotter.v1.overlayfs"]
    root_path = "/var/lib/containerd/snapshotter"
`,
			expectedRoot: "/var/lib/containerd", // Default path
			expectError:  false,
		},
		{
			name:         "Nonexistent config file",
			configPath:   "/path/to/nowhere",
			expectedRoot: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			var err error

			// Create a temporary config file if content is provided
			if tt.configContent != "" {
				tmpFile, err := ioutil.TempFile("", "config-*.toml")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				defer os.Remove(tmpFile.Name()) // Clean up after test

				// Write the content
				configPath = tmpFile.Name()
				if _, err = tmpFile.WriteString(tt.configContent); err != nil {
					t.Fatalf("Failed to write to temp file: %v", err)
				}
				tmpFile.Close()
			} else {
				configPath = "/nonexistent/config.toml"
			}

			root, err := parseDataRoot(configPath)

			// Check if an error was expected or not
			if tt.expectError && err == nil {
				t.Fatalf("Expected error: %v, got: %v", tt.expectError, err)
			}

			// Check the expected root path if no error was expected
			if !tt.expectError && root != tt.expectedRoot {
				t.Fatalf("Expected root: %s, got: %s", tt.expectedRoot, root)
			}
		})
	}
}
