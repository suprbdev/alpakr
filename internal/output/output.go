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
	case "json", "yaml", "yml":
		return nil
	}
	return fmt.Errorf("unknown output format %q (want json or yaml)", format)
}

// New returns a Writer for format ("json", "yaml" or "yml") that writes to out.
// indent sets JSON indentation in spaces; 0 produces compact output.
func New(format string, indent int, out io.Writer) (Writer, error) {
	if err := CheckFormat(format); err != nil {
		return nil, err
	}
	if strings.ToLower(format) == "json" {
		return &JSONWriter{Indent: indent, Out: out}, nil
	}
	return &YAMLWriter{Out: out}, nil
}
