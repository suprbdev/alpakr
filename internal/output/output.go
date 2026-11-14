// Package output encodes transformation results as JSON or YAML.
package output

import (
	"fmt"
	"io"
	"strings"
)

// Writer encodes results to an underlying io.Writer.
type Writer interface {
	// Write encodes v as a complete document.
	Write(v interface{}) error
	// WriteOne encodes v as one record of a stream (one JSON line or one YAML document).
	WriteOne(v interface{}) error
}

// CheckFormat reports whether format is a supported output format.
func CheckFormat(format string) error {
	switch strings.ToLower(format) {
	case "json", "yaml", "yml", "ndjson", "jsonl":
		return nil
	}
	return fmt.Errorf("unknown output format %q (want json, yaml or ndjson)", format)
}

// New returns a Writer for format that writes to out: "json", "yaml" (or
// "yml"), or "ndjson" (or "jsonl"). indent sets JSON indentation in spaces;
// 0 produces compact output. It does not apply to ndjson.
func New(format string, indent int, out io.Writer) (Writer, error) {
	if err := CheckFormat(format); err != nil {
		return nil, err
	}
	switch strings.ToLower(format) {
	case "json":
		return &JSONWriter{Indent: indent, Out: out}, nil
	case "ndjson", "jsonl":
		return &NDJSONWriter{JSONWriter{Out: out}}, nil
	}
	return &YAMLWriter{Out: out}, nil
}

// NDJSONWriter writes one compact JSON value per line.
type NDJSONWriter struct {
	JSONWriter
}

// Write writes each element of an array result as its own line, or a
// single non-array result as one line. A nil result (a record rejected by
// the handler's filter) writes nothing.
func (w *NDJSONWriter) Write(v interface{}) error {
	switch x := v.(type) {
	case nil:
		return nil
	case []interface{}:
		for _, e := range x {
			if err := w.WriteOne(e); err != nil {
				return err
			}
		}
		return nil
	}
	return w.WriteOne(v)
}

// WriteOne writes a streamed record: an array result (from an 'each'
// handler) is written one element per line, like Write.
func (w *NDJSONWriter) WriteOne(v interface{}) error {
	if a, ok := v.([]interface{}); ok {
		for _, e := range a {
			if err := w.JSONWriter.WriteOne(e); err != nil {
				return err
			}
		}
		return nil
	}
	return w.JSONWriter.WriteOne(v)
}
