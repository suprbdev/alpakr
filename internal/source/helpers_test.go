package source

import (
	"bytes"
	"fmt"
	"io"
)

// readAll returns every value NewValues yields for r.
func readAll(r io.Reader, format string) ([]interface{}, error) {
	vals, err := NewValues(r, format)
	if err != nil {
		return nil, err
	}
	var out []interface{}
	for {
		v, ok, err := vals.Next()
		if err != nil {
			return out, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, v)
	}
}

// readOne returns the single value in r, failing if there is more than one.
func readOne(r io.Reader, format string) (interface{}, error) {
	all, err := readAll(r, format)
	if err != nil {
		return nil, err
	}
	if len(all) != 1 {
		return nil, fmt.Errorf("got %d values, want 1", len(all))
	}
	return all[0], nil
}

func parse(data []byte, format string) (interface{}, error) {
	return readOne(bytes.NewReader(data), format)
}

func parseYAML(data []byte) (interface{}, error) { return parse(data, "yaml") }
