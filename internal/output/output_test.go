package output

import (
	"bytes"
	"fmt"
	"math/big"
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

func TestWriters_BigInt(t *testing.T) {
	b, _ := new(big.Int).SetString("12345678901234567890", 10)
	v := map[string]interface{}{"id": b, "a": []interface{}{b}}

	var js bytes.Buffer
	if err := (&JSONWriter{Out: &js}).Write(v); err != nil {
		t.Fatal(err)
	}
	if want := `{"a":[12345678901234567890],"id":12345678901234567890}` + "\n"; js.String() != want {
		t.Errorf("json = %q, want %q", js.String(), want)
	}

	var ys bytes.Buffer
	if err := (&YAMLWriter{Out: &ys}).Write(v); err != nil {
		t.Fatal(err)
	}
	if want := "a:\n    - 12345678901234567890\nid: 12345678901234567890\n"; ys.String() != want {
		t.Errorf("yaml = %q, want %q", ys.String(), want)
	}
}
