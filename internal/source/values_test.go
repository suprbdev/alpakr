package source

import (
	"math"
	"strings"
	"testing"
)

func TestValues_JSON(t *testing.T) {
	v, err := readOne(strings.NewReader(`{"name":"alice"}`), "json")
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

func TestValues_YAML(t *testing.T) {
	v, err := readOne(strings.NewReader("name: bob\n"), "yaml")
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

func TestValues_DefaultJSON(t *testing.T) {
	v, err := readOne(strings.NewReader(`[1,2,3]`), "")
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

func TestValues_InvalidJSON(t *testing.T) {
	_, err := readOne(strings.NewReader("not json"), "json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValues_AutoDetect_JSON(t *testing.T) {
	v, err := readOne(strings.NewReader(`{"name":"alice"}`), "")
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

func TestValues_AutoDetect_JSONArray(t *testing.T) {
	v, err := readOne(strings.NewReader(`[1,2,3]`), "")
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

func TestValues_AutoDetect_YAML(t *testing.T) {
	v, err := readOne(strings.NewReader("name: bob\nage: 25\n"), "")
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

func TestValues_AutoDetect_LeadingWhitespace(t *testing.T) {
	v, err := readOne(strings.NewReader("  \n  {\"x\":42}"), "")
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

func TestValues_AutoDetect_BOM(t *testing.T) {
	v, err := readOne(strings.NewReader("\xEF\xBB\xBF{\"url\":\"a\\/b\"}"), "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m, ok := v.(map[string]interface{}); !ok || m["url"] != "a/b" {
		t.Errorf("got %#v, want url a/b (parsed as JSON)", v)
	}
}

func TestDetectFormat_SkipsBOM(t *testing.T) {
	vals, err := NewValues(strings.NewReader("\xEF\xBB\xBF [1]"), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := vals.(*jsonValues); !ok {
		t.Errorf("detected %T, want *jsonValues", vals)
	}
}

func TestValues_JSONMultipleValues(t *testing.T) {
	for name, in := range map[string]string{
		"ndjson":       "{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n",
		"same line":    `{"a":1} {"a":2} {"a":3}`,
		"pretty":       "{\n  \"a\": 1\n}\n{\n  \"a\": 2\n}\n[3]",
		"auto-detect":  "\n{\"a\":1}\n{\"a\":2}\n{\"a\":3}",
		"mixed values": `1 "two" {"a":3}`,
	} {
		format := "json"
		if name == "auto-detect" {
			format = ""
		}
		got, err := readAll(strings.NewReader(in), format)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(got) != 3 {
			t.Errorf("%s: got %d values, want 3: %#v", name, len(got), got)
		}
	}
}

func TestValues_JSONErrors(t *testing.T) {
	for _, in := range []string{`{"a":1}}`, `[1] x`, `{"a":`, ""} {
		if _, err := readAll(strings.NewReader(in), "json"); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
	if _, err := readAll(strings.NewReader(""), "json"); err == nil || !strings.Contains(err.Error(), "empty input") {
		t.Errorf("empty input: err = %v", err)
	}
}

func TestValues_YAMLDocuments(t *testing.T) {
	got, err := readAll(strings.NewReader("a: 1\n---\na: 2\n---\n- 3\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d documents, want 3: %#v", len(got), got)
	}
	if got[1].(map[string]interface{})["a"] != 2 {
		t.Errorf("second document = %#v", got[1])
	}
	empty, err := readAll(strings.NewReader(""), "yaml")
	if err != nil || len(empty) != 1 || empty[0] != nil {
		t.Errorf("empty YAML = %#v, %v; want one null document", empty, err)
	}
}

func TestValues_UnknownFormat(t *testing.T) {
	if _, err := NewValues(strings.NewReader("x"), "csv"); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestValues_DetectAfterLongWhitespace(t *testing.T) {
	pad := strings.Repeat(" \n", 5000)
	got, err := readOne(strings.NewReader(pad+`{"a":1,"a":2,"b":1e400}`), "")
	if err != nil {
		t.Fatalf("JSON after long whitespace: %v", err)
	}
	if m := got.(map[string]interface{}); m["a"] != 2 || m["b"] != math.MaxFloat64 {
		t.Errorf("parsed as %#v, want JSON semantics", m)
	}
	y, err := readOne(strings.NewReader("\n\n  a: 1\n  b: 2\n"), "")
	if err != nil {
		t.Fatalf("indented YAML: %v", err)
	}
	if m := y.(map[string]interface{}); m["a"] != 1 || m["b"] != 2 {
		t.Errorf("indented YAML = %#v", y)
	}
}
