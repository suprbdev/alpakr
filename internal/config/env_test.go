package config

import (
	"strings"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	t.Setenv("ALPAKR_TOKEN", "s3cret")
	t.Setenv("ALPAKR_EMPTY", "")
	for in, want := range map[string]string{
		"Bearer ${ALPAKR_TOKEN}":          "Bearer s3cret",
		"${ALPAKR_UNSET:-fallback}":       "fallback",
		"${ALPAKR_EMPTY:-fallback}":       "fallback",
		"${ALPAKR_EMPTY}":                 "",
		"${ALPAKR_UNSET:-}":               "",
		"$${ALPAKR_TOKEN}":                "${ALPAKR_TOKEN}",
		"cost $$5":                        "cost $5",
		`{"query": "query($id: ID)"}`:     `{"query": "query($id: ID)"}`,
		"$ALPAKR_TOKEN":                   "$ALPAKR_TOKEN",
		"${ALPAKR_TOKEN}/${ALPAKR_TOKEN}": "s3cret/s3cret",
		"${not closed":                    "${not closed",
	} {
		got, err := expandEnv("x", in)
		if err != nil {
			t.Errorf("expandEnv(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("expandEnv(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandEnv_Missing(t *testing.T) {
	_, err := expandEnv("sources.api.headers.Authorization", "Bearer ${ALPAKR_DEFINITELY_UNSET}")
	if err == nil || !strings.Contains(err.Error(), `sources.api.headers.Authorization: environment variable "ALPAKR_DEFINITELY_UNSET" is not set`) {
		t.Errorf("err = %v", err)
	}
}

func TestLoad_ExpandsEnv(t *testing.T) {
	t.Setenv("ALPAKR_HOST", "api.example.com")
	t.Setenv("ALPAKR_TOKEN", "tok")
	cfg := mustLoad(t, `
version: "1"
sources:
  api:
    url: https://${ALPAKR_HOST}/items.yaml
    timeout: ${ALPAKR_TIMEOUT:-5s}
    headers:
      Authorization: Bearer ${ALPAKR_TOKEN}
output:
  file: /tmp/${ALPAKR_OUT:-out}.json
handlers:
  root:
    source: api
    fields:
      id: ".id"
`)
	api := cfg.Sources["api"]
	if api.URL != "https://api.example.com/items.yaml" || api.Format != "yaml" {
		t.Errorf("url = %q (format %q)", api.URL, api.Format)
	}
	if api.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("Authorization = %q", api.Headers["Authorization"])
	}
	if api.Timeout != "5s" {
		t.Errorf("timeout = %q", api.Timeout)
	}
	if cfg.Output.File != "/tmp/out.json" {
		t.Errorf("output.file = %q", cfg.Output.File)
	}
}

func TestLoad_MissingEnvIsError(t *testing.T) {
	_, err := loadString(t, `
version: "1"
source:
  url: https://example.com/${ALPAKR_DEFINITELY_UNSET}
handlers:
  root:
    fields:
      id: ".id"
`)
	if err == nil || !strings.Contains(err.Error(), "source.url") {
		t.Errorf("err = %v", err)
	}
}
