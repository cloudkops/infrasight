package sarif

import (
	"encoding/json"
	"io"

	"github.com/cloudkops/infrasight/internal/report"
)

func init() {
	report.Register(renderer{})
}

type renderer struct{}

func (renderer) Name() string { return "sarif" }

// sarifLog is a minimal SARIF 2.1.0 document — enough to dedupe rules and map
// severity to a level, not a full-fidelity implementation of the spec.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	LogicalLocations []sarifLogicalLocation `json:"logicalLocations"`
}

type sarifLogicalLocation struct {
	FullyQualifiedName string `json:"fullyQualifiedName"`
}

func (renderer) Write(w io.Writer, findings []report.Finding) error {
	seenRules := map[string]bool{}
	var rules []sarifRule
	var results []sarifResult

	for _, f := range findings {
		if !seenRules[f.RuleID] {
			seenRules[f.RuleID] = true
			rules = append(rules, sarifRule{ID: f.RuleID, Name: f.Title})
		}

		loc := f.ResourceName
		if f.ContainerName != "" {
			loc = f.ResourceName + "/" + f.ContainerName
		}

		results = append(results, sarifResult{
			RuleID: f.RuleID,
			Level:  severityToLevel(f.Severity),
			Message: sarifMessage{
				Text: f.Title,
			},
			Locations: []sarifLocation{{
				LogicalLocations: []sarifLogicalLocation{{FullyQualifiedName: loc}},
			}},
		})
	}

	doc := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{Name: "infrasight", Rules: rules}},
			Results: func() []sarifResult {
				if results == nil {
					return []sarifResult{}
				}
				return results
			}(),
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func severityToLevel(severity string) string {
	switch severity {
	case "CRITICAL", "HIGH":
		return "error"
	case "MEDIUM":
		return "warning"
	default:
		return "note"
	}
}
