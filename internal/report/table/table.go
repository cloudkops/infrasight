package table

import (
	"io"

	"github.com/cloudkops/infrasight/internal/report"
	"github.com/olekukonko/tablewriter"
)

func init() {
	report.Register(renderer{})
}

type renderer struct{}

func (renderer) Name() string { return "table" }

func (renderer) Write(w io.Writer, findings []report.Finding) error {
	t := tablewriter.NewWriter(w)
	t.Header("SEVERITY", "RULE", "TITLE", "RESOURCE", "CONTAINER", "FIELD")

	for _, f := range findings {
		if err := t.Append([]string{f.Severity, f.RuleID, f.Title, f.ResourceName, f.ContainerName, f.Field}); err != nil {
			return err
		}
	}
	return t.Render()
}
