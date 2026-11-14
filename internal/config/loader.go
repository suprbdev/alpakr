package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing config %s: %w", path, friendlyYAMLError(err))
	}

	if err := expandConfigEnv(&cfg); err != nil {
		return nil, err
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
	case "", "json", "yaml", "yml", "ndjson", "jsonl", "csv":
	default:
		return fmt.Errorf("config: output.format must be json, yaml, ndjson or csv (got %q)", cfg.Output.Format)
	}
	if err := checkDelimiter("output", cfg.Output.Delimiter); err != nil {
		return err
	}
	for name := range cfg.Vars {
		if err := CheckVarName(name); err != nil {
			return fmt.Errorf("config: vars: %w", err)
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
		}
		for _, f := range h.Fields {
			if f.Handler != "" {
				if _, ok := cfg.Handlers[f.Handler]; !ok {
					return fmt.Errorf("config: handler %q field %q references undefined handler %q", name, f.Name, f.Handler)
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
	case "", "json", "yaml", "yml", "ndjson", "csv":
	default:
		return fmt.Errorf("config: %s: unknown format %q (want json, yaml, ndjson or csv)", label, s.Format)
	}
	if s.Delimiter != "" || s.Header != nil {
		if f, _ := effectiveFormat(s); f != "csv" {
			return fmt.Errorf("config: %s: 'delimiter' and 'header' require format csv", label)
		}
	}
	if err := checkDelimiter(label, s.Delimiter); err != nil {
		return err
	}
	if p := s.Pagination; p != nil {
		if s.URL == "" {
			return fmt.Errorf("config: %s: 'pagination' requires 'url'", label)
		}
		if p.Next == "" {
			return fmt.Errorf("config: %s.pagination: 'next' is required", label)
		}
		if p.MaxPages != nil && *p.MaxPages < 0 {
			return fmt.Errorf("config: %s.pagination: 'max_pages' must be >= 0", label)
		}
		if f, _ := effectiveFormat(s); f != "json" && f != "yaml" && f != "yml" {
			return fmt.Errorf("config: %s: pagination needs json or yaml pages (format is %s)", label, f)
		}
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
	format, detected := effectiveFormat(*s)
	s.Format, s.FormatDetected = format, detected
	if detected && s.Delimiter == "" && strings.EqualFold(extension(*s), ".tsv") {
		s.Delimiter = "\t"
	}
}

// effectiveFormat returns the source's format: the configured one, or the
// one implied by its path or URL extension (json if unrecognised).
func effectiveFormat(s SourceConfig) (format string, detected bool) {
	if s.Format != "" {
		return strings.ToLower(s.Format), false
	}
	switch strings.ToLower(extension(s)) {
	case ".yaml", ".yml":
		return "yaml", true
	case ".ndjson", ".jsonl":
		return "ndjson", true
	case ".csv", ".tsv":
		return "csv", true
	}
	return "json", true
}

func extension(s SourceConfig) string {
	ext := filepath.Ext(s.Path)
	if s.URL != "" {
		ext = path.Ext(s.URL)
		if u, err := url.Parse(s.URL); err == nil {
			ext = path.Ext(u.Path)
		}
	}
	return ext
}

// SourceFor returns the SourceConfig that applies to the named handler.
func (cfg *Config) SourceFor(handlerName string) SourceConfig {
	h := cfg.Handlers[handlerName]
	if h.Source != "" {
		return cfg.Sources[h.Source]
	}
	return cfg.Source
}
