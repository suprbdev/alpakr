package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type JSONWriter struct {
	Indent int
	Out    io.Writer
}

func (w *JSONWriter) Write(v interface{}) error {
	prefix := ""
	indent := strings.Repeat(" ", w.Indent)
	data, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Out.Write(data)
	return err
}
