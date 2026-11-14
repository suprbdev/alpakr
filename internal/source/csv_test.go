package source

import (
	"reflect"
	"strings"
	"testing"
)

func TestCSV_Header(t *testing.T) {
	in := "\xEF\xBB\xBFid,name,note\n1,Ann,\"hi, there\"\n2,Bob,\"multi\nline\"\n"
	got, err := readAll(strings.NewReader(in), "csv")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		map[string]interface{}{"id": "1", "name": "Ann", "note": "hi, there"},
		map[string]interface{}{"id": "2", "name": "Bob", "note": "multi\nline"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestCSV_NoHeaderAndDelimiter(t *testing.T) {
	vals, err := NewValues(strings.NewReader("a\tb\n007\t\n"), "csv", CSVDelimiter('\t'), CSVHeader(false))
	if err != nil {
		t.Fatal(err)
	}
	var got []interface{}
	for {
		v, ok, err := vals.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		got = append(got, v)
	}
	want := []interface{}{[]interface{}{"a", "b"}, []interface{}{"007", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestCSV_EmptyAndHeaderOnly(t *testing.T) {
	for _, in := range []string{"", "id,name\n"} {
		got, err := readAll(strings.NewReader(in), "csv")
		if err != nil || len(got) != 0 {
			t.Errorf("%q: got %v, %v; want no records", in, got, err)
		}
	}
}

func TestCSV_Errors(t *testing.T) {
	for in, want := range map[string]string{
		"a,b\n1,2,3\n":  "wrong number of fields",
		"a,a\n1,2\n":    `duplicate column "a"`,
		"a,,c\n1,2,3\n": "column 2 has no name",
		"a\n\"x\n":      "parsing CSV",
	} {
		if _, err := readAll(strings.NewReader(in), "csv"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", in, err, want)
		}
	}
}
