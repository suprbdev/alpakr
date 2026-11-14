package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	return encodeJSON(enc, v)
}

// WriteOne writes a single record as a JSON line (no indent, newline-terminated).
func (w *JSONWriter) WriteOne(v interface{}) error {
	if w.line == nil {
		w.line = json.NewEncoder(w.Out)
		w.line.SetEscapeHTML(false)
	}
	return encodeJSON(w.line, v)
}

// encodeJSON encodes v, retrying with NaN and infinities replaced as jq
// prints them (null and ±1.7976931348623157e+308) if JSON rejects them.
func encodeJSON(enc *json.Encoder, v interface{}) error {
	err := enc.Encode(v)
	var unsupported *json.UnsupportedValueError
	if errors.As(err, &unsupported) {
		err = enc.Encode(finiteFloats(v))
	}
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

func finiteFloats(v interface{}) interface{} {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) {
			return nil
		}
		if math.IsInf(x, 0) {
			return math.Copysign(math.MaxFloat64, x)
		}
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, e := range x {
			m[k] = finiteFloats(e)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(x))
		for i, e := range x {
			a[i] = finiteFloats(e)
		}
		return a
	}
	return v
}
