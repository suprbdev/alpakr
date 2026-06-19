package source

import "testing"

func TestParseJSON_Object(t *testing.T) {
	data := []byte(`{"name":"alice","age":30}`)
	v, err := parseJSON(data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestParseJSON_Array(t *testing.T) {
	data := []byte(`[1,2,3]`)
	v, err := parseJSON(data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", v)
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3", len(arr))
	}
}

func TestParseJSON_Invalid(t *testing.T) {
	_, err := parseJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseYAML_Object(t *testing.T) {
	data := []byte("name: alice\nage: 30\n")
	v, err := parseYAML(data)
	if err != nil {
		t.Fatalf("parseYAML: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestParseDispatch(t *testing.T) {
	jsonData := []byte(`{"x":1}`)
	yamlData := []byte("x: 1\n")

	for _, format := range []string{"json", ""} {
		v, err := parse(jsonData, format)
		if err != nil {
			t.Errorf("parse(%q): %v", format, err)
		}
		if v.(map[string]interface{})["x"] != float64(1) {
			t.Errorf("parse(%q): x = %v, want 1", format, v.(map[string]interface{})["x"])
		}
	}

	v, err := parse(yamlData, "yaml")
	if err != nil {
		t.Errorf("parse(yaml): %v", err)
	}
	if v.(map[string]interface{})["x"] != 1 {
		t.Errorf("parse(yaml): x = %v, want 1", v.(map[string]interface{})["x"])
	}
}
