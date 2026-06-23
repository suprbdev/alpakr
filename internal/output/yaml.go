package output

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type YAMLWriter struct {
	Out io.Writer
}

func (w *YAMLWriter) Write(v interface{}) error {
	data, err := yaml.Marshal(v)
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
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding YAML: %w", err)
	}
	_, err = w.Out.Write(data)
	return err
}
