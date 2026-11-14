package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	resetFlags(args)
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

	resetFlags(args)
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = old
	r.Close()

	if execErr == nil {
		t.Fatal("expected error but command succeeded")
	}
	return execErr
}

// resetFlags clears state left behind by earlier Execute calls in the same process.
func resetFlags(args []string) {
	rootCmd.SetArgs(args)
	handlerName = ""
	limitRecords = 0
	outputFormat = ""
	outputFile = ""
	for _, c := range rootCmd.Commands() {
		c.SilenceUsage = false
	}
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

func TestRuntimeError_NoUsage(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: /nonexistent/data.json
handlers:
  root:
    fields:
      id: ".id"
`)
	var stderr bytes.Buffer
	rootCmd.SetOut(&stderr)
	rootCmd.SetErr(&stderr)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	runWithArgsExpectError(t, "run", "-c", cfgFile)
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("runtime error printed usage:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "nonexistent") {
		t.Errorf("error printed by cobra; Execute should print it once:\n%s", stderr.String())
	}
}

func TestFlagError_ShowsUsage(t *testing.T) {
	var stderr bytes.Buffer
	rootCmd.SetOut(&stderr)
	rootCmd.SetErr(&stderr)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	runWithArgsExpectError(t, "run", "--no-such-flag")
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("flag error should print usage, got:\n%s", stderr.String())
	}
}

func TestRun_UnknownOutputFormat(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":1}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	err := runWithArgsExpectError(t, "run", "-c", cfgFile, "-f", "toml")
	if !strings.Contains(err.Error(), "toml") {
		t.Errorf("error = %v, want mention of unknown format", err)
	}
}

func TestRun_OutputFile(t *testing.T) {
	dir := t.TempDir()
	dataFile := writeTemp(t, "data.json", `[{"u":"a&b"},{"u":"<c>"}]`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    each: true
    fields:
      u: ".u"
`)
	outFile := filepath.Join(dir, "nested", "out.json")
	if out := runWithArgs(t, "run", "-c", cfgFile, "-o", outFile); out != "" {
		t.Errorf("stdout should be empty when writing to file, got %q", out)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"u":"a&b"},{"u":"<c>"}]` + "\n"; string(got) != want {
		t.Errorf("output file = %q, want %q", got, want)
	}
}

func TestRun_NdjsonOutputFile(t *testing.T) {
	dir := t.TempDir()
	dataFile := writeTemp(t, "data.ndjson", "{\"id\":1}\n{\"id\":2}\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	outFile := filepath.Join(dir, "out.ndjson")
	runWithArgs(t, "run", "-c", cfgFile, "-o", outFile)
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\"id\":1}\n{\"id\":2}\n"; string(got) != want {
		t.Errorf("output file = %q, want %q", got, want)
	}
}

func TestFlushingReader_FlushesBeforeRead(t *testing.T) {
	var out bytes.Buffer
	bw := bufio.NewWriter(&out)
	fr := &flushingReader{r: strings.NewReader("x"), w: bw}
	if _, err := bw.WriteString("pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := fr.Read(make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	if out.String() != "pending" {
		t.Errorf("buffered output not flushed before read: %q", out.String())
	}
}

func TestRun_NdjsonURLSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "{\"id\":1}\n{\"id\":2}\n{\"id\":3}\n"); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  url: `+srv.URL+`/events.ndjson?page=1
handlers:
  root:
    filter: ".id != 2"
    fields:
      id: ".id"
`)
	if got, want := runWithArgs(t, "run", "-c", cfgFile), "{\"id\":1}\n{\"id\":3}\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRun_MultiSourceSubHandler(t *testing.T) {
	dataFile := writeTemp(t, "a.json", `[{"loc":{"city":"Leeds"}}]`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
sources:
  a:
    path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    source: a
    each: true
    fields:
      loc:
        handler: loc
        input: ".loc"
  loc:
    fields:
      city: ".city"
`)
	if got, want := runWithArgs(t, "run", "-c", cfgFile), `[{"loc":{"city":"Leeds"}}]`+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRun_YAMLSourceTimestampsAndKeys(t *testing.T) {
	dataFile := writeTemp(t, "data.yaml", "- d: 2024-01-01\n  codes: {1: one, 2: two}\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    each: true
    fields:
      d: ".d"
      type: ".d | type"
      one: '.codes["1"]'
`)
	if got, want := runWithArgs(t, "run", "-c", cfgFile), `[{"d":"2024-01-01","one":"one","type":"string"}]`+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRun_LargeIntegersPreserved(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `[{"id":12345678901234567890,"n":9007199254740993}]`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    each: true
    fields:
      id: ".id"
      n: ".n"
      next: ".n + 1"
`)
	if got, want := runWithArgs(t, "run", "-c", cfgFile), `[{"id":12345678901234567890,"n":9007199254740993,"next":9007199254740994}]`+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
