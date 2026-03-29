package rules

type Rule struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Severity    string `yaml:"severity"`
	Category    string `yaml:"category,omitempty"`
	Description string `yaml:"description,omitempty"`
	Remediation string `yaml:"remediation,omitempty"`
	DocsUrl     string `yaml:"docs_url,omitempty"`
	// Single condition (backward-compatible)
	Condition Condition `yaml:"condition,omitempty"`
	// Compound: ALL must match
	Conditions []Condition `yaml:"conditions,omitempty"`
	// Compound: ANY must match
	AnyOf []Condition `yaml:"any_of,omitempty"`
}

type Condition struct {
	Field     string      `yaml:"field"`
	Equals    interface{} `yaml:"equals,omitempty"`
	NotEquals interface{} `yaml:"not_equals,omitempty"`
	Contains  string      `yaml:"contains,omitempty"`
	Exists    bool        `yaml:"exists,omitempty"`
}

/*

example rules yaml:
id: 1
Title: THIS IS A RULE
Severity: HIGH
Condition:
  Field: container.image
  Equals: nginx:latest
  Exists: true

*/
