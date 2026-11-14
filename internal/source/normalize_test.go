package source

import (
	"math/big"
	"reflect"
	"testing"
)

func TestParseJSON_Numbers(t *testing.T) {
	v, err := parse([]byte(`{"i":42,"neg":-7,"f":1.5,"e":1e3,"big":12345678901234567890,"p":9007199254740993}`), "json")
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	for k, want := range map[string]interface{}{"i": 42, "neg": -7, "f": 1.5, "e": 1000.0, "p": 9007199254740993} {
		if m[k] != want {
			t.Errorf("%s = %#v, want %#v", k, m[k], want)
		}
	}
	b, ok := m["big"].(*big.Int)
	if !ok || b.String() != "12345678901234567890" {
		t.Errorf("big = %#v, want *big.Int 12345678901234567890", m["big"])
	}
}

func TestParseJSON_NestedNumbers(t *testing.T) {
	v, err := parse([]byte(`[{"a":[1,{"b":2}]}]`), "json")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{map[string]interface{}{"a": []interface{}{1, map[string]interface{}{"b": 2}}}}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("got %#v, want %#v", v, want)
	}
}

func TestParseJSON_TrailingData(t *testing.T) {
	for _, in := range []string{`{"a":1} {"a":2}`, `{"a":1}}`, `[1] x`} {
		if _, err := parse([]byte(in), "json"); err == nil {
			t.Errorf("parse(%q): expected error for trailing data", in)
		}
	}
	if _, err := parse([]byte("{\"a\":1}\n\t "), "json"); err != nil {
		t.Errorf("trailing whitespace should be accepted: %v", err)
	}
}

func TestParseYAML_Timestamps(t *testing.T) {
	v, err := parse([]byte("d: 2024-01-01\nt: 2024-01-01T10:30:00Z\nx: !!timestamp 2001-12-14\nq: \"2024-02-02\"\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"d": "2024-01-01", "t": "2024-01-01T10:30:00Z", "x": "2001-12-14", "q": "2024-02-02"}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("got %#v, want %#v", v, want)
	}
}

func TestParseYAML_NonStringKeys(t *testing.T) {
	v, err := parse([]byte("m:\n  1: a\n  true: b\n  ~: c\n  1.5: d\n  s: e\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"m": map[string]interface{}{"1": "a", "true": "b", "null": "c", "1.5": "d", "s": "e"}}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("got %#v, want %#v", v, want)
	}
}

func TestParseYAML_LargeIntegers(t *testing.T) {
	v, err := parse([]byte("u: 18446744073709551615\ni: 42\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	if b, ok := m["u"].(*big.Int); !ok || b.String() != "18446744073709551615" {
		t.Errorf("u = %#v, want *big.Int 18446744073709551615", m["u"])
	}
	if m["i"] != 42 {
		t.Errorf("i = %#v, want 42", m["i"])
	}
}

func TestParseYAML_AnchorsAndMerge(t *testing.T) {
	v, err := parse([]byte("base: &b {when: 2024-01-01, n: 1}\nchild:\n  <<: *b\n  n: 2\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	child := v.(map[string]interface{})["child"]
	want := map[string]interface{}{"when": "2024-01-01", "n": 2}
	if !reflect.DeepEqual(child, want) {
		t.Errorf("child = %#v, want %#v", child, want)
	}
}

func TestParseYAML_Empty(t *testing.T) {
	v, err := parse([]byte(""), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("got %#v, want nil", v)
	}
}
