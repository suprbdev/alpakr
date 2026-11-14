package source

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
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

func TestParseYAML_IntegerForms(t *testing.T) {
	v, err := parse([]byte("huge: 123456789012345678901234\nneg: -123456789012345678901234\nhex: 0x10\nund: 1_000\noct: 0o17\nneg5: -5\nexplicit: !!int \"7\"\nstr: !!str 123\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	for k, want := range map[string]string{"huge": "123456789012345678901234", "neg": "-123456789012345678901234"} {
		if b, ok := m[k].(*big.Int); !ok || b.String() != want {
			t.Errorf("%s = %#v, want *big.Int %s", k, m[k], want)
		}
	}
	for k, want := range map[string]interface{}{"hex": 16, "und": 1000, "oct": 15, "neg5": -5, "explicit": 7, "str": "123"} {
		if m[k] != want {
			t.Errorf("%s = %#v, want %#v", k, m[k], want)
		}
	}
}

func TestParseYAML_Scalars(t *testing.T) {
	v, err := parse([]byte("t: true\nf: False\ns: yes\nn: ~\nfl: 1.5\ne: 1e3\ninf: -.inf\nbin: !!binary aGk=\nq: \"1\"\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]interface{})
	for k, want := range map[string]interface{}{"t": true, "f": false, "s": "yes", "n": nil, "fl": 1.5, "e": 1000.0, "inf": math.Inf(-1), "bin": "hi", "q": "1"} {
		if m[k] != want {
			t.Errorf("%s = %#v, want %#v", k, m[k], want)
		}
	}
	if nan, err := parse([]byte("x: .nan\n"), "yaml"); err != nil || !math.IsNaN(nan.(map[string]interface{})["x"].(float64)) {
		t.Errorf(".nan = %#v, %v", nan, err)
	}
}

func TestParseYAML_KeyCollisions(t *testing.T) {
	for _, in := range []string{"1: a\n1.0: b\n", "~: a\nnull: b\n", "1: a\n0x1: b\n", "a: 1\na: 2\n"} {
		if _, err := parse([]byte(in), "yaml"); err == nil || !strings.Contains(err.Error(), "duplicate mapping key") {
			t.Errorf("parse(%q): err = %v, want duplicate key error", in, err)
		}
	}
}

func TestParseYAML_ComplexKeyError(t *testing.T) {
	if _, err := parse([]byte("? [1, 2]\n: x\n"), "yaml"); err == nil {
		t.Error("expected error for sequence used as mapping key")
	}
}

func TestParseYAML_MergePrecedence(t *testing.T) {
	v, err := parse([]byte("a: &a {x: 1, y: 1}\nb: &b {x: 2, z: 2}\nc:\n  w: 0\n  <<: [*a, *b]\n  y: 3\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"w": 0, "x": 1, "y": 3, "z": 2}
	if got := v.(map[string]interface{})["c"]; !reflect.DeepEqual(got, want) {
		t.Errorf("c = %#v, want %#v", got, want)
	}
}

func TestParseYAML_RecursiveAnchor(t *testing.T) {
	if _, err := parse([]byte("a: &x [1, *x]\n"), "yaml"); err == nil {
		t.Error("expected error for recursive anchor")
	}
}

func TestParseYAML_AliasExpansionIsShared(t *testing.T) {
	var b strings.Builder
	b.WriteString("l0: &l0 [x, x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "l%d: &l%d [*l%d, *l%d, *l%d, *l%d, *l%d, *l%d, *l%d, *l%d, *l%d, *l%d]\n", i, i, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1)
	}
	if _, err := parse([]byte(b.String()), "yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestParseJSON_OutOfRangeClamped(t *testing.T) {
	v, err := parse([]byte(`[1e400, -1e400, 1e-400]`), "json")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{math.MaxFloat64, -math.MaxFloat64, 0.0}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("got %#v, want %#v", v, want)
	}
}

func TestParseYAML_ExplicitFloatTagKept(t *testing.T) {
	v, err := parse([]byte("x: !!float 123\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.(map[string]interface{})["x"]; got != 123.0 {
		t.Errorf("x = %#v, want float 123", got)
	}
}
