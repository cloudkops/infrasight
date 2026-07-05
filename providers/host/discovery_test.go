package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeSystemdLister lets discovery_test.go drive discoverAll without a real
// systemd instance, mirroring providers/kubernetes/discovery_test.go's use of a
// fake clientset.
type fakeSystemdLister struct {
	units []unitInfo
	err   error
}

func (f fakeSystemdLister) ListUnits(ctx context.Context) ([]unitInfo, error) {
	return f.units, f.err
}

// writeProcess builds a fake /proc/<pid>/{status,cmdline,exe,cgroup} tree.
func writeProcess(t *testing.T, procRoot string, pid int, comm string, uid int, cgroupUnit string) string {
	t.Helper()
	dir := filepath.Join(procRoot, fmt.Sprintf("%d", pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	status := fmt.Sprintf("Name:\t%s\nState:\tS (sleeping)\nUid:\t%d\t%d\t%d\t%d\n", comm, uid, uid, uid, uid)
	if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte("/usr/bin/"+comm+"\x00--flag\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/bin/"+comm, filepath.Join(dir, "exe")); err != nil {
		t.Fatal(err)
	}

	cgroupContent := "0::/user.slice/session.scope\n"
	if cgroupUnit != "" {
		cgroupContent = fmt.Sprintf("0::/system.slice/%s\n", cgroupUnit)
	}
	if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(cgroupContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// addListeningSocket adds a fd symlink under procDir/fd pointing at
// "socket:[<inode>]" — the corresponding /proc/net/tcp row is written separately
// via tcpTableLine/writeNetTCP.
func addListeningSocket(t *testing.T, procDir string, fdNum, inode int) {
	t.Helper()
	fdDir := filepath.Join(procDir, "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := fmt.Sprintf("socket:[%d]", inode)
	if err := os.Symlink(target, filepath.Join(fdDir, fmt.Sprintf("%d", fdNum))); err != nil {
		t.Fatal(err)
	}
}

func tcpTableLine(port int32, inode int) string {
	return fmt.Sprintf("   0: 00000000:%04X 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 %d 1 0000000000000000 100 0 0 10 0",
		port, inode)
}

func writeNetTCP(t *testing.T, procRoot string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(procRoot, "net"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(procRoot, "net", "tcp"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// tcp6 must exist too (listeningSockets reads both); empty is fine.
	header := "sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	if err := os.WriteFile(filepath.Join(procRoot, "net", "tcp6"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverAll_AttributesSocketToSystemdUnit(t *testing.T) {
	procRoot := t.TempDir()

	nginxDir := writeProcess(t, procRoot, 100, "nginx", 0, "nginx.service")
	addListeningSocket(t, nginxDir, 10, 500)
	writeNetTCP(t, procRoot, []string{tcpTableLine(8080, 500)})

	lister := fakeSystemdLister{units: []unitInfo{{Name: "nginx.service", ActiveState: "active"}}}

	raw, err := discoverAll(context.Background(), procRoot, lister)
	if err != nil {
		t.Fatalf("discoverAll: %v", err)
	}
	if len(raw.units) != 1 {
		t.Fatalf("expected 1 unit, got %d: %+v", len(raw.units), raw.units)
	}
	if raw.units[0].Name != "nginx.service" {
		t.Fatalf("unexpected unit: %+v", raw.units[0])
	}
	if len(raw.units[0].ports) != 1 || raw.units[0].ports[0] != 8080 {
		t.Errorf("expected unit to own port 8080, got %v", raw.units[0].ports)
	}
	if len(raw.processes) != 0 {
		t.Errorf("expected no standalone processes (socket attributed to a unit), got %+v", raw.processes)
	}
}

func TestDiscoverAll_UnmanagedListeningProcessReportedStandalone(t *testing.T) {
	procRoot := t.TempDir()

	appDir := writeProcess(t, procRoot, 200, "my-app", 1000, "") // no systemd unit
	addListeningSocket(t, appDir, 11, 600)
	writeNetTCP(t, procRoot, []string{tcpTableLine(9000, 600)})

	lister := fakeSystemdLister{units: nil}

	raw, err := discoverAll(context.Background(), procRoot, lister)
	if err != nil {
		t.Fatalf("discoverAll: %v", err)
	}
	if len(raw.units) != 0 {
		t.Fatalf("expected no units, got %+v", raw.units)
	}
	if len(raw.processes) != 1 {
		t.Fatalf("expected 1 standalone process, got %d: %+v", len(raw.processes), raw.processes)
	}
	p := raw.processes[0]
	if p.pid != 200 || p.comm != "my-app" || p.uid != 1000 {
		t.Errorf("unexpected process: %+v", p)
	}
	if len(p.ports) != 1 || p.ports[0] != 9000 {
		t.Errorf("expected process to own port 9000, got %v", p.ports)
	}
}

func TestDiscoverAll_NonListeningProcessProducesNoResource(t *testing.T) {
	procRoot := t.TempDir()
	writeProcess(t, procRoot, 300, "idle-proc", 1000, "")
	writeNetTCP(t, procRoot, nil)

	raw, err := discoverAll(context.Background(), procRoot, fakeSystemdLister{})
	if err != nil {
		t.Fatalf("discoverAll: %v", err)
	}
	if len(raw.units) != 0 || len(raw.processes) != 0 {
		t.Errorf("expected no units/processes for a process with no listening socket, got units=%+v processes=%+v", raw.units, raw.processes)
	}
}

func TestDiscoverAll_SystemdUnavailableDegradesToProcessOnly(t *testing.T) {
	procRoot := t.TempDir()
	appDir := writeProcess(t, procRoot, 400, "standalone", 0, "")
	addListeningSocket(t, appDir, 12, 700)
	writeNetTCP(t, procRoot, []string{tcpTableLine(22, 700)})

	lister := fakeSystemdLister{err: fmt.Errorf("systemctl: not found")}

	raw, err := discoverAll(context.Background(), procRoot, lister)
	if err != nil {
		t.Fatalf("expected discoverAll to tolerate systemd being unavailable, got error: %v", err)
	}
	if len(raw.warnings) == 0 {
		t.Error("expected a warning recorded for the systemd failure")
	}
	if len(raw.processes) != 1 {
		t.Fatalf("expected the listening process to still be reported, got %+v", raw.processes)
	}
}

func TestDiscoverAll_MissingProcRootIsFatal(t *testing.T) {
	if _, err := discoverAll(context.Background(), "/nonexistent/proc/root/for/test", fakeSystemdLister{}); err == nil {
		t.Fatal("expected an error when procRoot itself cannot be read")
	}
}
