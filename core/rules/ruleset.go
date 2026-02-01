package rules

import (
	"os"
	"path/filepath"
)

func LoadRuleset(path string) ([]Rule, error) {
	var all []Rule

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yaml" {
			continue
		}

		rules, err := Load(filepath.Join(path, f.Name()))
		if err != nil {
			return nil, err
		}
		all = append(all, rules...)
	}

	return all, nil
}
