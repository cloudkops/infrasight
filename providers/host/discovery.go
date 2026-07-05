package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// unitWithPorts is a discovered systemd unit plus the TCP ports any of its
// processes are listening on, resolved via the cgroup-based correlation in
// correlateSockets.
type unitWithPorts struct {
	unitInfo
	ports []int32
}

// procInfo is a listening process discovery could NOT attribute to any systemd
// unit — reported standalone so nothing exposed is silently dropped.
type procInfo struct {
	pid     int
	comm    string
	cmdline string
	exePath string
	uid     int
	ports   []int32
}

type rawObjects struct {
	units     []unitWithPorts
	processes []procInfo
	warnings  []string
}

// discoverAll gathers systemd unit data (via lister) and /proc-based
// process/socket data (rooted at procRoot), then correlates listening sockets to
// the systemd unit that owns them — or, if none does, to a standalone process.
// Partial failures (systemctl unavailable, a single PID unreadable) are recorded
// as warnings and do not abort discovery; only procRoot itself being unreadable
// is a hard failure, since without it there is nothing left to discover.
func discoverAll(ctx context.Context, procRoot string, lister systemdLister) (*rawObjects, error) {
	raw := &rawObjects{}

	pids, err := listPIDs(procRoot)
	if err != nil {
		return nil, fmt.Errorf("host: read %s: %w", procRoot, err)
	}

	units, err := lister.ListUnits(ctx)
	if err != nil {
		raw.warnings = append(raw.warnings, fmt.Sprintf("systemd: %v (unit discovery skipped, listening processes still reported)", err))
	}
	unitByName := make(map[string]*unitWithPorts, len(units))
	for _, u := range units {
		unitByName[u.Name] = &unitWithPorts{unitInfo: u}
	}

	sockets, warnings := listeningSockets(procRoot)
	raw.warnings = append(raw.warnings, warnings...)

	pidPorts, warnings := correlateSockets(procRoot, pids, sockets)
	raw.warnings = append(raw.warnings, warnings...)

	for pid, ports := range pidPorts {
		unitName, err := unitNameForPID(procRoot, pid)
		if err != nil {
			raw.warnings = append(raw.warnings, fmt.Sprintf("pid %d: read cgroup: %v", pid, err))
		}
		if u, ok := unitByName[unitName]; unitName != "" && ok {
			u.ports = append(u.ports, ports...)
			continue
		}

		p, err := readProcInfo(procRoot, pid)
		if err != nil {
			raw.warnings = append(raw.warnings, fmt.Sprintf("pid %d: %v", pid, err))
			continue
		}
		p.ports = ports
		raw.processes = append(raw.processes, p)
	}

	for _, u := range unitByName {
		raw.units = append(raw.units, *u)
	}
	return raw, nil
}

func listPIDs(procRoot string) ([]int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // not a PID directory (self, net, sys, ...)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// socketEntry is one row parsed from /proc/net/{tcp,tcp6} in the TCP_LISTEN
// state. UDP is deliberately out of scope for v1 — it has no equivalent
// unambiguous "listening" state, and forcing one in would risk misclassifying
// ordinary bound-but-not-listening UDP sockets as exposures.
type socketEntry struct {
	inode uint64
	port  int32
}

const tcpListenState = "0A"

func listeningSockets(procRoot string) ([]socketEntry, []string) {
	var sockets []socketEntry
	var warnings []string
	for _, name := range []string{"net/tcp", "net/tcp6"} {
		entries, err := parseProcNetTCP(filepath.Join(procRoot, name))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		sockets = append(sockets, entries...)
	}
	return sockets, warnings
}

func parseProcNetTCP(path string) ([]socketEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []socketEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if fields[3] != tcpListenState {
			continue
		}
		addrPort := strings.Split(fields[1], ":")
		if len(addrPort) != 2 {
			continue
		}
		portNum, err := strconv.ParseInt(addrPort[1], 16, 32)
		if err != nil {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		entries = append(entries, socketEntry{inode: inode, port: int32(portNum)})
	}
	return entries, nil
}

// correlateSockets maps each listening socket to its owning PID by scanning
// every process's fd table for a "socket:[<inode>]" link, and returns each
// owning PID's set of listening ports. Reading another user's fd directory
// requires root; a permission error is recorded as a warning for that PID and
// that PID is simply skipped, not treated as a fatal error — running as
// non-root means this correlation is best-effort, not complete.
func correlateSockets(procRoot string, pids []int, sockets []socketEntry) (map[int][]int32, []string) {
	if len(sockets) == 0 {
		return nil, nil
	}
	portsByInode := make(map[uint64][]int32, len(sockets))
	for _, s := range sockets {
		portsByInode[s.inode] = append(portsByInode[s.inode], s.port)
	}

	pidPorts := make(map[int][]int32)
	var warnings []string
	for _, pid := range pids {
		fdDir := filepath.Join(procRoot, strconv.Itoa(pid), "fd")
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			if os.IsPermission(err) {
				warnings = append(warnings, fmt.Sprintf("pid %d: %v (socket ownership for this process not resolved; run as root for complete results)", pid, err))
			}
			continue
		}
		for _, e := range entries {
			target, err := os.Readlink(filepath.Join(fdDir, e.Name()))
			if err != nil {
				continue
			}
			inode, ok := socketInode(target)
			if !ok {
				continue
			}
			if ports, ok := portsByInode[inode]; ok {
				pidPorts[pid] = append(pidPorts[pid], ports...)
			}
		}
	}
	return pidPorts, warnings
}

// socketInode extracts the inode number from an fd symlink target shaped
// "socket:[12345]".
func socketInode(target string) (uint64, bool) {
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return 0, false
	}
	inode, err := strconv.ParseUint(target[len("socket:["):len(target)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	return inode, true
}

// unitNameForPID resolves a PID to the systemd unit that owns its cgroup, using
// the same technique systemd-cgls/"ps -o unit" use: the innermost path segment
// ending in ".service". This is robust to forking/multi-process units, unlike
// matching solely on a unit's MainPID (a worker process's PID never equals it).
func unitNameForPID(procRoot string, pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}

	var unit string
	for _, line := range strings.Split(string(data), "\n") {
		for _, part := range strings.Split(line, "/") {
			if strings.HasSuffix(part, ".service") {
				unit = part
			}
		}
	}
	return unit, nil
}

func readProcInfo(procRoot string, pid int) (procInfo, error) {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))

	status, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		return procInfo{}, fmt.Errorf("read status: %w", err)
	}
	comm, uid := parseStatus(string(status))

	cmdlineRaw, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
	cmdline := strings.TrimSpace(strings.ReplaceAll(string(cmdlineRaw), "\x00", " "))

	exePath, _ := os.Readlink(filepath.Join(dir, "exe")) // permission denied/kernel thread: leave empty

	return procInfo{pid: pid, comm: comm, cmdline: cmdline, exePath: exePath, uid: uid}, nil
}

// parseStatus reads a /proc/<pid>/status file's "Name:" and effective UID (the
// second field of "Uid:\t<real>\t<effective>\t<saved>\t<fs>") lines.
func parseStatus(status string) (comm string, uid int) {
	for _, line := range strings.Split(status, "\n") {
		switch {
		case strings.HasPrefix(line, "Name:"):
			comm = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		case strings.HasPrefix(line, "Uid:"):
			fields := strings.Fields(strings.TrimPrefix(line, "Uid:"))
			if len(fields) >= 2 {
				uid = atoiOr(fields[1], 0)
			}
		}
	}
	return comm, uid
}
