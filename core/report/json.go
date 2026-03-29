package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JsonFormat writes compact JSON to stdout (backward-compat).
func JsonFormat(findings []Finding) {
	_ = WriteJSON(os.Stdout, findings, false)
}

// WriteJSON writes findings as JSON to w. Set pretty=true for indented output.
func WriteJSON(w io.Writer, findings []Finding, pretty bool) error {
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(findings, "", "  ")
	} else {
		data, err = json.Marshal(findings)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
