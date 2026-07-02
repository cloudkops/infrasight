package markdown

import (
	"fmt"
	"io"

	"github.com/cloudkops/infrasight/internal/report"
)

func init() {
	report.Register(renderer{})
}

type renderer struct{}

func (renderer) Name() string { return "markdown" }

func (renderer) Write(w io.Writer, findings []report.Finding) error {
	if len(findings) == 0 {
		_, err := fmt.Fprintln(w, "No findings.")
		return err
	}

	fmt.Fprintln(w, "| Severity | Rule | Title | Resource | Container | Field |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|")
	for _, f := range findings {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s |\n",
			f.Severity, f.RuleID, f.Title, f.ResourceName, f.ContainerName, f.Field)
	}
	return nil
}
