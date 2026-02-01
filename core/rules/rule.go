package rules

type Rule struct {
	ID          string    `yaml:"id"`
	Title       string    `yaml:"title"`
	Severity    string    `yaml:"severity"`
	Category    string    `yaml:"category,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Remediation string    `yaml:"remediation,omitempty"`
	DocsUrl     string    `yaml:"docs_url,omitempty"`
	Condition   Condition `yaml:"condition"`
}
type Condition struct {
	Field  string      `yaml:"field"`
	Equals interface{} `yaml:"equals,omitempty"`
	Exists bool        `yaml:"exists,omitempty"`
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
