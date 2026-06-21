package source

import (
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
