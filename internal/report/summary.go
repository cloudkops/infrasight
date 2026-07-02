package report

import (
	"fmt"
	"io"

	"github.com/cloudkops/infrasight/internal/rule/parser"
)

// WriteSummary prints a human-readable severity/count summary.
func WriteSummary(w io.Writer, findings []Finding, resourceCount, ruleCount int) {
	counts := map[string]int{
		parser.SeverityLow:      0,
		parser.SeverityMedium:   0,
		parser.SeverityHigh:     0,
		parser.SeverityCritical: 0,
	}
	for _, f := range findings {
		counts[parser.NormalizeSeverity(f.Severity)]++
	}

	fmt.Fprintf(w, "Scanned %d resource(s) against %d rule(s)\n", resourceCount, ruleCount)
	fmt.Fprintf(w, "Findings: %d total (CRITICAL=%d, HIGH=%d, MEDIUM=%d, LOW=%d)\n",
		len(findings), counts[parser.SeverityCritical], counts[parser.SeverityHigh],
		counts[parser.SeverityMedium], counts[parser.SeverityLow])
}
