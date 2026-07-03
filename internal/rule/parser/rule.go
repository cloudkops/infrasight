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

// Condition is one field-match predicate. Exactly one operator key is expected to
// be set, or Exists used alone; internal/rule/evaluator resolves which Operator
// applies from whichever field is populated (see its documented precedence when
// more than one is set — this struct enforces nothing at the type level).
type Condition struct {
	Field string `yaml:"field"`

	Equals    interface{} `yaml:"equals,omitempty"`
	NotEquals interface{} `yaml:"not_equals,omitempty"`
	Exists    bool        `yaml:"exists,omitempty"`

	Contains    string `yaml:"contains,omitempty"`
	NotContains string `yaml:"not_contains,omitempty"`
	StartsWith  string `yaml:"starts_with,omitempty"`
	EndsWith    string `yaml:"ends_with,omitempty"`
	Regex       string `yaml:"regex,omitempty"`

	In    []interface{} `yaml:"in,omitempty"`
	NotIn []interface{} `yaml:"not_in,omitempty"`

	GreaterThan        interface{} `yaml:"greater_than,omitempty"`
	LessThan           interface{} `yaml:"less_than,omitempty"`
	GreaterThanOrEqual interface{} `yaml:"greater_than_or_equal,omitempty"`
	LessThanOrEqual    interface{} `yaml:"less_than_or_equal,omitempty"`
}
