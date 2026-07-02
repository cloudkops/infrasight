package parser

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a single rules YAML file and validates every rule it contains.
func Load(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("parser: read %s: %w", path, err)
	}

	var rules []Rule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parser: parse %s: %w", path, err)
	}

	if err := ValidateRules(rules); err != nil {
		return nil, fmt.Errorf("parser: %s: %w", path, err)
	}
	return rules, nil
}
