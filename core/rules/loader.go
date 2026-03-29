package rules

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules []Rule
	err = yaml.Unmarshal(data, &rules)
	if err != nil {
		return nil, err
	}

	err = ValidateRules(rules)
	if err != nil {
		return nil, err
	}

	return rules, nil
}
