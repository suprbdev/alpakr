package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	f := mustField(t, cfg.Handlers["root"].Fields, "name")
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
	f := mustField(t, cfg.Handlers["root"].Fields, "location")
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
	if cfg.Output.Indent == nil || *cfg.Output.Indent != 2 {
		t.Errorf("default indent = %v, want 2", cfg.Output.Indent)
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
	if cfg.Source.Method != http.MethodPost {
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
	if err := f.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
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
	f := mustField(t, cfg.Handlers["root"].Fields, "location")
	if f.Fields == nil {
		t.Fatal("Fields should be set for inline object")
	}
	if f.Input != ".loc" {
		t.Errorf("Input = %q, want .loc", f.Input)
	}
	if mustField(t, f.Fields, "city").Expr != ".city" {
		t.Errorf("city expr = %q, want .city", mustField(t, f.Fields, "city").Expr)
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
	f := mustField(t, cfg.Handlers["root"].Fields, "meta")
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

func TestLoadOutput_IndentZeroKept(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
output:
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`)
	if cfg.Output.Indent == nil || *cfg.Output.Indent != 0 {
		t.Errorf("indent = %v, want explicit 0", cfg.Output.Indent)
	}
}

func TestLoadOutput_Invalid(t *testing.T) {
	for name, output := range map[string]string{
		"negative indent": "indent: -1",
		"unknown format":  "format: xml",
	} {
		_, err := loadString(t, `
version: "1"
output:
  `+output+`
handlers:
  root:
    fields:
      id: ".id"
`)
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestLoadValidation_MultiSourceSubHandlerWithoutSource(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
sources:
  a:
    path: /tmp/a.json
handlers:
  root:
    source: a
    fields:
      loc:
        handler: loc
        input: ".loc"
  loc:
    fields:
      city: ".city"
`)
	if got := cfg.SourceFor("root").Path; got != "/tmp/a.json" {
		t.Errorf("root source path = %q, want /tmp/a.json", got)
	}
}

func TestLoadValidation_Version(t *testing.T) {
	for _, v := range []string{"", `version: "1"`} {
		if _, err := loadString(t, v+"\nhandlers:\n  root:\n    fields:\n      id: \".id\"\n"); err != nil {
			t.Errorf("%q: unexpected error: %v", v, err)
		}
	}
	if _, err := loadString(t, "version: \"2\"\nhandlers:\n  root:\n    fields:\n      id: \".id\"\n"); err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestLoadValidation_UnknownSourceFormat(t *testing.T) {
	for name, src := range map[string]string{
		"source":  "source:\n  path: /tmp/x.csv\n  format: csv\n",
		"sources": "sources:\n  a:\n    path: /tmp/x.csv\n    format: csv\n",
	} {
		_, err := loadString(t, "version: \"1\"\n"+src+"handlers:\n  root:\n    fields:\n      id: \".id\"\n")
		if err == nil {
			t.Errorf("%s: expected error for unknown format", name)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	for _, tc := range []struct {
		src  SourceConfig
		want string
	}{
		{SourceConfig{Path: "/data/x.JSON"}, "json"},
		{SourceConfig{Path: "/data/x.yml"}, "yaml"},
		{SourceConfig{Path: "/data/x.jsonl"}, "ndjson"},
		{SourceConfig{Path: "/data/x"}, "json"},
		{SourceConfig{URL: "https://example.com/data.yaml?token=abc"}, "yaml"},
		{SourceConfig{URL: "https://example.com/events.ndjson#frag"}, "ndjson"},
		{SourceConfig{URL: "https://example.com/api/records?format=.yaml"}, "json"},
		{SourceConfig{Path: "/data/x.yaml", Format: "json"}, "json"},
	} {
		s := tc.src
		detectFormat(&s)
		if s.Format != tc.want {
			t.Errorf("detectFormat(%+v) = %q, want %q", tc.src, s.Format, tc.want)
		}
	}
}

func TestURLOptions_Timeout(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
sources:
  a:
    url: http://example.com/a.json
    timeout: 2m
  b:
    url: http://example.com/b.json
  c:
    url: http://example.com/c.json
    timeout: "0"
handlers:
  root:
    source: a
    fields:
      id: ".id"
`)
	for name, want := range map[string]time.Duration{"a": 2 * time.Minute, "b": DefaultTimeout, "c": 0} {
		if got := cfg.Sources[name].TimeoutDuration(); got != want {
			t.Errorf("sources.%s timeout = %s, want %s", name, got, want)
		}
	}
}

func TestURLOptions_TimeoutInvalid(t *testing.T) {
	for name, src := range map[string]string{
		"not a duration": "  url: http://example.com/x.json\n  timeout: soon\n",
		"negative":       "  url: http://example.com/x.json\n  timeout: -1s\n",
		"bare number":    "  url: http://example.com/x.json\n  timeout: 30\n",
		"path source":    "  path: /tmp/x.json\n  timeout: 30s\n",
	} {
		if _, err := loadString(t, "version: \"1\"\nsource:\n"+src+"handlers:\n  root:\n    fields:\n      id: \".id\"\n"); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestLoadStrict_UnknownKeys(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"handler": {"handlers:\n  root:\n    filtr: x\n    fields:\n      a: \".a\"\n",
			`line 4: unknown key "filtr" in handler (valid keys: source, input, each, filter, fields)`},
		"top level": {"sorce:\n  path: x.json\nhandlers:\n  root:\n    fields:\n      a: \".a\"\n",
			`line 2: unknown key "sorce" in top level`},
		"source": {"source:\n  path: x.json\n  hedaers: {a: b}\nhandlers:\n  root:\n    fields:\n      a: \".a\"\n",
			`line 4: unknown key "hedaers" in source`},
		"output": {"output:\n  indnet: 2\nhandlers:\n  root:\n    fields:\n      a: \".a\"\n",
			`line 3: unknown key "indnet" in output (valid keys: format, indent, file)`},
		"field": {"handlers:\n  root:\n    fields:\n      loc:\n        handler: root\n        inptu: \".x\"\n",
			`line 7: unknown key "inptu" in field (valid keys: handler, input, fields)`},
		"nested inline field": {"handlers:\n  root:\n    fields:\n      loc:\n        fields:\n          city:\n            fields: {a: \".a\"}\n            extra: 1\n",
			`line 9: unknown key "extra" in field`},
	} {
		_, err := loadString(t, "version: \"1\"\n"+tc.yaml)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", name, err, tc.want)
		}
	}
}

func TestLoadStrict_FieldShapes(t *testing.T) {
	for name, field := range map[string]string{
		"sequence":        "[1]",
		"handler not str": "{handler: [x]}",
		"input not str":   "{handler: root, input: {a: 1}}",
		"empty fields":    "{fields: {}}",
	} {
		_, err := loadString(t, "version: \"1\"\nhandlers:\n  root:\n    fields:\n      a: "+field+"\n")
		if err == nil || !strings.Contains(err.Error(), "line 5:") {
			t.Errorf("%s: err = %v, want line-numbered error", name, err)
		}
	}
}

func TestLoadStrict_ScalarFieldForms(t *testing.T) {
	cfg := mustLoad(t, "version: \"1\"\nhandlers:\n  root:\n    fields:\n      n: 5\n      b: true\n      q: '\"lit\"'\n")
	for name, want := range map[string]string{"n": "5", "b": "true", "q": `"lit"`} {
		if got := mustField(t, cfg.Handlers["root"].Fields, name).Expr; got != want {
			t.Errorf("%s: Expr = %q, want %q", name, got, want)
		}
	}
}

func mustField(t *testing.T, fs Fields, name string) FieldConfig {
	t.Helper()
	f, ok := fs.Get(name)
	if !ok {
		t.Fatalf("field %q not found", name)
	}
	return f
}

func TestFields_KeepConfigOrder(t *testing.T) {
	cfg := mustLoad(t, "version: \"1\"\nhandlers:\n  root:\n    fields:\n      zeta: \".z\"\n      alpha: \".a\"\n      mid:\n        fields:\n          y: \".y\"\n          b: \".b\"\n")
	var names []string
	for _, f := range cfg.Handlers["root"].Fields {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "zeta,alpha,mid" {
		t.Errorf("order = %v", names)
	}
	mid := mustField(t, cfg.Handlers["root"].Fields, "mid")
	if mid.Fields[0].Name != "y" || mid.Fields[1].Name != "b" {
		t.Errorf("nested order = %+v", mid.Fields)
	}
}

func TestFields_Duplicate(t *testing.T) {
	_, err := loadString(t, "version: \"1\"\nhandlers:\n  root:\n    fields:\n      a: \".a\"\n      a: \".b\"\n")
	if err == nil || !strings.Contains(err.Error(), `duplicate field "a"`) {
		t.Errorf("err = %v", err)
	}
}

func TestFields_NullValue(t *testing.T) {
	_, err := loadString(t, "version: \"1\"\nhandlers:\n  root:\n    fields:\n      a:\n")
	if err == nil || !strings.Contains(err.Error(), `line 5: field "a" has no value`) {
		t.Errorf("err = %v", err)
	}
}

func TestFieldConfig_Aliases(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
handlers:
  inner:
    fields:
      x: &sel ".x"
  root:
    fields:
      a:
        handler: &h inner
        input: &in ".sub"
      b:
        handler: *h
        input: *in
      c: *sel
      d:
        input: *in
        fields: &fs
          y: ".y"
      e:
        fields: *fs
`)
	root := cfg.Handlers["root"].Fields
	if b := mustField(t, root, "b"); b.Handler != "inner" || b.Input != ".sub" {
		t.Errorf("b = %+v", b)
	}
	if c := mustField(t, root, "c"); c.Expr != ".x" {
		t.Errorf("c = %+v", c)
	}
	if e := mustField(t, root, "e"); len(e.Fields) != 1 || e.Fields[0].Name != "y" {
		t.Errorf("e = %+v", e)
	}
}
