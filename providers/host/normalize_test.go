package host

import (
	"testing"

	infraresource "github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/engine"
	"github.com/cloudkops/infrasight/internal/rule/parser"
	"github.com/cloudkops/infrasight/internal/scan/pipeline"
)

func TestMapUnit_RootAndHardeningFlags(t *testing.T) {
	u := unitWithPorts{
		unitInfo: unitInfo{
			Name:                   "nginx.service",
			Description:            "nginx web server",
			ActiveState:            "active",
			SubState:               "running",
			FragmentPath:           "/lib/systemd/system/nginx.service",
			MainPID:                1234,
			User:                   "",
			NoNewPrivileges:        false,
			ProtectSystem:          "strict",
			ProtectHome:            "no",
			AmbientCapabilities:    []string{"NET_BIND_SERVICE"},
			CapabilityBoundingSet:  []string{"SYS_ADMIN", "NET_BIND_SERVICE"},
			PrivateTmp:             true,
			RestrictNamespaces:     "yes",
			MemoryDenyWriteExecute: true,
		},
		ports: []int32{80, 443},
	}

	got := mapUnit(u)

	if got.ID != "nginx.service" || got.Name != "nginx.service" || got.Kind != "systemd_unit" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.Provider != Name {
		t.Errorf("expected provider %q, got %q", Name, got.Provider)
	}
	if !got.Security.RunAsRoot {
		t.Error("expected empty User= to be flagged as root")
	}
	if got.Security.Privileged {
		t.Error("expected Privileged to always be false (no direct systemd equivalent)")
	}
	if got.Security.AllowPrivilegeEscalation == nil || !*got.Security.AllowPrivilegeEscalation {
		t.Error("expected AllowPrivilegeEscalation=true when NoNewPrivileges=false")
	}
	if got.Security.ReadOnlyRootFilesystem == nil || !*got.Security.ReadOnlyRootFilesystem {
		t.Error("expected ReadOnlyRootFilesystem=true when ProtectSystem=strict")
	}
	if len(got.Security.CapabilitiesAdd) != 1 || got.Security.CapabilitiesAdd[0] != "NET_BIND_SERVICE" {
		t.Errorf("expected CapabilitiesAdd=[NET_BIND_SERVICE], got %v", got.Security.CapabilitiesAdd)
	}
	if got.Security.CapabilitiesDrop != nil {
		t.Error("expected CapabilitiesDrop to stay nil (CapabilityBoundingSet is not an additive drop list)")
	}
	if len(got.Networking.Exposures) != 2 {
		t.Fatalf("expected 2 exposures, got %d", len(got.Networking.Exposures))
	}
	if got.Networking.Exposures[0].Type != "port" || got.Networking.Exposures[0].Port != 80 {
		t.Errorf("unexpected first exposure: %+v", got.Networking.Exposures[0])
	}
	if got.Attributes["systemd.capability_bounding_set"].([]string)[0] != "SYS_ADMIN" {
		t.Errorf("expected raw bounding set preserved in attributes, got %+v", got.Attributes["systemd.capability_bounding_set"])
	}
	if got.Attributes["systemd.protect_system"] != "strict" {
		t.Errorf("expected raw protect_system preserved, got %+v", got.Attributes["systemd.protect_system"])
	}
	if got.Attributes["systemd.main_pid"] != 1234 {
		t.Errorf("expected main_pid=1234, got %+v", got.Attributes["systemd.main_pid"])
	}
}

func TestMapUnit_NonRootNoNewPrivilegesNotStrict(t *testing.T) {
	u := unitWithPorts{unitInfo: unitInfo{
		Name:            "app.service",
		User:            "appuser",
		NoNewPrivileges: true,
		ProtectSystem:   "full",
	}}

	got := mapUnit(u)

	if got.Security.RunAsRoot {
		t.Error("expected non-root User= to not be flagged as root")
	}
	if *got.Security.AllowPrivilegeEscalation {
		t.Error("expected AllowPrivilegeEscalation=false when NoNewPrivileges=true")
	}
	if *got.Security.ReadOnlyRootFilesystem {
		t.Error("expected ReadOnlyRootFilesystem=false when ProtectSystem=full (only strict counts)")
	}
	if len(got.Networking.Exposures) != 0 {
		t.Errorf("expected no exposures for a unit with no ports, got %+v", got.Networking.Exposures)
	}
}

func TestMapProcess_UnmanagedListeningProcess(t *testing.T) {
	p := procInfo{
		pid:     4242,
		comm:    "my-custom-app",
		cmdline: "/opt/app/bin/my-custom-app --serve",
		exePath: "/opt/app/bin/my-custom-app",
		uid:     0,
		ports:   []int32{9000},
	}

	got := mapProcess(p)

	if got.Kind != "process" {
		t.Fatalf("unexpected kind: %+v", got)
	}
	if got.ID != "pid-4242" {
		t.Errorf("expected ID=pid-4242, got %q", got.ID)
	}
	if got.Name != "my-custom-app" {
		t.Errorf("expected name from comm, got %q", got.Name)
	}
	if !got.Security.RunAsRoot {
		t.Error("expected uid 0 to be flagged as root")
	}
	if len(got.Networking.Exposures) != 1 || got.Networking.Exposures[0].Port != 9000 {
		t.Errorf("unexpected exposures: %+v", got.Networking.Exposures)
	}
	if got.Attributes["process.exe_path"] != "/opt/app/bin/my-custom-app" {
		t.Errorf("unexpected exe_path: %+v", got.Attributes["process.exe_path"])
	}
}

