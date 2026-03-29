package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "rules-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

const validRulesYAML = `
- id: ISG-TEST-001
  title: Container running as root
  severity: HIGH
  category: security
  description: Containers running as root increase blast radius.
  remediation: Set runAsUser to a non-root UID.
  docs_url: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
  condition:
    field: container.user
    equals: 0

- id: ISG-TEST-002
  title: Privileged container
  severity: CRITICAL
  condition:
    field: container.privileged
    equals: true
`

func TestLoad_ValidFile(t *testing.T) {
	path := writeTempYAML(t, validRulesYAML)
	rules, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].ID != "ISG-TEST-001" {
		t.Errorf("rules[0].ID = %q, want ISG-TEST-001", rules[0].ID)
	}
	if rules[1].ID != "ISG-TEST-002" {
		t.Errorf("rules[1].ID = %q, want ISG-TEST-002", rules[1].ID)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/tmp/infrasight-does-not-exist-xyz.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempYAML(t, ":::not valid yaml:::")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoad_InvalidRule_MissingID(t *testing.T) {
	yaml := `
- title: No ID rule
  severity: HIGH
  condition:
    field: container.user
    equals: 0
`
	path := writeTempYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for missing ID, got nil")
	}
}

func TestLoad_InvalidRule_BadSeverity(t *testing.T) {
	yaml := `
- id: ISG-BAD-001
  title: Bad severity
  severity: BLOCKER
  condition:
    field: container.user
    equals: 0
`
	path := writeTempYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for bad severity, got nil")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeTempYAML(t, "")
	rules, err := Load(path)
	if err != nil {
		t.Errorf("unexpected error for empty file: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for empty file, got %d", len(rules))
	}
}

func TestLoadRuleset_ValidDirectory(t *testing.T) {
	dir := t.TempDir()

	file1 := `
- id: R1
  title: Rule 1
  severity: HIGH
  condition:
    field: container.user
    equals: 0
`
	file2 := `
- id: R2
  title: Rule 2
  severity: CRITICAL
  condition:
    field: container.privileged
    equals: true
`
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(file1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(file2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a rule"), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadRuleset(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestLoadRuleset_DirectoryNotFound(t *testing.T) {
	_, err := LoadRuleset("/tmp/infrasight-ruleset-does-not-exist")
	if err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestLoadRuleset_IgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# docs"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadRuleset(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules (non-yaml files ignored), got %d", len(rules))
	}
}

func TestLoadRuleset_InvalidRuleInOneFile(t *testing.T) {
	dir := t.TempDir()

	good := `
- id: GOOD
  title: Good rule
  severity: HIGH
  condition:
    field: container.user
    equals: 0
`
	bad := `
- title: No ID
  severity: HIGH
  condition:
    field: container.user
    equals: 0
`
	if err := os.WriteFile(filepath.Join(dir, "good.yaml"), []byte(good), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRuleset(dir)
	if err == nil {
		t.Error("expected error when a file in ruleset contains invalid rule, got nil")
	}
}
