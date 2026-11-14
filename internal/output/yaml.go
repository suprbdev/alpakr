package output

import (
	"fmt"
	"io"
	"math/big"

	"gopkg.in/yaml.v3"
)

// YAMLWriter writes YAML documents.
type YAMLWriter struct {
	Out io.Writer
}

// Write encodes v as a single YAML document.
func (w *YAMLWriter) Write(v interface{}) error {
	data, err := yaml.Marshal(yamlValue(v))
	if err != nil {
		return fmt.Errorf("encoding YAML: %w", err)
	}
	_, err = w.Out.Write(data)
	return err
}

// WriteOne writes a single record as a YAML document with a separator.
func (w *YAMLWriter) WriteOne(v interface{}) error {
	if _, err := fmt.Fprintln(w.Out, "---"); err != nil {
		return err
	}
	return w.Write(v)
}

// yamlValue returns v with *big.Int values replaced by integer nodes; yaml.v3
// would otherwise marshal them as quoted strings via encoding.TextMarshaler.
func yamlValue(v interface{}) interface{} {
	switch x := v.(type) {
	case *big.Int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: x.String()}
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, e := range x {
			m[k] = yamlValue(e)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(x))
		for i, e := range x {
			a[i] = yamlValue(e)
		}
		return a
	}
	return v
}
