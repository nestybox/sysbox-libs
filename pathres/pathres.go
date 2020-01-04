// This package performs path resolution on Linux, allowing a caller to determine if a
// given process has permissions to access a given path. Path resolution is done using the
// rules in Linux's path_resolution(7) man page. The caller is assumed to have permissions
// to make all the necessary path checks.

// TODO
//
// * Check if uid 0 bypasses capability checks
// * Consider adding ACL support

package pathres

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	symlinkMax = 40
)

type procInfo struct {
	root string // root dir
	cwd  string // current working dir
	uid  int    // effective uid
	gid  int    // effective gid
	sgid []int  // supplementary groups
	cap  uint64 // effective caps
}

type AccessMode uint32

const (
	R_OK AccessMode = 0x4 // read ok
	W_OK AccessMode = 0x2 // write ok
	X_OK AccessMode = 0x1 // execute ok
)

// isSymlink returns true if the given file is a symlink
func isSymlink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	return fi.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

// intSliceContains returns true if x is in a
func intSliceContains(a []int, x int) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// checkPerm checks if the given process has permission to access the file or directory at
// the given path. The access mode indicates what type of access is being checked (i.e.,
// read, write, execute, or a combination of these). The given path must not be a symlink.
// Returns true if the given process has the required permission, false otherwise. The
// returned error indicates if an error occurred during the check.
func checkPerm(proc *procInfo, path string, aMode AccessMode) (bool, error) {

	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	fperm := fi.Mode().Perm()

	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("failed to convert to syscall.Stat_t")
	}
	fuid := int(st.Uid)
	fgid := int(st.Gid)

	mode := uint32(aMode)

	// Note: the order of the checks below mimics those done by the Linux kernel.

	// owner check
	if fuid == proc.uid {
		perm := uint32((fperm & 0700) >> 6)
		if mode&perm == mode {
			return true, nil
		}
	}

	// group check
	if fgid == proc.gid || intSliceContains(proc.sgid, fgid) {
		perm := uint32((fperm & 0070) >> 3)
		if mode&perm == mode {
			return true, nil
		}
	}

	// "other" check
	perm := uint32(fperm & 0007)
	if mode&perm == mode {
		return true, nil
	}

	// capability checks
	if (proc.cap & unix.CAP_DAC_OVERRIDE) == unix.CAP_DAC_OVERRIDE {
		// Per capabilities(7): CAP_DAC_OVERRIDE bypasses file read, write, and execute
		// permission checks.
		//
		// Per The Linux Programming Interface, 15.4.3: A process with the CAP_DAC_OVERRIDE
		// capability always has read and write permissions for any type of file, and also
		// has execute permission if the file is a directory or if execute permission is
		// granted to at least one of the permission categories for the file.
		if fi.IsDir() {
			return true, nil
		} else {
			if aMode&X_OK != X_OK {
				return true, nil
			} else {
				if fperm&0111 != 0 {
					return true, nil
				}
			}
		}
	}

	if (proc.cap & unix.CAP_DAC_READ_SEARCH) == unix.CAP_DAC_READ_SEARCH {
		// Per capabilities(7): CAP_DAC_READ_SEARCH bypasses file read permission checks and
		// directory read and execute permission checks
		if fi.IsDir() && (aMode&W_OK != W_OK) {
			return true, nil
		}

		if !fi.IsDir() && (aMode == R_OK) {
			return true, nil
		}
	}

	return false, nil
}

func procPathAccess(proc *procInfo, path string, mode AccessMode) error {

	if path == "" {
		return syscall.ENOENT
	}

	if len(path)+1 > syscall.PathMax {
		return syscall.ENAMETOOLONG
	}

	// Determine the start point
	var start string
	if filepath.IsAbs(path) {
		start = proc.root
	} else {
		start = proc.cwd
	}

	// Break up path into it's components; note that repeated "/" results in empty path
	// components
	components := strings.Split(path, "/")

	cur := start
	linkCnt := 0
	final := false

	for i, c := range components {

		if i == len(components)-1 {
			final = true
		}

		if c == "" {
			continue
		}

		if c == ".." {
			parent := filepath.Dir(cur)
			if !strings.HasPrefix(parent, proc.root) {
				parent = proc.root
			}
			cur = parent
		} else if c != "." {
			cur = filepath.Join(cur, c)
		}

		fi, err := os.Stat(cur)
		if err != nil {
			return syscall.ENOENT
		}

		symlink, err := isSymlink(cur)
		if err != nil {
			return syscall.ENOENT
		}

		if !final && !symlink && !fi.IsDir() {
			return syscall.ENOTDIR
		}

		// Follow the symlink (unless it's the proc.root); may recurse if symlink points to
		// another symlink and so on; we stop at symlinkMax recursions (just as the Linux
		// kernel does)

		if symlink && cur != proc.root {
			for {
				if linkCnt >= symlinkMax {
					return syscall.ELOOP
				}
				cur, err = os.Readlink(cur)
				if err != nil {
					return syscall.ENOENT
				}
				isLink, err := isSymlink(cur)
				if err != nil {
					return syscall.ENOENT
				}
				if !isLink {
					break
				}
				linkCnt += 1
			}
			fi, err := os.Stat(cur)
			if err != nil {
				return syscall.ENOENT
			}

			if !final && !fi.IsDir() {
				return syscall.ENOTDIR
			}
		}

		perm := false
		if !final {
			perm, err = checkPerm(proc, cur, X_OK)
		} else {
			perm, err = checkPerm(proc, cur, mode)
		}

		if err != nil || !perm {
			return syscall.EACCES
		}
	}

	return nil
}

