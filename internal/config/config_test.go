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
	// No source is valid at config-load time — cmd/run.go enforces the requirement
	// at runtime because piped stdin makes a source config optional.
	_, err := loadString(t, `
version: "1"
handlers:
  root:
    fields:
      id: ".id"
`)
	if err != nil {
		t.Fatalf("unexpected error for no-source config: %v", err)
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

func TestFieldConfigUnmarshalYAML_InlineFields(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      location:
        input: ".loc"
        fields:
          city: ".city"
          country: ".country"
`)
	f := cfg.Handlers["root"].Fields["location"]
	if f.Fields == nil {
		t.Fatal("Fields should be set for inline object")
	}
	if f.Input != ".loc" {
		t.Errorf("Input = %q, want .loc", f.Input)
	}
	if f.Fields["city"].Expr != ".city" {
		t.Errorf("city expr = %q, want .city", f.Fields["city"].Expr)
	}
	if f.Handler != "" || f.Expr != "" {
		t.Errorf("Handler and Expr should be empty for inline object")
	}
}

func TestFieldConfigUnmarshalYAML_InlineFields_NoInput(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      meta:
        fields:
          id: ".id"
          slug: ".name | slugify"
`)
	f := cfg.Handlers["root"].Fields["meta"]
	if f.Fields == nil {
		t.Fatal("Fields should be set")
	}
	if f.Input != "" {
		t.Errorf("Input should be empty, got %q", f.Input)
	}
	if len(f.Fields) != 2 {
		t.Errorf("nested field count = %d, want 2", len(f.Fields))
	}
}

func TestFieldConfigUnmarshalYAML_InlineFields_HandlerAndFieldsError(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      bad:
        handler: other
        fields:
          x: ".x"
  other:
    fields:
      x: ".x"
`)
	if err == nil {
		t.Fatal("expected error for field with both handler and fields keys")
	}
}

func TestFieldConfigUnmarshalYAML_BareMapError(t *testing.T) {
	// A map with neither 'handler' nor 'fields' key should error — previously
	// this was silently treated as inline fields, creating ambiguity with data
	// that has keys named 'handler' or 'input'.
	_, err := loadString(t, `
version: "1"
source:
  path: /tmp/x.json
handlers:
  root:
    fields:
      location:
        city: ".city"
        country: ".country"
`)
	if err == nil {
		t.Fatal("expected error for bare map without handler or fields key")
	}
}
