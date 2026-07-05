package host

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// defaultProcRoot is where discovery reads process/socket data from on a real
// machine. discoverAll takes procRoot as an explicit parameter rather than
// reading this constant directly — tests substitute a fake tree built under
// t.TempDir() the same way providers/kubernetes substitutes a fake clientset.
const defaultProcRoot = "/proc"

// unitInfo is one systemd service unit's identity plus every hardening-relevant
// property this provider understands, already flattened into simple types.
// discovery.go's only systemd-related job is producing these; normalize.go's only
// job is turning them into resource.Resource. Capability names are normalized to
// the same upper-case, no-"CAP_"-prefix form ("SYS_ADMIN") the Kubernetes/Docker
// providers already use, so a rule author writes one capability name convention
// project-wide.
type unitInfo struct {
	Name         string // e.g. "nginx.service"
	Description  string
	ActiveState  string
	SubState     string
	FragmentPath string
	MainPID      int

	User                   string
	NoNewPrivileges        bool
	ProtectSystem          string
	ProtectHome            string
	AmbientCapabilities    []string
	CapabilityBoundingSet  []string
	PrivateTmp             bool
	RestrictNamespaces     string
	MemoryDenyWriteExecute bool
}

// systemdLister is the one seam between discovery.go and the real systemctl
// binary — a fake implementing this interface drives providers/host tests
// without a real systemd instance, mirroring providers/kubernetes/client.go's
// kubernetes.Interface fake-clientset seam.
type systemdLister interface {
	ListUnits(ctx context.Context) ([]unitInfo, error)
}

func newSystemdLister() systemdLister { return execSystemdLister{} }

type execSystemdLister struct{}

// showProperties is passed to a single batched `systemctl show` call — every
// hardening-relevant property this provider maps, plus Id (used to match each
// returned properties block back to the unit that produced it; systemd does not
// guarantee block order matches argument order, so this is required, not
// cosmetic — see client_test.go).
var showProperties = []string{
	"Id", "Description", "ActiveState", "SubState", "FragmentPath", "MainPID",
	"User", "NoNewPrivileges", "ProtectSystem", "ProtectHome",
	"AmbientCapabilities", "CapabilityBoundingSet", "PrivateTmp",
	"RestrictNamespaces", "MemoryDenyWriteExecute",
}

func (execSystemdLister) ListUnits(ctx context.Context) ([]unitInfo, error) {
	names, err := listUnitNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("systemctl list-units: %w", err)
	}
	if len(names) == 0 {
		return nil, nil
	}
	units, err := showUnits(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("systemctl show: %w", err)
	}
	return units, nil
}

type listUnitEntry struct {
	Unit string `json:"unit"`
	Load string `json:"load"`
}

// listUnitNames returns every loaded service unit's name. Units systemd could
// never load (masked, nonexistent — Load != "loaded") are skipped: showUnits
// would only get empty properties back for them.
func listUnitNames(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "list-units",
		"--type=service", "--all", "--output=json", "--no-pager").Output()
	if err != nil {
		return nil, err
	}

	var entries []listUnitEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parse list-units JSON: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Load == "loaded" {
			names = append(names, e.Unit)
		}
	}
	return names, nil
}

// showUnits fetches every unit's properties in one batched `systemctl show`
// call — systemd supports multiple unit names in a single invocation — rather
// than one call per unit, avoiding an N+1 subprocess-per-unit problem entirely.
func showUnits(ctx context.Context, names []string) ([]unitInfo, error) {
	args := []string{"show", "--no-pager"}
	for _, p := range showProperties {
		args = append(args, "-p", p)
	}
	args = append(args, names...)

	out, err := exec.CommandContext(ctx, "systemctl", args...).Output()
	if err != nil {
		return nil, err
	}
	return parseShowOutput(string(out)), nil
}

// parseShowOutput splits systemctl show's output into per-unit blocks (blank-line
// separated) and parses each block's "Key=Value" lines. Matched back to a unit by
// its own "Id" property, never by block position — systemd does not promise block
// order follows argument order.
func parseShowOutput(out string) []unitInfo {
	var units []unitInfo
	for _, block := range strings.Split(strings.TrimRight(out, "\n"), "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		props := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			props[key] = value
		}
		if props["Id"] == "" {
			continue
		}
		units = append(units, unitInfo{
			Name:                   props["Id"],
			Description:            props["Description"],
			ActiveState:            props["ActiveState"],
			SubState:               props["SubState"],
			FragmentPath:           props["FragmentPath"],
			MainPID:                atoiOr(props["MainPID"], 0),
			User:                   props["User"],
			NoNewPrivileges:        props["NoNewPrivileges"] == "yes",
			ProtectSystem:          props["ProtectSystem"],
			ProtectHome:            props["ProtectHome"],
			AmbientCapabilities:    normalizeCapabilities(props["AmbientCapabilities"]),
			CapabilityBoundingSet:  normalizeCapabilities(props["CapabilityBoundingSet"]),
			PrivateTmp:             props["PrivateTmp"] == "yes",
			RestrictNamespaces:     props["RestrictNamespaces"],
			MemoryDenyWriteExecute: props["MemoryDenyWriteExecute"] == "yes",
		})
	}
	return units
}

// normalizeCapabilities turns systemd's space-separated, lower-case "cap_sys_admin"
// form into the upper-case, no-prefix "SYS_ADMIN" form the Docker/Kubernetes
// providers already use for CapabilitiesAdd/CapabilitiesDrop, so a rule author
// writes one capability-name convention regardless of provider.
func normalizeCapabilities(raw string) []string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil
	}
	caps := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToUpper(strings.TrimPrefix(f, "cap_"))
		caps = append(caps, f)
	}
	return caps
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
