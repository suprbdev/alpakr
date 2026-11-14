package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

func parseJSON(data []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("parsing JSON: unexpected data after top-level value at offset %d", dec.InputOffset())
	}
	return normalizeJSON(v), nil
}

func parseYAML(data []byte) (interface{}, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	v, err := yamlToValue(&doc)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	return v, nil
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func trimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, utf8BOM)
}

func parse(data []byte, format string) (interface{}, error) {
	data = trimBOM(data)
	switch format {
	case "yaml", "yml":
		return parseYAML(data)
	case "ndjson":
		return (&NdjsonSource{Reader: bytes.NewReader(data)}).Load()
	default:
		return parseJSON(data)
	}
}
