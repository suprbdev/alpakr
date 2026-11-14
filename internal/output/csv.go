package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"

	"github.com/suprbdev/alpakr/internal/record"
)

// CSVWriter writes object records as CSV rows. The header row is taken from
// the first record's fields, in config order (sorted for objects built by
// jq). Nested values are written as compact JSON.
type CSVWriter struct {
	w      *csv.Writer
	header []string
	index  map[string]int
}

// NewCSVWriter returns a CSVWriter using delimiter (',' when zero).
func NewCSVWriter(out io.Writer, delimiter rune) *CSVWriter {
	w := csv.NewWriter(out)
	if delimiter != 0 {
		w.Comma = delimiter
	}
	return &CSVWriter{w: w}
}

// Write writes an array result one row per element, or a single object as
// one row. A nil result writes nothing.
func (c *CSVWriter) Write(v interface{}) error {
	return c.WriteOne(v)
}

// WriteOne writes a record as a row, or each element of an array result as
// a row.
func (c *CSVWriter) WriteOne(v interface{}) error {
	switch x := v.(type) {
	case nil:
		return nil
	case []interface{}:
		for _, e := range x {
			if err := c.row(e); err != nil {
				return err
			}
		}
	default:
		if err := c.row(v); err != nil {
			return err
		}
	}
	c.w.Flush()
	return c.w.Error()
}

func (c *CSVWriter) row(v interface{}) error {
	if v == nil {
		return nil
	}
	keys, get, err := csvFields(v)
	if err != nil {
		return err
	}
	if c.header == nil {
		c.header = keys
		c.index = make(map[string]int, len(keys))
		for i, k := range keys {
			c.index[k] = i
		}
		if err := c.w.Write(keys); err != nil {
			return err
		}
	}
	cells := make([]string, len(c.header))
	if o, ok := v.(*record.Object); ok && sameKeys(o.Keys, c.header) {
		for i, val := range o.Values {
			if cells[i], err = csvCell(val); err != nil {
				return err
			}
		}
		return c.w.Write(cells)
	}
	for _, k := range keys {
		i, ok := c.index[k]
		if !ok {
			return fmt.Errorf("encoding CSV: record field %q is not in the header (columns come from the first record)", k)
		}
		if cells[i], err = csvCell(get(k)); err != nil {
			return err
		}
	}
	return c.w.Write(cells)
}

// sameKeys reports whether a and b are the same shared key slice, as for
// objects built from one handler's field list.
func sameKeys(a, b []string) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

func csvFields(v interface{}) ([]string, func(string) interface{}, error) {
	switch x := v.(type) {
	case *record.Object:
		return x.Keys, func(k string) interface{} { val, _ := x.Get(k); return val }, nil
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys, func(k string) interface{} { return x[k] }, nil
	}
	return nil, nil, fmt.Errorf("encoding CSV: records must be objects, got %T", v)
}

func csvCell(v interface{}) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case *big.Int:
		return x.String(), nil
	}
	data, err := encodeJSON(nil, v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
