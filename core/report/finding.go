package report

type Finding struct {
	RuleID    string `json:"rule_id"`
	Title     string `json:"title"`
	Severity  string `json:"severity"`
	Workload  string `json:"workload"`
	Container string `json:"container"`
	Field     string `json:"field"`
}
