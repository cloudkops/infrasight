package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadRuleset loads every *.yaml/*.yml file in dir and concatenates their rules.
// Non-YAML files are skipped. If provider is non-empty, rules are filtered to only
// those with a matching Rule.Provider tag or no tag at all (applies everywhere).
func LoadRuleset(dir string, activeProvider string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("parser: read ruleset dir %s: %w", dir, err)
	}

	var all []Rule
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		rules, err := Load(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		all = append(all, rules...)
	}

	if activeProvider == "" {
		return all, nil
	}

	filtered := make([]Rule, 0, len(all))
	for _, r := range all {
		if r.Provider == "" || r.Provider == activeProvider {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}
