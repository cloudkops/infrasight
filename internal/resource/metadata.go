package resource

// Metadata carries provider-agnostic labels/annotations.
type Metadata struct {
	Labels      map[string]string
	Annotations map[string]string
}