func TestMapProcess_NameFallsBackWhenNoComm(t *testing.T) {
	p := procInfo{pid: 55, cmdline: "/usr/bin/sleep 100"}
	got := mapProcess(p)
	if got.Name != "sleep" {
		t.Errorf("expected name fallback to cmdline basename, got %q", got.Name)
	}

	p2 := procInfo{pid: 66}
	got2 := mapProcess(p2)
	if got2.Name != "pid-66" {
		t.Errorf("expected final fallback to pid-<n>, got %q", got2.Name)
	}
}

func TestNormalizeAll_CombinesUnitsAndProcesses(t *testing.T) {
	raw := &rawObjects{
		units:     []unitWithPorts{{unitInfo: unitInfo{Name: "a.service"}}},
		processes: []procInfo{{pid: 1, comm: "b"}},
	}
	got := NormalizeAll(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(got))
	}
	if got[0].Kind != "systemd_unit" || got[1].Kind != "process" {
		t.Errorf("unexpected kinds: %+v, %+v", got[0].Kind, got[1].Kind)
	}
}

// TestHostResources_AreContainerless guards the key design decision this
// provider is built around: a systemd_unit or process Resource must never carry
// Runtime.Containers, or internal/rule/engine's containerless-rule-skip logic
// (see engine.go's ruleReferencesContainerField) would silently never fire any
// container.*-scoped rule against it — exactly the bug class the Docker
// provider had to be fixed for. host-baseline rules must stick to
// resource.*/systemd.*/process.* fields.
func TestHostResources_AreContainerless(t *testing.T) {
	unit := mapUnit(unitWithPorts{unitInfo: unitInfo{Name: "x.service"}})
	if len(unit.Runtime.Containers) != 0 {
		t.Error("expected systemd_unit Resource to have zero nested containers")
	}
	proc := mapProcess(procInfo{pid: 1})
	if len(proc.Runtime.Containers) != 0 {
		t.Error("expected process Resource to have zero nested containers")
	}
}

// TestHostBaseline_KeyRulesFireOnUnhardenedUnit proves the shipped
// host-baseline ruleset actually produces findings against resources shaped
// like normalize.go really produces, not just that the YAML parses — the same
// class of regression test that caught the Docker provider's container-scoped
// field bug (see providers/docker/normalize_test.go).
func TestHostBaseline_KeyRulesFireOnUnhardenedUnit(t *testing.T) {
	rules, err := parser.LoadRuleset("../../rulesets/host-baseline", "host")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}

	unhardened := mapUnit(unitWithPorts{unitInfo: unitInfo{
		Name:                   "legacy-app.service",
		User:                   "",    // root
		NoNewPrivileges:        false, // escalation allowed
		ProtectSystem:          "",    // not read-only
		ProtectHome:            "no",
		AmbientCapabilities:    []string{"SYS_ADMIN"},
		CapabilityBoundingSet:  []string{"SYS_ADMIN"},
		PrivateTmp:             false,
		RestrictNamespaces:     "no",
		MemoryDenyWriteExecute: false,
		ActiveState:            "active",
	}})
	failedUnit := mapUnit(unitWithPorts{unitInfo: unitInfo{Name: "broken.service", ActiveState: "failed"}})
	unmanaged := mapProcess(procInfo{pid: 999, comm: "sketchy-listener", ports: []int32{31337}})

	resources := pipeline.EnrichAttributes([]infraresource.Resource{unhardened, failedUnit, unmanaged})
	findings := engine.Evaluate(resources, rules)

	fired := make(map[string]bool, len(findings))
	for _, f := range findings {
		fired[f.RuleID] = true
	}

	mustFire := []string{
		"ISG-HOST-001", // running as root
		"ISG-HOST-002", // privilege escalation allowed
		"ISG-HOST-003", // root filesystem not strictly protected
		"ISG-HOST-004", // dangerous ambient capability (SYS_ADMIN)
		"ISG-HOST-005", // dangerous capability still in bounding set
		"ISG-HOST-006", // PrivateTmp not enabled
		"ISG-HOST-007", // home directories not protected
		"ISG-HOST-008", // namespaces not restricted
		"ISG-HOST-009", // MemoryDenyWriteExecute not set
		"ISG-HOST-010", // failed unit
		"ISG-HOST-011", // unmanaged listening process
	}
	for _, id := range mustFire {
		if !fired[id] {
			t.Errorf("expected rule %s to fire, it did not", id)
		}
	}
}
