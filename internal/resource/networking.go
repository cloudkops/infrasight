package resource

// Networking carries resource-level exposure signals — a resource-scoped concept
// (a Kubernetes Pod's hostNetwork, a Service's NodePort, a Docker published port)
// rather than a per-container one.
type Networking struct {
	Exposures []Exposure
}

// Exposure is one way a Resource is reachable.
type Exposure struct {
	Type string // port, hostNetwork, publicIP, NodePort, LoadBalancer
	Port int32
}

// HasExposure reports whether any exposure of the given type is present.
func (n Networking) HasExposure(exposureType string) bool {
	for _, e := range n.Exposures {
		if e.Type == exposureType {
			return true
		}
	}
	return false
}

// Flatten exposes exposure types as rule-queryable boolean attributes under
// prefix (e.g. "networking.hostNetwork" -> true/false). The well-known types get
// explicit true/false so "equals: false" works even when not exposed; any other
// exposure type a provider reports gets an implicit true entry so it's still
// queryable without a code change, just without "equals: false" support until
// it's promoted to the well-known list here.
func (n Networking) Flatten(prefix string) map[string]any {
	attrs := map[string]any{
		prefix + ".hostNetwork":  n.HasExposure("hostNetwork"),
		prefix + ".publicIP":     n.HasExposure("publicIP"),
		prefix + ".nodePort":     n.HasExposure("NodePort"),
		prefix + ".loadBalancer": n.HasExposure("LoadBalancer"),
	}
	for _, e := range n.Exposures {
		key := prefix + "." + e.Type
		if _, known := attrs[key]; !known {
			attrs[key] = true
		}
	}
	return attrs
}
