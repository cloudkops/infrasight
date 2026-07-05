package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"

	"github.com/cloudkops/infrasight/internal/report"
	infraresource "github.com/cloudkops/infrasight/internal/resource"
	"github.com/cloudkops/infrasight/internal/rule/engine"
	"github.com/cloudkops/infrasight/internal/rule/parser"
	"github.com/cloudkops/infrasight/internal/scan/pipeline"
)

func privilegedContainer() *container.InspectResponse {
	return &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:       "abc123def456abc123def456abc123def456abc123def456abc123def456ab",
			Name:     "/evil-container",
			Platform: "linux",
			State:    &container.State{Status: "running", Running: true, Pid: 4242},
			HostConfig: &container.HostConfig{
				Privileged:     true,
				ReadonlyRootfs: false,
				CapAdd:         []string{"SYS_ADMIN"},
				NetworkMode:    "host",
				PidMode:        "host",
				IpcMode:        "host",
				SecurityOpt:    []string{"seccomp=unconfined"},
			},
		},
		Config: &container.Config{
			Image:  "docker.io/library/nginx:latest",
			User:   "",
			Labels: map[string]string{"team": "payments"},
		},
		Mounts: []container.MountPoint{
			{Source: "/var/run/docker.sock"},
		},
	}
}

// TestMapContainer_ShapeMirrorsKubernetes documents the fix for the bug where
// container-scoped rule fields (container.*, container.security.*) never fired
// for any Docker resource: the container's own security/attributes must live on a
// nested resource.Container inside Runtime.Containers, exactly like
// providers/kubernetes's mapPod does for a Pod's containers — not on the
// top-level Resource, which only carries the aggregated/resource-wide view.
func TestMapContainer_ShapeMirrorsKubernetes(t *testing.T) {
	got := mapContainer(privilegedContainer())

	if got.Kind != "container" || got.Name != "evil-container" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if len(got.Runtime.Containers) != 1 {
		t.Fatalf("expected exactly one nested container, got %d", len(got.Runtime.Containers))
	}

	c := got.Runtime.Containers[0]
	if c.Image != "docker.io/library/nginx:latest" {
		t.Errorf("expected container.Image to be set from Config.Image, got %q", c.Image)
	}
	if !c.Privileged {
		t.Error("expected nested Container.Privileged to be true")
	}
	if !c.Security.Privileged {
		t.Error("expected nested Container.Security.Privileged to be true")
	}
	if len(c.Security.CapabilitiesAdd) != 1 || c.Security.CapabilitiesAdd[0] != "SYS_ADMIN" {
		t.Errorf("expected CapabilitiesAdd=[SYS_ADMIN], got %v", c.Security.CapabilitiesAdd)
	}
	if c.Security.ReadOnlyRootFilesystem == nil || *c.Security.ReadOnlyRootFilesystem {
		t.Error("expected ReadOnlyRootFilesystem to be an explicit false, not unset/true")
	}

	// Resource-level rollup, not the per-container detail.
	if !got.Security.Privileged {
		t.Error("expected aggregateSecurity to roll privileged up to the resource")
	}
	if !got.Networking.HasExposure("hostNetwork") {
		t.Error("expected hostNetwork exposure at the resource level")
	}
	if got.Attributes["resource.host_pid"] != true {
		t.Errorf("expected resource.host_pid=true, got %+v", got.Attributes["resource.host_pid"])
	}
	if got.Attributes["resource.host_ipc"] != true {
		t.Errorf("expected resource.host_ipc=true, got %+v", got.Attributes["resource.host_ipc"])
	}

	// Container-scoped attributes belong on the nested container, not the resource.
	if c.Attributes["container.docker_sock_mount"] != true {
		t.Errorf("expected docker_sock_mount on the nested container, got %+v", c.Attributes)
	}
	if _, dup := got.Attributes["container.docker_sock_mount"]; dup {
		t.Error("container.docker_sock_mount should not also live on the resource")
	}
}

// TestMapContainer_DeadAttributesRemoved guards against reintroducing
// resource.Container-shadowed attributes: resolveField's typed fast path for
// "container.image"/"container.privileged" always wins over the Attributes
// fallback, so setting them in Attributes too is dead weight that can silently
// diverge from the typed fields.
func TestMapContainer_DeadAttributesRemoved(t *testing.T) {
	got := mapContainer(privilegedContainer())
	c := got.Runtime.Containers[0]
	for _, key := range []string{"container.image", "container.user", "container.privileged", "container.cap_add"} {
		if _, ok := c.Attributes[key]; ok {
			t.Errorf("attribute %q should not be set on the container (shadowed by a typed field or SecurityContext.Flatten)", key)
		}
	}
}

