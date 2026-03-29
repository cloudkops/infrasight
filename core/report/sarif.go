package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const sarifSchema = "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json"

// WriteSARIF writes SARIF 2.1 output to w.
func WriteSARIF(w io.Writer, findings []Finding) error {
	report := buildSARIF(findings)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// PrintSARIF writes SARIF to stdout (backward-compat).
func PrintSARIF(findings []Finding) {
	_ = WriteSARIF(os.Stdout, findings)
}

func buildSARIF(findings []Finding) sarifReport {
	rules := deduplicateSARIFRules(findings)
	results := make([]sarifResult, 0, len(findings))
	for _, f := range findings {
		results = append(results, sarifResult{
			RuleID: f.RuleID,
			Level:  severityToSARIFLevel(f.Severity),
			Message: sarifText{
				Text: fmt.Sprintf("[%s] %s in workload=%q container=%q", f.Severity, f.Title, f.Workload, f.Container),
			},
			Locations: []sarifLocation{
				{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{
							URI: fmt.Sprintf("k8s://%s/%s", f.Workload, f.Container),
						},
					},
				},
			},
		})
	}

	return sarifReport{
		Schema:  sarifSchema,
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:           "infrasight",
						Version:        "0.0.1",
						InformationURI: "https://github.com/cloudkops/infrasight",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}
}

func deduplicateSARIFRules(findings []Finding) []sarifRule {
	seen := map[string]bool{}
	var rules []sarifRule
	for _, f := range findings {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true
		rules = append(rules, sarifRule{
			ID:   f.RuleID,
			Name: titleToName(f.Title),
			ShortDescription: sarifText{
				Text: f.Title,
			},
			FullDescription: sarifText{
				Text: f.Description,
			},
			HelpURI: f.DocsUrl,
			Properties: sarifRuleProps{
				Severity: f.Severity,
				Category: f.Category,
			},
		})
	}
	return rules
}

func titleToName(title string) string {
	words := strings.Fields(title)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

func severityToSARIFLevel(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL", "HIGH":
		return "error"
	case "MEDIUM":
		return "warning"
	default:
		return "note"
	}
}

// ── SARIF structs ─────────────────────────────────────────────────────────────

type sarifReport struct {
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
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ShortDescription sarifText      `json:"shortDescription"`
	FullDescription  sarifText      `json:"fullDescription,omitempty"`
	HelpURI          string         `json:"helpUri,omitempty"`
	Properties       sarifRuleProps `json:"properties,omitempty"`
}

type sarifRuleProps struct {
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}
