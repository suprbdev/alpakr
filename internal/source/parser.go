package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ParseJSON decodes data holding exactly one JSON value, with integers kept
// exact.
func ParseJSON(data []byte) (interface{}, error) {
	return parseJSON(data)
}

func parseJSON(data []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing JSON: unexpected data after top-level value at offset %d", dec.InputOffset())
	}
	return normalizeJSON(v), nil
}
