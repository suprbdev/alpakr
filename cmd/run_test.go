package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runWithArgs executes the root command with the given args and captures stdout.
func runWithArgs(t *testing.T, args ...string) string {
	t.Helper()

	// Capture os.Stdout via a pipe (writers go directly to os.Stdout, not cmd.OutOrStdout)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(args)
	handlerName = ""
	limitRecords = 0
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if execErr != nil {
		t.Fatalf("command failed: %v\noutput: %s", execErr, buf.String())
	}
	return buf.String()
}

// runWithArgsExpectError runs the command and returns the error (fails if no error).
func runWithArgsExpectError(t *testing.T, args ...string) error {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(args)
	handlerName = ""
	limitRecords = 0
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = old
	r.Close()

	if execErr == nil {
		t.Fatal("expected error but command succeeded")
	}
	return execErr
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLimit_ArraySource(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.json")
	os.WriteFile(dataFile, []byte(`[{"id":1},{"id":2},{"id":3}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	os.WriteFile(cfgFile, []byte(`
version: "1"
source:
  path: `+dataFile+`
output:
  format: json
  indent: 0
handlers:
  root:
    each: true
    fields:
      id: ".id"
`), 0o644)

	out := runWithArgs(t, "run", "-c", cfgFile, "--limit", "2")
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse output: %v\nraw: %s", err, out)
	}
	if len(result) != 2 {
		t.Errorf("got %d records, want 2", len(result))
	}
	if result[0]["id"].(float64) != 1 || result[1]["id"].(float64) != 2 {
		t.Errorf("unexpected records: %v", result)
	}
}

func TestLimit_ArraySource_LargerThanInput(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.json")
	os.WriteFile(dataFile, []byte(`[{"id":1},{"id":2}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	os.WriteFile(cfgFile, []byte(`
version: "1"
source:
  path: `+dataFile+`
output:
  format: json
  indent: 0
handlers:
  root:
    each: true
    fields:
      id: ".id"
`), 0o644)

	out := runWithArgs(t, "run", "-c", cfgFile, "--limit", "100")
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse output: %v\nraw: %s", err, out)
	}
	if len(result) != 2 {
		t.Errorf("got %d records, want 2", len(result))
	}
}

func TestLimit_NoLimit(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.json")
	os.WriteFile(dataFile, []byte(`[{"id":1},{"id":2},{"id":3}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	os.WriteFile(cfgFile, []byte(`
version: "1"
source:
  path: `+dataFile+`
output:
  format: json
  indent: 0
handlers:
  root:
    each: true
    fields:
      id: ".id"
`), 0o644)

	out := runWithArgs(t, "run", "-c", cfgFile)
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parse output: %v\nraw: %s", err, out)
	}
	if len(result) != 3 {
		t.Errorf("got %d records, want 3", len(result))
	}
}

func TestLimit_NdjsonSource(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.ndjson")
	os.WriteFile(dataFile, []byte(`{"id":1}
{"id":2}
{"id":3}
`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	os.WriteFile(cfgFile, []byte(`
version: "1"
source:
  path: `+dataFile+`
  format: ndjson
output:
  format: json
handlers:
  root:
    fields:
      id: ".id"
`), 0o644)

	out := runWithArgs(t, "run", "-c", cfgFile, "--limit", "2")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2\noutput: %s", len(lines), out)
	}
}

func TestLimit_NdjsonSource_WithFilter(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.ndjson")
	// 5 records, 3 pass filter (.v > 1), limit 2 → expect 2 output
	os.WriteFile(dataFile, []byte(`{"id":1,"v":0}
{"id":2,"v":2}
{"id":3,"v":3}
{"id":4,"v":4}
{"id":5,"v":5}
`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	os.WriteFile(cfgFile, []byte(`
version: "1"
source:
  path: `+dataFile+`
  format: ndjson
output:
  format: json
handlers:
  root:
    filter: ".v > 1"
    fields:
      id: ".id"
`), 0o644)

	out := runWithArgs(t, "run", "-c", cfgFile, "--limit", "2")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2\noutput: %s", len(lines), out)
	}
	var first map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("parse first line: %v", err)
	}
	if first["id"].(float64) != 2 {
		t.Errorf("first record id = %v, want 2", first["id"])
	}
}

func TestNoSource_NoStdin_Error(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
output:
  format: json
handlers:
  root:
    fields:
      id: ".id"
`)
	// No stdin pipe in tests (os.Stdin is a regular file descriptor, not a pipe),
	// so this should error at runtime.
	err := runWithArgsExpectError(t, "run", "-c", cfgFile)
	if err == nil {
		t.Fatal("expected error when no source and no stdin pipe")
	}
}
