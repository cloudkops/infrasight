package parser

// Rule is one security/compliance/governance check, loaded from YAML.
type Rule struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Severity    string `yaml:"severity"`
	Category    string `yaml:"category,omitempty"`
	Description string `yaml:"description,omitempty"`
	Remediation string `yaml:"remediation,omitempty"`
	DocsUrl     string `yaml:"docs_url,omitempty"`
	// Provider scopes the rule to a single provider name (kubernetes, docker, ...).
	// Empty means the rule applies to every provider whose fields it references.
	Provider string `yaml:"provider,omitempty"`

	// Single condition (backward-compatible with a one-condition rule).
	Condition Condition `yaml:"condition,omitempty"`
	// Compound: ALL conditions must match.
	Conditions []Condition `yaml:"conditions,omitempty"`
	// Compound: ANY condition must match.
	AnyOf []Condition `yaml:"any_of,omitempty"`
}

// Condition is one field-match predicate. Exactly one of Equals/NotEquals/Contains
// is expected to be set, or Exists used alone; internal/rule/evaluator resolves
// which Operator applies from whichever field is populated.
type Condition struct {
	Field     string      `yaml:"field"`
	Equals    interface{} `yaml:"equals,omitempty"`
	NotEquals interface{} `yaml:"not_equals,omitempty"`
	Contains  string      `yaml:"contains,omitempty"`
	Exists    bool        `yaml:"exists,omitempty"`
}
