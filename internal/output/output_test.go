package output

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestJSONWriter_NonFiniteFloats(t *testing.T) {
	v := map[string]interface{}{"inf": math.Inf(1), "ninf": []interface{}{math.Inf(-1)}, "nan": math.NaN(), "ok": 1.5}
	var buf bytes.Buffer
	w := &JSONWriter{Out: &buf}
	if err := w.Write(v); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteOne(v); err != nil {
		t.Fatal(err)
	}
	line := `{"inf":1.7976931348623157e+308,"nan":null,"ninf":[-1.7976931348623157e+308],"ok":1.5}` + "\n"
	if buf.String() != line+line {
		t.Errorf("got %q, want %q twice", buf.String(), line)
	}
}

func TestYAMLWriter_HugeBigIntRoundTrips(t *testing.T) {
	b, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	var buf bytes.Buffer
	if err := (&YAMLWriter{Out: &buf}).Write(map[string]interface{}{"id": b}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "id: 123456789012345678901234567890\n" {
		t.Errorf("got %q", buf.String())
	}
	var back map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Errorf("output does not parse back: %v", err)
	}
}

func TestScalarNode_MatchesYAMLEncode(t *testing.T) {
	values := []interface{}{
		nil, true, false, 0, -42, 1 << 40, 1.5, 3.0, 1e21, 1e-7, math.Inf(1), math.Inf(-1), math.NaN(),
		"", "plain", "n", "Yes", "off", "1:20", "123", "1.5", "true", "null", "~", "-", "- x", "a: b",
		"#c", "x #y", "multi\nline", "trailing\n", " lead", "trail ", "'q'", `"dq"`, "{a}", "[1]",
		"*ref", "&anc", "!tag", "%dir", "@at", "`bt", "0x1F", "0o17", ".inf", "2024-01-01", "é", "\t",
	}
	for _, v := range values {
		want := &yaml.Node{}
		if err := want.Encode(map[string]interface{}{"k": v}); err != nil {
			t.Fatal(err)
		}
		wantOut, err := yaml.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := (&YAMLWriter{Out: &buf}).Write(map[string]interface{}{"k": v}); err != nil {
			t.Fatal(err)
		}
		if buf.String() != string(wantOut) {
			t.Errorf("%#v: got %q, want %q", v, buf.String(), wantOut)
		}
	}
}
