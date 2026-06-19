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
	Path   string `yaml:"path"`
	URL    string `yaml:"url"`
	Format string `yaml:"format"`
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

// FieldConfig is a union: either a jq expression string, or a sub-handler reference.
type FieldConfig struct {
	Expr    string
	Handler string
	Input   string
}

func (f *FieldConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try plain string first (jq expression)
	var expr string
	if err := unmarshal(&expr); err == nil {
		f.Expr = expr
		return nil
	}

	// Try mapping with handler/input keys
	var m map[string]string
	if err := unmarshal(&m); err != nil {
		return fmt.Errorf("field must be a jq expression string or {handler, input} mapping")
	}

	handler, ok := m["handler"]
	if !ok || handler == "" {
		return fmt.Errorf("field mapping must have a non-empty 'handler' key")
	}
	f.Handler = handler
	f.Input = m["input"]
	return nil
}
