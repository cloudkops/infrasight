package report

import (
	"io"
	"os"

	"github.com/olekukonko/tablewriter"
)

// PrintTable writes a findings table to stdout (backward-compat).
func PrintTable(findings []Finding) {
	WriteTable(os.Stdout, findings)
}

// WriteTable renders findings as a formatted table to w.
func WriteTable(w io.Writer, findings []Finding) {
	if len(findings) == 0 {
		_, _ = io.WriteString(w, "\n✅  No findings.\n")
		return
	}

	tbl := tablewriter.NewTable(w)
	tbl.Header("Rule", "Severity", "Workload", "Container", "Field", "Title")

	for _, f := range findings {
		_ = tbl.Append(f.RuleID, f.Severity, f.Workload, f.Container, f.Field, f.Title)
	}

	_ = tbl.Render()

	if hasRemediation(findings) {
		_, _ = io.WriteString(w, "\n── Remediation ─────────────────────────────────────────────\n")
		for _, f := range findings {
			if f.Remediation != "" {
				_, _ = io.WriteString(w, "  ["+f.RuleID+"] "+f.Remediation+"\n")
			}
			if f.DocsUrl != "" {
				_, _ = io.WriteString(w, "  └─ Docs: "+f.DocsUrl+"\n")
			}
		}
	}
}

func hasRemediation(findings []Finding) bool {
	for _, f := range findings {
		if f.Remediation != "" || f.DocsUrl != "" {
			return true
		}
	}
	return false
}
