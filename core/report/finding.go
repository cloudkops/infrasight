package report

type Finding struct {
	RuleID      string `json:"rule_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	DocsUrl     string `json:"docs_url,omitempty"`
	Workload    string `json:"workload"`
	Container   string `json:"container"`
	Field       string `json:"field"`
}
