package report

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// PrintSummary writes a scan summary to stderr (backward-compat).
func PrintSummary(findings []Finding, workloadCount, ruleCount int) {
	WriteSummary(os.Stderr, findings, workloadCount, ruleCount)
}

// WriteSummary writes a human-readable scan summary to w.
func WriteSummary(w io.Writer, findings []Finding, workloadCount, ruleCount int) {
	counts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}
	for _, f := range findings {
		counts[strings.ToUpper(f.Severity)]++
	}

	line := strings.Repeat("─", 44)
	fmt.Fprintf(w, "\n%s\n", line)
	fmt.Fprintf(w, "  SCAN SUMMARY\n")
	fmt.Fprintf(w, "%s\n", line)
	fmt.Fprintf(w, "  Workloads scanned : %d\n", workloadCount)
	fmt.Fprintf(w, "  Rules applied     : %d\n", ruleCount)
	fmt.Fprintf(w, "  Total findings    : %d\n", len(findings))
	fmt.Fprintf(w, "%s\n", line)
	fmt.Fprintf(w, "  CRITICAL          : %d\n", counts["CRITICAL"])
	fmt.Fprintf(w, "  HIGH              : %d\n", counts["HIGH"])
	fmt.Fprintf(w, "  MEDIUM            : %d\n", counts["MEDIUM"])
	fmt.Fprintf(w, "  LOW               : %d\n", counts["LOW"])
	fmt.Fprintf(w, "%s\n\n", line)
}
