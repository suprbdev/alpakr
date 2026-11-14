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

// LineEncoder is implemented by writers whose streamed records are
// independent lines, so records can be encoded concurrently and written in
// order afterwards.
type LineEncoder interface {
	// EncodeLine appends to dst exactly what WriteOne(v) would write. It
	// does not touch the writer's state and is safe for concurrent use.
	EncodeLine(dst []byte, v interface{}) ([]byte, error)
	// WriteEncoded writes bytes produced by EncodeLine.
	WriteEncoded(p []byte) error
}

// CheckFormat reports whether format is a supported output format.
func CheckFormat(format string) error {
	switch strings.ToLower(format) {
	case "json", "yaml", "yml", "ndjson", "jsonl", "csv":
		return nil
	}
	return fmt.Errorf("unknown output format %q (want json, yaml, ndjson or csv)", format)
}

// Option configures New.
type Option func(*options)

type options struct {
	delimiter rune
}

// Delimiter sets the CSV field delimiter (default ',').
func Delimiter(r rune) Option { return func(o *options) { o.delimiter = r } }

// New returns a Writer for format that writes to out: "json", "yaml" (or
// "yml"), "ndjson" (or "jsonl") or "csv". indent sets JSON indentation in
// spaces; 0 produces compact output. It does not apply to ndjson or csv.
func New(format string, indent int, out io.Writer, opts ...Option) (Writer, error) {
	if err := CheckFormat(format); err != nil {
		return nil, err
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	switch strings.ToLower(format) {
	case "csv":
		return NewCSVWriter(out, o.delimiter), nil
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

// EncodeLine appends the lines WriteOne(v) would write.
func (w *NDJSONWriter) EncodeLine(dst []byte, v interface{}) ([]byte, error) {
	a, ok := v.([]interface{})
	if !ok {
		return w.JSONWriter.EncodeLine(dst, v)
	}
	var err error
	for _, e := range a {
		if dst, err = w.JSONWriter.EncodeLine(dst, e); err != nil {
			return nil, err
		}
	}
	return dst, nil
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
