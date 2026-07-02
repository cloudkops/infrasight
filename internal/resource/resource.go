package resource

// Resource is the Generic Resource Model every provider normalizes into. The Rule
// Engine only ever sees this type — never a provider-specific object.
type Resource struct {
	ID         string
	Name       string
	Namespace  string // or file path / module path for static providers
	Kind       string // pod, deployment, service, container, process, systemd_unit, ...
	Provider   string // kubernetes, docker, host, terraform, ansible
	Metadata   Metadata
	Security   SecurityContext
	Networking Networking
	Runtime    Runtime
	Owner      *Owner
	Attributes map[string]any
}

// Runtime describes the container(s)/process(es) that compose this resource. Empty
// for resources with no such concept (e.g. a Terraform planned resource).
type Runtime struct {
	Containers []Container
}

// Container is one containerized or process-like unit within a Resource.
type Container struct {
	Name       string
	Image      string
	User       int64
	Privileged bool
	Limits     Limits
	Security   SecurityContext
	Attributes map[string]any
}

// Limits carries CPU/memory request and limit values as raw provider strings.
type Limits struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// Owner records the resolved controller/owner of a Resource, used generically by
// internal/scan/pipeline to drop redundant child resources (see relationships.go).
type Owner struct {
	Kind string
	Name string
}
