package report

// Finding is the output of one matched rule, shared across every renderer.
type Finding struct {
	RuleID        string `json:"rule_id"`
	Title         string `json:"title"`
	Severity      string `json:"severity"`
	Category      string `json:"category,omitempty"`
	Description   string `json:"description,omitempty"`
	Remediation   string `json:"remediation,omitempty"`
	DocsUrl       string `json:"docs_url,omitempty"`
	ResourceName  string `json:"resource"`
	ResourceKind  string `json:"resource_kind"`
	Namespace     string `json:"namespace,omitempty"`
	ContainerName string `json:"container,omitempty"`
	Field         string `json:"field"`
}
