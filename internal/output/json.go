package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/suprbdev/alpakr/internal/record"
)

// JSONWriter writes JSON without HTML escaping. Objects produced by handlers
// keep their config field order.
type JSONWriter struct {
	// Indent is the number of spaces per indentation level; 0 writes compact JSON.
	Indent int
	Out    io.Writer

	buf    []byte
	indent bytes.Buffer
}

// Write encodes v as a single JSON document followed by a newline.
func (w *JSONWriter) Write(v interface{}) error {
	return w.write(v, w.Indent)
}

// WriteOne writes a single record as a JSON line (no indent, newline-terminated).
func (w *JSONWriter) WriteOne(v interface{}) error {
	return w.write(v, 0)
}

func (w *JSONWriter) write(v interface{}, indent int) error {
	data, err := encodeJSON(w.buf[:0], v)
	if err != nil {
		return err
	}
	w.buf = data
	if indent > 0 {
		w.indent.Reset()
		if err := json.Indent(&w.indent, data, "", strings.Repeat(" ", indent)); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
		data = w.indent.Bytes()
	}
	if _, err := w.Out.Write(data); err != nil {
		return err
	}
	_, err = w.Out.Write([]byte{'\n'})
	return err
}

// EncodeLine appends the compact JSON line WriteOne(v) would write.
func (w *JSONWriter) EncodeLine(dst []byte, v interface{}) ([]byte, error) {
	dst, err := encodeJSON(dst, v)
	if err != nil {
		return nil, err
	}
	return append(dst, '\n'), nil
}

// WriteEncoded writes bytes produced by EncodeLine.
func (w *JSONWriter) WriteEncoded(p []byte) error {
	_, err := w.Out.Write(p)
	return err
}

// encodeJSON appends the compact encoding of v to dst, retrying with NaN and
// infinities replaced as jq prints them (null and ±1.7976931348623157e+308)
// if JSON rejects them.
func encodeJSON(dst []byte, v interface{}) ([]byte, error) {
	data, err := record.AppendJSON(dst, v)
	var unsupported *json.UnsupportedValueError
	if errors.As(err, &unsupported) {
		data, err = record.AppendJSON(dst, finiteFloats(v))
	}
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return data, nil
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
	case *record.Object:
		o := &record.Object{Keys: x.Keys, Values: make([]interface{}, len(x.Values))}
		for i, e := range x.Values {
			o.Values[i] = finiteFloats(e)
		}
		return o
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
