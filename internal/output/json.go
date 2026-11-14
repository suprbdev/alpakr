package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONWriter writes JSON without HTML escaping.
type JSONWriter struct {
	// Indent is the number of spaces per indentation level; 0 writes compact JSON.
	Indent int
	Out    io.Writer

	line *json.Encoder
}

// Write encodes v as a single JSON document followed by a newline.
func (w *JSONWriter) Write(v interface{}) error {
	enc := json.NewEncoder(w.Out)
	enc.SetEscapeHTML(false)
	if w.Indent > 0 {
		enc.SetIndent("", strings.Repeat(" ", w.Indent))
	}
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// WriteOne writes a single record as a JSON line (no indent, newline-terminated).
func (w *JSONWriter) WriteOne(v interface{}) error {
	if w.line == nil {
		w.line = json.NewEncoder(w.Out)
		w.line.SetEscapeHTML(false)
	}
	if err := w.line.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}