// getProcStatus retrieves process status info obtained from the /proc/[pid]/status file
func getProcStatus(pid int, fields []string) (map[string]string, error) {

	filename := fmt.Sprintf("/proc/%d/status", pid)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)

	status := make(map[string]string)
	for s.Scan() {
		text := s.Text()
		parts := strings.Split(text, ":")

		if len(parts) < 1 {
			continue
		}

		for _, f := range fields {
			if parts[0] == f {
				if len(parts) > 1 {
					status[f] = parts[1]
				} else {
					status[f] = ""
				}
			}
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return status, nil
}

// getProcInfo retrieves info about the process with the given pid
func getProcInfo(pid int) (*procInfo, error) {

	space := regexp.MustCompile(`\s+`)

	fields := []string{"Uid", "Gid", "Groups", "CapEff"}
	status, err := getProcStatus(pid, fields)

	// effective uid
	str := space.ReplaceAllString(status["Uid"], " ")
	str = strings.TrimSpace(str)
	uids := strings.Split(str, " ")
	if len(uids) != 4 {
		return nil, fmt.Errorf("invalid uid status: %+v", uids)
	}
	euid, err := strconv.Atoi(uids[1])
	if err != nil {
		return nil, err
	}

	// effective gid
	str = space.ReplaceAllString(status["Gid"], " ")
	str = strings.TrimSpace(str)
	gids := strings.Split(str, " ")
	if len(gids) != 4 {
		return nil, fmt.Errorf("invalid gid status: %+v", gids)
	}
	egid, err := strconv.Atoi(gids[1])
	if err != nil {
		return nil, err
	}

	// supplementary groups
	sgid := []int{}
	str = space.ReplaceAllString(status["Groups"], " ")
	str = strings.TrimSpace(str)
	groups := strings.Split(str, " ")
	for _, g := range groups {
		if g == "" {
			continue
		}
		val, err := strconv.Atoi(g)
		if err != nil {
			return nil, err
		}
		sgid = append(sgid, val)
	}

	// effective caps
	str = strings.TrimSpace(status["CapEff"])
	capEff, err := strconv.ParseInt(str, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid cap status")
	}

	// process root & cwd
	root := fmt.Sprintf("/proc/%d/root", pid)
	cwd := fmt.Sprintf("/proc/%d/cwd", pid)

	pi := &procInfo{
		root: root,
		cwd:  cwd,
		uid:  euid,
		gid:  egid,
		sgid: sgid,
		cap:  uint64(capEff),
	}

	return pi, nil
}

// PathAccess emulates the path resolution and permission checking process done by
// the Linux kernel, as described in path_resolution(7).
//
// It checks if the process with the given pid can access the file or directory at the
// given path. The given mode determines what type of access to check for (e.g., read,
// write, execute, or a combination of these).
//
// The given path may be absolute or relative. Each component of the path is checked to
// see if it exists and whether the process has permissions to access it, following the
// rules for path resolution in Linux (see path_resolution(7)). The path may contain ".",
// "..", and symlinks. For absolute paths, the check is done starting from the process'
// root directory. For relative paths, the check is done starting from the process'
// current working directory.
//
// Returns nil if the process can access the path, or one of the following errors
// otherwise:
//
// syscall.ENOENT: some component of the path does not exist.
// syscall.ENOTDIR: a non-final component of the path is not a directory.
// syscall.EACCES: the process does not have permission to access at least one component of the path.
// syscall.ELOOP: the path too many symlinks (e.g. > 40).

func PathAccess(pid int, path string, mode AccessMode) error {
	procInfo, err := getProcInfo(pid)
	if err != nil {
		return err
	}
	return procPathAccess(procInfo, path, mode)
}
