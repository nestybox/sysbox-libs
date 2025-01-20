package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// IsMountPoint quickly checks if the given path is a mountpoint. It's fast
// because it avoids the expensive reading and parsing of /proc/self/mountinfo
// for the current process and instead relies on comparing the device IDs for
// the given path versus that of it's parent mount path. This works well, except
// for bind-mounts since the device ID does not differ in that case (use FindMount()
// instead).
func IsMountPoint(path string) (bool, error) {

	if path == "/" {
		return true, nil
	}

	// Get file info for the path
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("failed to stat path: %w", err)
	}

	// Get file info for the parent directory
	parentPath := filepath.Join(path, "..")
	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat parent path: %w", err)
	}

	// Compare device IDs using Sys() data
	fileStat, ok1 := fileInfo.Sys().(*syscall.Stat_t)
	parentStat, ok2 := parentInfo.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		return false, fmt.Errorf("failed to retrieve Stat_t from file info")
	}

	return fileStat.Dev != parentStat.Dev, nil
}

// GetMounts retrieves a list of mounts for the current running process.
func GetMounts() ([]*Info, error) {
	return parseMountTable()
}

// GetMountsPid retrieves a list of mounts for the 'pid' process.
func GetMountsPid(pid uint32) ([]*Info, error) {
	return parseMountTableForPid(pid)
}

func FindMount(mountpoint string, mounts []*Info) bool {
	for _, m := range mounts {
		if m.Mountpoint == mountpoint {
			return true
		}
	}
	return false
}

// MountedWithFs looks at /proc/self/mountinfo to determine if the specified
// mountpoint has been mounted with the given filesystem type.
func MountedWithFs(mountpoint string, fs string, mounts []*Info) (bool, error) {

	// Search the table for the mountpoint
	for _, m := range mounts {
		if m.Mountpoint == mountpoint && m.Fstype == fs {
			return true, nil
		}
	}
	return false, nil
}

// GetMountAt returns information about the given mountpoint.
func GetMountAt(mountpoint string, mounts []*Info) (*Info, error) {

	// Search the table for the given mountpoint
	for _, m := range mounts {
		if m.Mountpoint == mountpoint {
			return m, nil
		}
	}
	return nil, fmt.Errorf("%s is not a mountpoint", mountpoint)
}

// Converts the set of mount options (e.g., "rw", "nodev", etc.) to it's
// corresponding mount flags representation
func OptionsToFlags(opt []string) int {
	return optToFlag(opt)
}
