package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type JSONWriter struct {
	Indent int
	Out    io.Writer
}

func (w *JSONWriter) Write(v interface{}) error {
	prefix := ""
	indent := strings.Repeat(" ", w.Indent)
	data, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Out.Write(data)
	return err
}

// WriteOne writes a single record as a JSON line (no indent, newline-terminated).
func (w *JSONWriter) WriteOne(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Out.Write(data)
	return err
}
