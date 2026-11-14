package source

import (
	"strings"
	"testing"
)

func TestNdjson_Stream(t *testing.T) {
	input := `{"name":"alice","age":30}` + "\n" + `{"name":"bob","age":25}` + "\n"
	records, err := readAll(strings.NewReader(input), "ndjson")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	m := records[0].(map[string]interface{})
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestNdjson_SkipsBlankLines(t *testing.T) {
	input := `{"x":1}` + "\n\n" + `{"x":2}` + "\n"
	records, err := readAll(strings.NewReader(input), "ndjson")
	count := len(records)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d records, want 2", count)
	}
}

func TestNdjson_InvalidJSON(t *testing.T) {
	input := `{"a":1}` + "\n" + `not json` + "\n"
	_, err := readAll(strings.NewReader(input), "ndjson")
	if err == nil {
		t.Fatal("expected error for invalid JSON line")
	}
}

func TestNdjson_SkipsWhitespaceAndCRLFLines(t *testing.T) {
	input := "\xEF\xBB\xBF{\"x\":1}\r\n   \r\n\t\n\r\n{\"x\":2}\r\n  {\"x\":3}  \n"
	records, err := readAll(strings.NewReader(input), "ndjson")
	var got []interface{}
	for _, r := range records {
		got = append(got, r.(map[string]interface{})["x"])
	}
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", got)
	}
}

func TestNdjson_ErrorLineNumber(t *testing.T) {
	input := "{\"x\":1}\n\n{bad}\n"
	_, err := readAll(strings.NewReader(input), "ndjson")
	if err == nil || !strings.HasPrefix(err.Error(), "ndjson line 3:") {
		t.Errorf("err = %v, want ndjson line 3 error", err)
	}
}

func TestNdjson_LineTooLong(t *testing.T) {
	input := "{\"x\":1}\n{\"s\":\"" + strings.Repeat("a", maxLine) + "\"}\n"
	_, err := readAll(strings.NewReader(input), "ndjson")
	if err == nil || !strings.Contains(err.Error(), "ndjson line 2: exceeds") {
		t.Errorf("err = %v, want line-too-long error for line 2", err)
	}
}
