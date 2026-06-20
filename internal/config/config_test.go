package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFieldConfigUnmarshalYAML_Expr(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      name: ".name | ascii_upcase"
`)
	f := cfg.Handlers["root"].Fields["name"]
	if f.Expr != ".name | ascii_upcase" {
		t.Errorf("Expr = %q, want %q", f.Expr, ".name | ascii_upcase")
	}
	if f.Handler != "" {
		t.Errorf("Handler should be empty, got %q", f.Handler)
	}
}

func TestFieldConfigUnmarshalYAML_SubHandler(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      location:
        handler: loc
        input: ".loc"
  loc:
    fields:
      name: ".name"
`)
	f := cfg.Handlers["root"].Fields["location"]
	if f.Handler != "loc" {
		t.Errorf("Handler = %q, want %q", f.Handler, "loc")
	}
	if f.Input != ".loc" {
		t.Errorf("Input = %q, want %q", f.Input, ".loc")
	}
	if f.Expr != "" {
		t.Errorf("Expr should be empty, got %q", f.Expr)
	}
}

func TestLoadValidation_NoSource(t *testing.T) {
	_, err := loadString(t, `
version: "1"
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestLoadValidation_BothSourcePathAndURL(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
  url: http://example.com/data.json
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil {
		t.Fatal("expected error for both path and url")
	}
}

func TestLoadValidation_UndefinedSubHandler(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      loc:
        handler: missing
        input: ".loc"
`)
	if err == nil {
		t.Fatal("expected error for undefined sub-handler")
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      id: ".id"
`)
	if cfg.Output.Format != "json" {
		t.Errorf("default output format = %q, want json", cfg.Output.Format)
	}
	if cfg.Output.Indent != 2 {
		t.Errorf("default indent = %d, want 2", cfg.Output.Indent)
	}
	if cfg.Source.Format != "json" {
		t.Errorf("auto-detected format = %q, want json", cfg.Source.Format)
	}
}

func TestLoadDefaults_YAMLExtension(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/data.yaml
handlers:
  root:
    fields:
      id: ".id"
`)
	if cfg.Source.Format != "yaml" {
		t.Errorf("auto-detected format = %q, want yaml", cfg.Source.Format)
	}
}

func TestURLOptions_HeadersRequireURL(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
  headers:
    Authorization: "Bearer tok"
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil {
		t.Fatal("expected error: headers on path source")
	}
}

func TestURLOptions_MethodRequiresURL(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
  method: POST
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil {
		t.Fatal("expected error: method on path source")
	}
}

func TestURLOptions_BodyRequiresPostMethod(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  url: http://example.com/data.json
  body: '{"q":"test"}'
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil {
		t.Fatal("expected error: body without POST/PUT/PATCH method")
	}
}

func TestURLOptions_BodyWithPOST(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  url: http://example.com/data.json
  method: post
  body: '{"q":"test"}'
handlers:
  root:
    fields:
      id: ".id"
`)
	if cfg.Source.Method != "POST" {
		t.Errorf("method = %q, want POST (uppercased)", cfg.Source.Method)
	}
	if cfg.Source.Body != `{"q":"test"}` {
		t.Errorf("body = %q, want {\"q\":\"test\"}", cfg.Source.Body)
	}
}

func TestURLOptions_HeadersParsed(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  url: http://example.com/data.json
  headers:
    Authorization: "Bearer tok"
    Accept: "application/json"
handlers:
  root:
    fields:
      id: ".id"
`)
	if cfg.Source.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("Authorization header = %q", cfg.Source.Headers["Authorization"])
	}
	if cfg.Source.Headers["Accept"] != "application/json" {
		t.Errorf("Accept header = %q", cfg.Source.Headers["Accept"])
	}
}

// mustLoad parses YAML config string; fails test on error.
func mustLoad(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := loadString(t, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
}

// loadString writes content to a temp file and calls Load.
func loadString(t *testing.T, content string) (*Config, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "alpakr-*.yaml")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()
	// Make source path absolute so validate doesn't fail on relative resolution
	return Load(filepath.Clean(f.Name()))
}
