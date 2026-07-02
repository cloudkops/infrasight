package report

import "io"

// Renderer writes findings in one output format. table/json/sarif/markdown each
// implement this and register under their format name (see registry.go) instead of
// internal/app hardcoding a format switch.
type Renderer interface {
	Name() string
	Write(w io.Writer, findings []Finding) error
}
