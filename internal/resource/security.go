package resource

// SecurityContext carries cross-provider security signals. Pointer fields
// distinguish "not set by the provider" from an explicit false.
type SecurityContext struct {
	RunAsRoot                bool
	Privileged               bool
	AllowPrivilegeEscalation *bool
	ReadOnlyRootFilesystem   *bool
	CapabilitiesAdd          []string
	CapabilitiesDrop         []string
	Public                   bool
	Encrypted                *bool
}

// Flatten exposes every populated field as a rule-queryable attribute under the
// given prefix (e.g. "resource.security" or "container.security"). This is the
// single place to update when a new security signal is added — internal/rule
// never needs a dedicated case for it; internal/scan/pipeline's EnrichAttributes
// stage merges this into Resource/Container Attributes for every provider.
func (s SecurityContext) Flatten(prefix string) map[string]any {
	attrs := map[string]any{
		prefix + ".run_as_root": s.RunAsRoot,
		prefix + ".privileged":  s.Privileged,
		prefix + ".public":      s.Public,
	}
	if s.AllowPrivilegeEscalation != nil {
		attrs[prefix+".allow_privilege_escalation"] = *s.AllowPrivilegeEscalation
	}
	if s.ReadOnlyRootFilesystem != nil {
		attrs[prefix+".read_only_root_filesystem"] = *s.ReadOnlyRootFilesystem
	}
	if s.Encrypted != nil {
		attrs[prefix+".encrypted"] = *s.Encrypted
	}
	if len(s.CapabilitiesAdd) > 0 {
		attrs[prefix+".capabilities_add"] = s.CapabilitiesAdd
	}
	if len(s.CapabilitiesDrop) > 0 {
		attrs[prefix+".capabilities_drop"] = s.CapabilitiesDrop
	}
	return attrs
}
