package json

import (
	stdjson "encoding/json"
	"io"

	"github.com/cloudkops/infrasight/internal/report"
)

func init() {
	report.Register(renderer{})
}

type renderer struct{}

func (renderer) Name() string { return "json" }

func (renderer) Write(w io.Writer, findings []report.Finding) error {
	enc := stdjson.NewEncoder(w)
	enc.SetIndent("", "  ")
	if findings == nil {
		findings = []report.Finding{}
	}
	return enc.Encode(findings)
}
