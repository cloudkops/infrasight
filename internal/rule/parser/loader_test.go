package parser

import "testing"

func TestLoadRuleset_SecurityBaseline(t *testing.T) {
	rules, err := LoadRuleset("../../../rulesets/security-baseline", "kubernetes")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}
	if len(rules) != 11 {
		t.Fatalf("expected 11 rules, got %d", len(rules))
	}
	if err := ValidateRules(rules); err != nil {
		t.Fatalf("ValidateRules: %v", err)
	}
}

func TestLoadRuleset_AllKubernetesProfiles(t *testing.T) {
	cases := []struct {
		dir  string
		want int
	}{
		{"../../../rulesets/security-baseline", 11},
		{"../../../rulesets/dev-baseline", 6},
		{"../../../rulesets/strict-runtime", 19},
		{"../../../rulesets/ci-critical", 2},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			rules, err := LoadRuleset(tc.dir, "kubernetes")
			if err != nil {
				t.Fatalf("LoadRuleset: %v", err)
			}
			if len(rules) != tc.want {
				t.Fatalf("expected %d rules, got %d", tc.want, len(rules))
			}
			if err := ValidateRules(rules); err != nil {
				t.Fatalf("ValidateRules: %v", err)
			}
		})
	}
}

func TestLoadRuleset_FiltersByProvider(t *testing.T) {
	rules, err := LoadRuleset("../../../rulesets/security-baseline", "docker")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules for provider=docker, got %d", len(rules))
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("does-not-exist.yaml"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
