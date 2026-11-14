package source

import (
	"bufio"
	"strings"
	"testing"
)

func TestStdinSource_JSON(t *testing.T) {
	s := &StdinSource{
		Format: "json",
		reader: strings.NewReader(`{"name":"alice"}`),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestStdinSource_YAML(t *testing.T) {
	s := &StdinSource{
		Format: "yaml",
		reader: strings.NewReader("name: bob\n"),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "bob" {
		t.Errorf("name = %v, want bob", m["name"])
	}
}

func TestStdinSource_DefaultJSON(t *testing.T) {
	s := &StdinSource{
		reader: strings.NewReader(`[1,2,3]`),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3", len(arr))
	}
}

func TestStdinSource_InvalidJSON(t *testing.T) {
	s := &StdinSource{
		Format: "json",
		reader: strings.NewReader("not json"),
	}
	_, err := s.Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStdinSource_AutoDetect_JSON(t *testing.T) {
	s := &StdinSource{
		reader: strings.NewReader(`{"name":"alice"}`),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestStdinSource_AutoDetect_JSONArray(t *testing.T) {
	s := &StdinSource{
		reader: strings.NewReader(`[1,2,3]`),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3", len(arr))
	}
}

func TestStdinSource_AutoDetect_YAML(t *testing.T) {
	s := &StdinSource{
		reader: strings.NewReader("name: bob\nage: 25\n"),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "bob" {
		t.Errorf("name = %v, want bob", m["name"])
	}
}

func TestStdinSource_AutoDetect_LeadingWhitespace(t *testing.T) {
	s := &StdinSource{
		reader: strings.NewReader("  \n  {\"x\":42}"),
	}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["x"] != 42 {
		t.Errorf("x = %v, want 42", m["x"])
	}
}

func TestStdinSource_AutoDetect_BOM(t *testing.T) {
	s := &StdinSource{reader: strings.NewReader("\xEF\xBB\xBF{\"url\":\"a\\/b\"}")}
	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m, ok := v.(map[string]interface{}); !ok || m["url"] != "a/b" {
		t.Errorf("got %#v, want url a/b (parsed as JSON)", v)
	}
}

func TestDetectStdinFormat_SkipsBOM(t *testing.T) {
	got, err := detectStdinFormat(bufio.NewReader(strings.NewReader("\xEF\xBB\xBF [1]")))
	if err != nil || got != "json" {
		t.Errorf("detectStdinFormat = %q, %v; want json", got, err)
	}
}
