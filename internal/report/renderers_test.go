package report_test

import (
	"bytes"
	"testing"

	"github.com/cloudkops/infrasight/internal/report"

	_ "github.com/cloudkops/infrasight/internal/report/json"
	_ "github.com/cloudkops/infrasight/internal/report/markdown"
	_ "github.com/cloudkops/infrasight/internal/report/sarif"
	_ "github.com/cloudkops/infrasight/internal/report/table"
)

func TestAllRenderersRegisteredAndWrite(t *testing.T) {
	findings := []report.Finding{
		{RuleID: "ISG-SEC-002", Title: "Privileged container", Severity: "CRITICAL", ResourceName: "db", ContainerName: "mysql", Field: "container.privileged"},
	}
	for _, name := range []string{"table", "json", "sarif", "markdown"} {
		r, err := report.Get(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var buf bytes.Buffer
		if err := r.Write(&buf, findings); err != nil {
			t.Fatalf("%s: write: %v", name, err)
		}
		if buf.Len() == 0 {
			t.Fatalf("%s: expected non-empty output", name)
		}
		t.Logf("=== %s ===\n%s", name, buf.String())
	}
}
