package config

import "fmt"

type Config struct {
	Version  string                   `yaml:"version"`
	Source   SourceConfig             `yaml:"source"`            // default source (single-source configs)
	Sources  map[string]SourceConfig  `yaml:"sources"`           // named sources map
	Output   OutputConfig             `yaml:"output"`
	Handlers map[string]HandlerConfig `yaml:"handlers"`
}

type SourceConfig struct {
	Path    string            `yaml:"path"`
	URL     string            `yaml:"url"`
	Format  string            `yaml:"format"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
	Indent int    `yaml:"indent"`
	File   string `yaml:"file"`
}

type HandlerConfig struct {
	Source string                 `yaml:"source"` // key into Config.Sources; empty = use Config.Source
	Input  string                 `yaml:"input"`
	Each   bool                   `yaml:"each"`
	Filter string                 `yaml:"filter"`
	Fields map[string]FieldConfig `yaml:"fields"`
}

// FieldConfig is a union: a jq expression string, a sub-handler reference, or an inline nested object.
type FieldConfig struct {
	Expr    string
	Handler string
	Input   string
	Fields  map[string]FieldConfig // inline nested object
}

func (f *FieldConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try plain string first (jq expression)
	var expr string
	if err := unmarshal(&expr); err == nil {
		f.Expr = expr
		return nil
	}

	// Decode as raw map to inspect dispatch keys
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("field must be a jq expression string, {handler, input} mapping, or {input, fields} inline object")
	}

	_, hasHandler := raw["handler"]
	_, hasFields := raw["fields"]

	if hasHandler && hasFields {
		return fmt.Errorf("field cannot have both 'handler' and 'fields' keys")
	}

	// Sub-handler reference: keyed by 'handler'
	if hasHandler {
		h, ok := raw["handler"].(string)
		if !ok || h == "" {
			return fmt.Errorf("field 'handler' must be a non-empty string")
		}
		f.Handler = h
		if inp, ok := raw["input"].(string); ok {
			f.Input = inp
		}
		return nil
	}

	// Inline nested object: keyed by 'fields'
	if hasFields {
		if inp, ok := raw["input"].(string); ok {
			f.Input = inp
		}
		// Re-unmarshal just the 'fields' value into map[string]FieldConfig
		type inlineShape struct {
			Input  string                 `yaml:"input"`
			Fields map[string]FieldConfig `yaml:"fields"`
		}
		var shape inlineShape
		if err := unmarshal(&shape); err != nil {
			return fmt.Errorf("inline fields: %w", err)
		}
		if len(shape.Fields) == 0 {
			return fmt.Errorf("inline 'fields' must be a non-empty map")
		}
		f.Fields = shape.Fields
		return nil
	}

	return fmt.Errorf("field map must have a 'handler' key (sub-handler) or a 'fields' key (inline object)")
}
