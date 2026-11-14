package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	if cfg.Version != "" && cfg.Version != "1" {
		return fmt.Errorf("config: unsupported version %q (want \"1\")", cfg.Version)
	}

	// Source presence is not validated here — cmd/run.go enforces it at runtime
	// because a piped stdin makes a source config optional.
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
	if cfg.Output.Indent != nil && *cfg.Output.Indent < 0 {
		return fmt.Errorf("config: output.indent must be >= 0")
	}
	switch strings.ToLower(cfg.Output.Format) {
	case "", "json", "yaml", "yml":
	default:
		return fmt.Errorf("config: output.format must be json or yaml (got %q)", cfg.Output.Format)
	}
	if len(cfg.Handlers) == 0 {
		return fmt.Errorf("config: handlers must be non-empty")
	}

	for name, h := range cfg.Handlers {
		if h.Source != "" {
			if _, ok := cfg.Sources[h.Source]; !ok {
				return fmt.Errorf("config: handler %q references undefined source %q", name, h.Source)
			}
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
	switch strings.ToLower(s.Format) {
	case "", "json", "yaml", "yml", "ndjson":
	default:
		return fmt.Errorf("config: %s: unknown format %q (want json, yaml or ndjson)", label, s.Format)
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
		if s.Timeout != "" {
			return fmt.Errorf("config: %s: 'timeout' requires 'url'", label)
		}
	}
	if s.Timeout != "" {
		d, err := time.ParseDuration(s.Timeout)
		if err != nil || d < 0 {
			return fmt.Errorf("config: %s: 'timeout' must be a non-negative duration such as 30s or 2m (got %q)", label, s.Timeout)
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
	cfg.Output.Format = strings.ToLower(cfg.Output.Format)
	if cfg.Output.Format == "" {
		cfg.Output.Format = "json"
	}
	if cfg.Output.Indent == nil {
		indent := 2
		cfg.Output.Indent = &indent
	}

	normaliseSource(&cfg.Source)
	for name, s := range cfg.Sources {
		normaliseSource(&s)
		cfg.Sources[name] = s
	}
}

func normaliseSource(s *SourceConfig) {
	s.Format = strings.ToLower(s.Format)
	detectFormat(s)
	if s.Method != "" {
		s.Method = strings.ToUpper(s.Method)
	}
}

func detectFormat(s *SourceConfig) {
	if s.Format != "" {
		return
	}
	ext := filepath.Ext(s.Path)
	if s.URL != "" {
		ext = path.Ext(s.URL)
		if u, err := url.Parse(s.URL); err == nil {
			ext = path.Ext(u.Path)
		}
	}
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		s.Format = "yaml"
	case ".ndjson", ".jsonl":
		s.Format = "ndjson"
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
