package source

import (
	"strings"
	"testing"
)

func TestNdjsonSource_Stream(t *testing.T) {
	input := `{"name":"alice","age":30}` + "\n" + `{"name":"bob","age":25}` + "\n"
	s := &NdjsonSource{Reader: strings.NewReader(input)}

	var records []interface{}
	err := s.Stream(func(v interface{}) error {
		records = append(records, v)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	m := records[0].(map[string]interface{})
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestNdjsonSource_SkipsBlankLines(t *testing.T) {
	input := `{"x":1}` + "\n\n" + `{"x":2}` + "\n"
	s := &NdjsonSource{Reader: strings.NewReader(input)}

	var count int
	err := s.Stream(func(_ interface{}) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d records, want 2", count)
	}
}

func TestNdjsonSource_Load(t *testing.T) {
	input := `{"a":1}` + "\n" + `{"a":2}` + "\n"
	s := &NdjsonSource{Reader: strings.NewReader(input)}

	v, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	if len(arr) != 2 {
		t.Errorf("got %d elements, want 2", len(arr))
	}
}

func TestNdjsonSource_InvalidJSON(t *testing.T) {
	input := `{"a":1}` + "\n" + `not json` + "\n"
	s := &NdjsonSource{Reader: strings.NewReader(input)}

	err := s.Stream(func(_ interface{}) error { return nil })
	if err == nil {
		t.Fatal("expected error for invalid JSON line")
	}
}
