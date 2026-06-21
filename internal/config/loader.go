package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validate(&cfg, path); err != nil {
		return nil, err
	}

	applyDefaults(&cfg, path)
	return &cfg, nil
}

func validate(cfg *Config, configPath string) error {
	hasDefaultSource := cfg.Source.Path != "" || cfg.Source.URL != ""

	if !hasDefaultSource && len(cfg.Sources) == 0 {
		return fmt.Errorf("config: must have 'source' or at least one entry in 'sources'")
	}
	if err := validateSourceConfig("source", cfg.Source); err != nil {
		return err
	}
	for name, s := range cfg.Sources {
		if s.Path == "" && s.URL == "" {
			return fmt.Errorf("config: sources.%s must have 'path' or 'url'", name)
		}
		if err := validateSourceConfig("sources."+name, s); err != nil {
			return err
		}
	}
	if len(cfg.Handlers) == 0 {
		return fmt.Errorf("config: handlers must be non-empty")
	}

	for name, h := range cfg.Handlers {
		if h.Source != "" {
			if _, ok := cfg.Sources[h.Source]; !ok {
				return fmt.Errorf("config: handler %q references undefined source %q", name, h.Source)
			}
		} else if !hasDefaultSource {
			return fmt.Errorf("config: handler %q has no 'source' key and no default source is defined", name)
		}
		for fieldName, f := range h.Fields {
			if f.Handler != "" {
				if _, ok := cfg.Handlers[f.Handler]; !ok {
					return fmt.Errorf("config: handler %q field %q references undefined handler %q", name, fieldName, f.Handler)
				}
			}
		}
	}

	// Resolve relative paths relative to config file location
	dir := filepath.Dir(configPath)
	if cfg.Source.Path != "" && !filepath.IsAbs(cfg.Source.Path) {
		cfg.Source.Path = filepath.Join(dir, cfg.Source.Path)
	}
	for name, s := range cfg.Sources {
		if s.Path != "" && !filepath.IsAbs(s.Path) {
			s.Path = filepath.Join(dir, s.Path)
			cfg.Sources[name] = s
		}
	}
	if cfg.Output.File != "" && !filepath.IsAbs(cfg.Output.File) {
		cfg.Output.File = filepath.Join(dir, cfg.Output.File)
	}

	return nil
}

func validateSourceConfig(label string, s SourceConfig) error {
	if s.Path != "" && s.URL != "" {
		return fmt.Errorf("config: %s cannot have both 'path' and 'url'", label)
	}
	if s.URL == "" {
		if len(s.Headers) > 0 {
			return fmt.Errorf("config: %s: 'headers' requires 'url'", label)
		}
		if s.Method != "" {
			return fmt.Errorf("config: %s: 'method' requires 'url'", label)
		}
		if s.Body != "" {
			return fmt.Errorf("config: %s: 'body' requires 'url'", label)
		}
	}
	if s.Body != "" {
		m := strings.ToUpper(s.Method)
		if m != "POST" && m != "PUT" && m != "PATCH" {
			return fmt.Errorf("config: %s: 'body' requires method POST, PUT, or PATCH (got %q)", label, s.Method)
		}
	}
	return nil
}

func applyDefaults(cfg *Config, _ string) {
	if cfg.Output.Format == "" {
		cfg.Output.Format = "json"
	}
	if cfg.Output.Indent == 0 {
		cfg.Output.Indent = 2
	}

	normaliseSource(&cfg.Source)
	for name, s := range cfg.Sources {
		normaliseSource(&s)
		cfg.Sources[name] = s
	}
}

func normaliseSource(s *SourceConfig) {
	detectFormat(s)
	if s.Method != "" {
		s.Method = strings.ToUpper(s.Method)
	}
}

func detectFormat(s *SourceConfig) {
	if s.Format != "" {
		return
	}
	src := s.Path
	if src == "" {
		src = s.URL
	}
	// stdin and URLs without extension default to JSON
	switch strings.ToLower(filepath.Ext(src)) {
	case ".yaml", ".yml":
		s.Format = "yaml"
	default:
		s.Format = "json"
	}
}

// SourceFor returns the SourceConfig that applies to the named handler.
func (cfg *Config) SourceFor(handlerName string) SourceConfig {
	h := cfg.Handlers[handlerName]
	if h.Source != "" {
		return cfg.Sources[h.Source]
	}
	return cfg.Source
}
