package output

import (
	"bytes"
	"fmt"
	"testing"
)

func TestJSONWriter_NoHTMLEscape(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{Out: &buf}
	if err := w.Write(map[string]interface{}{"u": "a?b=1&c=<d>"}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), `{"u":"a?b=1&c=<d>"}`+"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONWriter_Indent(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{Indent: 2, Out: &buf}
	if err := w.Write(map[string]interface{}{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "{\n  \"a\": 1\n}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONWriter_IndentZeroIsCompact(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{Indent: 0, Out: &buf}
	if err := w.Write([]interface{}{map[string]interface{}{"a": 1}, 2}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), `[{"a":1},2]`+"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONWriter_WriteOne(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{Indent: 4, Out: &buf}
	for _, v := range []interface{}{map[string]interface{}{"a": "<"}, map[string]interface{}{"a": 2}} {
		if err := w.WriteOne(v); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := buf.String(), "{\"a\":\"<\"}\n{\"a\":2}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestYAMLWriter_WriteOne(t *testing.T) {
	var buf bytes.Buffer
	w := &YAMLWriter{Out: &buf}
	for _, v := range []interface{}{map[string]interface{}{"a": 1}, map[string]interface{}{"a": 2}} {
		if err := w.WriteOne(v); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := buf.String(), "---\na: 1\n---\na: 2\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNew(t *testing.T) {
	for format, want := range map[string]string{"json": "*output.JSONWriter", "yaml": "*output.YAMLWriter", "yml": "*output.YAMLWriter"} {
		w, err := New(format, 2, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("New(%q): %v", format, err)
		}
		if got := fmt.Sprintf("%T", w); got != want {
			t.Errorf("New(%q) = %s, want %s", format, got, want)
		}
	}
}

func TestNew_UnknownFormat(t *testing.T) {
	if _, err := New("toml", 2, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
