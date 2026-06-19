package source

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func parseJSON(data []byte) (interface{}, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return v, nil
}

func parseYAML(data []byte) (interface{}, error) {
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	// yaml.v3 decodes maps as map[string]interface{} which gojq handles fine
	return v, nil
}

func parse(data []byte, format string) (interface{}, error) {
	switch format {
	case "yaml", "yml":
		return parseYAML(data)
	default:
		return parseJSON(data)
	}
}