func TestMapContainer_NonRootReadOnly(t *testing.T) {
	c := &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    "def456abc123def456abc123def456abc123def456abc123def456abc123de",
			Name:  "/app",
			State: &container.State{Status: "running", Running: true},
			HostConfig: &container.HostConfig{
				ReadonlyRootfs: true,
			},
		},
		Config: &container.Config{
			Image: "app:1.4.2",
			User:  "1000:1000",
		},
	}

	got := mapContainer(c)
	nested := got.Runtime.Containers[0]

	if nested.Security.RunAsRoot {
		t.Error("expected uid 1000 to not be flagged as root")
	}
	if nested.User != 1000 {
		t.Errorf("expected parsed UID 1000, got %d", nested.User)
	}
	if nested.Security.ReadOnlyRootFilesystem == nil || !*nested.Security.ReadOnlyRootFilesystem {
		t.Error("expected ReadOnlyRootFilesystem=true")
	}
	if got.Security.RunAsRoot {
		t.Error("expected resource-level RunAsRoot rollup to be false")
	}
}

func TestMapContainer_NilConfigAndHostConfigDoNotPanic(t *testing.T) {
	c := &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    "0011223344550011223344550011223344550011223344550011223344",
			Name:  "/bare",
			State: &container.State{},
		},
	}

	got := mapContainer(c) // must not panic despite nil Config/HostConfig
	if got.Kind != "container" {
		t.Errorf("unexpected kind: %+v", got)
	}
}

func TestShortID_HandlesShortInput(t *testing.T) {
	if got := shortID("abc123def456abc123"); got != "abc123def456" {
		t.Errorf("expected 12-char truncation, got %q", got)
	}
	if got := shortID("short"); got != "short" {
		t.Errorf("expected id shorter than 12 chars to pass through unchanged, got %q", got)
	}
}

func TestMapImage_NilConfigDoesNotPanic(t *testing.T) {
	img := &image.InspectResponse{ID: "sha256:deadbeefcafe00112233445566778899aabbccddeeff0011223344556677"}
	got := mapImage(img)
	if got.Kind != "image" {
		t.Errorf("unexpected kind: %+v", got)
	}
	if got.Metadata.Labels != nil {
		t.Errorf("expected nil labels when Config is nil, got %+v", got.Metadata.Labels)
	}
}

func TestMapVolumeAndNetwork_Basic(t *testing.T) {
	vol := volume.Volume{Name: "data", Driver: "local", Labels: map[string]string{}}
	got := mapVolume(&vol)
	if got.Attributes["volume.driver"] != "local" {
		t.Errorf("unexpected volume attrs: %+v", got.Attributes)
	}

	net := network.Inspect{ID: "netid0011223344556677", Name: "app-net", Driver: "bridge"}
	gotNet := mapNetwork(&net)
	if gotNet.Name != "app-net" || gotNet.ID != "netid0011223" {
		t.Errorf("unexpected network resource: %+v", gotNet)
	}
}

// TestDockerBaseline_KeyRulesFireOnPrivilegedContainer is the evaluation-level
// regression test that was missing before: it proves the shipped docker-baseline
// ruleset actually produces findings for a container shaped like
// providers/docker/normalize.go really produces one, not just that the YAML
// parses. This is the exact class of bug (container-scoped fields silently
// skipped for containerless resources) that a parse-only test cannot catch.
func TestDockerBaseline_KeyRulesFireOnPrivilegedContainer(t *testing.T) {
	rules, err := parser.LoadRuleset("../../rulesets/docker-baseline", "docker")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}

	resources := pipeline.EnrichAttributes([]infraresource.Resource{mapContainer(privilegedContainer())})
	findings := engine.Evaluate(resources, rules)

	fired := make(map[string]bool, len(findings))
	for _, f := range findings {
		fired[f.RuleID] = true
	}

	mustFire := []string{
		"ISG-DOC-002", // privileged
		"ISG-DOC-003", // docker socket mounted
		"ISG-DOC-005", // host PID
		"ISG-DOC-006", // host IPC
		"ISG-DOC-007", // dangerous capability added (SYS_ADMIN)
		"ISG-DOC-008", // read-only root filesystem not enforced
	}
	for _, id := range mustFire {
		if !fired[id] {
			t.Errorf("expected rule %s to fire, it did not (findings: %v)", id, findingIDs(findings))
		}
	}
}

func findingIDs(findings []report.Finding) []string {
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.RuleID
	}
	return ids
}
