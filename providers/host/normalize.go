package host

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudkops/infrasight/internal/resource"
)

// NormalizeAll maps discovered systemd units and standalone listening processes
// into resource.Resource. Both Kinds are deliberately containerless
// (Runtime.Containers stays nil) — a systemd unit or a bare process has no
// nested-container concept the way a Kubernetes Pod or a Docker container does,
// so Security is set directly and fully on the top-level Resource instead of
// being rolled up from a nested Container. Rules against this provider must use
// resource.*/resource.security.*/systemd.*/process.* fields — never
// container.* — see docs/host.md.
func NormalizeAll(raw *rawObjects) []resource.Resource {
	var resources []resource.Resource
	for _, u := range raw.units {
		resources = append(resources, mapUnit(u))
	}
	for _, p := range raw.processes {
		resources = append(resources, mapProcess(p))
	}
	return resources
}

func mapUnit(u unitWithPorts) resource.Resource {
	return resource.Resource{
		ID:         u.Name,
		Name:       u.Name,
		Kind:       "systemd_unit",
		Provider:   Name,
		Security:   mapUnitSecurity(u.unitInfo),
		Networking: resource.Networking{Exposures: mapExposures(u.ports)},
		Attributes: mapUnitAttributes(u.unitInfo),
	}
}

// mapUnitSecurity maps systemd hardening properties onto the full
// resource.SecurityContext directly — there is no nested Container to split this
// across the way Docker/Kubernetes do.
func mapUnitSecurity(u unitInfo) resource.SecurityContext {
	runAsRoot := u.User == "" || u.User == "root"
	allowEscalation := !u.NoNewPrivileges
	readOnlyRootFS := u.ProtectSystem == "strict"

	return resource.SecurityContext{
		RunAsRoot: runAsRoot,
		// Privileged has no direct systemd equivalent — left false rather than
		// invented from an approximation of other flags.
		Privileged:               false,
		AllowPrivilegeEscalation: &allowEscalation,
		// ReadOnlyRootFilesystem is a documented approximation: only
		// ProtectSystem=strict makes "/" itself read-only. "full"/"yes" protect
		// only parts of the filesystem — the raw value is kept as
		// systemd.protect_system for rules wanting that finer distinction.
		ReadOnlyRootFilesystem: &readOnlyRootFS,
		CapabilitiesAdd:        u.AmbientCapabilities,
		// CapabilityBoundingSet is a restriction list, not an additive "drop"
		// list — forcing it into CapabilitiesDrop would misrepresent it, so it's
		// exposed only as the raw systemd.capability_bounding_set attribute.
		CapabilitiesDrop: nil,
	}
}

func mapUnitAttributes(u unitInfo) map[string]any {
	return map[string]any{
		"systemd.active_state":              u.ActiveState,
		"systemd.sub_state":                 u.SubState,
		"systemd.description":               u.Description,
		"systemd.fragment_path":             u.FragmentPath,
		"systemd.private_tmp":               u.PrivateTmp,
		"systemd.protect_home":              u.ProtectHome,
		"systemd.protect_system":            u.ProtectSystem,
		"systemd.restrict_namespaces":       u.RestrictNamespaces,
		"systemd.memory_deny_write_execute": u.MemoryDenyWriteExecute,
		"systemd.capability_bounding_set":   u.CapabilityBoundingSet,
		"systemd.main_pid":                  u.MainPID,
	}
}

// mapProcess maps a listening process discovery could not attribute to any
// systemd unit. By construction this Kind only ever exists because it has a
// listening socket and no resolvable unit, so a rule can flag "unmanaged exposed
// process" with just `field: resource.kind, equals: "process"`.
func mapProcess(p procInfo) resource.Resource {
	return resource.Resource{
		ID:   fmt.Sprintf("pid-%d", p.pid),
		Name: processName(p),
		Kind: "process",
		Security: resource.SecurityContext{
			RunAsRoot: p.uid == 0,
		},
		Networking: resource.Networking{Exposures: mapExposures(p.ports)},
		Attributes: map[string]any{
			"process.pid":      p.pid,
			"process.exe_path": p.exePath,
			"process.cmdline":  p.cmdline,
			"process.uid":      p.uid,
		},
	}
}

func processName(p procInfo) string {
	if p.comm != "" {
		return p.comm
	}
	if arg0, _, ok := strings.Cut(p.cmdline, " "); ok || arg0 != "" {
		return filepath.Base(arg0)
	}
	return fmt.Sprintf("pid-%d", p.pid)
}

func mapExposures(ports []int32) []resource.Exposure {
	if len(ports) == 0 {
		return nil
	}
	exposures := make([]resource.Exposure, 0, len(ports))
	for _, port := range ports {
		exposures = append(exposures, resource.Exposure{Type: "port", Port: port})
	}
	return exposures
}
