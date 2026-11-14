package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

	var buf bytes.Buffer
	read := make(chan error, 1)
	go func() {
		_, err := buf.ReadFrom(r)
		read <- err
	}()

	resetFlags(args)
	execErr := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old
	if err := <-read; err != nil {
		t.Fatal(err)
	}

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
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(drained)
	}()

	resetFlags(args)
	execErr := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = old
	<-drained
	_ = r.Close()

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
	readStdin = false
	argStrings = nil
	argJSON = nil
	workers = 1
	for _, c := range rootCmd.Commands() {
		c.SilenceUsage = false
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
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
	mustWriteFile(t, dataFile, []byte(`[{"id":1},{"id":2},{"id":3}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	mustWriteFile(t, cfgFile, []byte(`
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
	mustWriteFile(t, dataFile, []byte(`[{"id":1},{"id":2}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	mustWriteFile(t, cfgFile, []byte(`
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
	mustWriteFile(t, dataFile, []byte(`[{"id":1},{"id":2},{"id":3}]`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	mustWriteFile(t, cfgFile, []byte(`
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
	mustWriteFile(t, dataFile, []byte(`{"id":1}
{"id":2}
{"id":3}
`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	mustWriteFile(t, cfgFile, []byte(`
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
	mustWriteFile(t, dataFile, []byte(`{"id":1,"v":0}
{"id":2,"v":2}
{"id":3,"v":3}
{"id":4,"v":4}
{"id":5,"v":5}
`), 0o644)

	cfgFile := filepath.Join(dir, "alpakr.yaml")
	mustWriteFile(t, cfgFile, []byte(`
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

	_ = runWithArgsExpectError(t, "run", "-c", cfgFile)
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

	_ = runWithArgsExpectError(t, "run", "--no-such-flag")
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
	fr := &flushingReader{r: strings.NewReader("x"), out: &liveOutput{w: bw}}
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
	if got, want := runWithArgs(t, "run", "-c", cfgFile), `[{"d":"2024-01-01","type":"string","one":"one"}]`+"\n"; got != want {
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

func TestRun_NdjsonRefusesToOverwriteInput(t *testing.T) {
	dataFile := writeTemp(t, "events.ndjson", "{\"a\":1}\n{\"a\":2}\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      a: ".a"
`)
	err := runWithArgsExpectError(t, "run", "-c", cfgFile, "-o", dataFile)
	if !strings.Contains(err.Error(), "is the input") {
		t.Errorf("err = %v, want same-file error", err)
	}
	if got, _ := os.ReadFile(dataFile); string(got) != "{\"a\":1}\n{\"a\":2}\n" {
		t.Errorf("input was modified: %q", got)
	}
}

func TestLimit_NdjsonStopsAfterLimit(t *testing.T) {
	dataFile := writeTemp(t, "data.ndjson", "{\"id\":1}\n{bad json\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile, "--limit", "1"); got != "{\"id\":1}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRun_FormatsCaseInsensitive(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":1}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
  format: JSON
output:
  format: Json
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile); got != "{\"id\":1}\n" {
		t.Errorf("output = %q", got)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "-f", "YAML"); got != "id: 1\n" {
		t.Errorf("-f YAML output = %q", got)
	}
}

// runWithStdin runs the command with stdin replaced by a pipe carrying input.
func runWithStdin(t *testing.T, input string, args ...string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		_ = r.Close()
	}()
	go func() {
		_, _ = io.WriteString(w, input)
		_ = w.Close()
	}()
	return runWithArgs(t, args...)
}

const stdinOnlyConfig = `
version: "1"
output:
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`

func TestStdin_ConfiguredSourceWinsOverPipe(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":"from-file"}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithStdin(t, `{"id":"from-stdin"}`, "run", "-c", cfgFile); got != `{"id":"from-file"}`+"\n" {
		t.Errorf("piped stdin replaced configured source: %q", got)
	}
	if got := runWithStdin(t, `{"id":"from-stdin"}`, "run", "-c", cfgFile, "--stdin"); got != `{"id":"from-stdin"}`+"\n" {
		t.Errorf("--stdin should read stdin: %q", got)
	}
}

func TestStdin_FlagUsesConfiguredFormat(t *testing.T) {
	dataFile := writeTemp(t, "events.ndjson", "")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithStdin(t, "{\"id\":1}\n\n{\"id\":2}\n", "run", "-c", cfgFile, "--stdin"); got != "{\"id\":1}\n{\"id\":2}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestStdin_AutoDetectStreams(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", stdinOnlyConfig)
	for name, in := range map[string]string{
		"ndjson":      "{\"id\":1}\n{\"id\":2}\n",
		"jq output":   "{\n  \"id\": 1\n}\n{\n  \"id\": 2\n}\n",
		"yaml stream": "id: 1\n---\nid: 2\n",
	} {
		if got := runWithStdin(t, in, "run", "-c", cfgFile); got != "{\"id\":1}\n{\"id\":2}\n" {
			t.Errorf("%s: output = %q", name, got)
		}
	}
}

func TestStdin_SingleDocumentIsBatch(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithStdin(t, `{"id": 1}`, "run", "-c", cfgFile); got != "{\n  \"id\": 1\n}\n" {
		t.Errorf("output = %q, want indented single document", got)
	}
}

func TestStdin_StreamKeepsNullRecords(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
handlers:
  root:
    fields:
      v: "."
`)
	if got := runWithStdin(t, "[1] null [3]", "run", "-c", cfgFile); got != "{\"v\":[1]}\n{\"v\":null}\n{\"v\":[3]}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestStdin_StreamLimit(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", stdinOnlyConfig)
	if got := runWithStdin(t, "{\"id\":1}\n{\"id\":2}\n{\"id\":3}\n", "run", "-c", cfgFile, "--limit", "2"); got != "{\"id\":1}\n{\"id\":2}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRun_MultiValueJSONFileStreams(t *testing.T) {
	dataFile := writeTemp(t, "data.json", "{\"id\":1}\n{\"id\":2}\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile); got != "{\"id\":1}\n{\"id\":2}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRun_BatchMayOverwriteItsInput(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":1,"x":2}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`)
	runWithArgs(t, "run", "-c", cfgFile, "-o", dataFile)
	if got, _ := os.ReadFile(dataFile); string(got) != `{"id":1}`+"\n" {
		t.Errorf("in-place batch output = %q", got)
	}
}

func TestRun_OutputKeepsConfigFieldOrder(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `[{"n":"x","c":"Leeds","z":1}]`)
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
      zeta: ".z"
      name: ".n"
      place:
        fields:
          town: ".c"
          code: '"GB"'
      raw: "{b: 1, a: 2}"
`)
	if got, want := runWithArgs(t, "run", "-c", cfgFile), `[{"zeta":1,"name":"x","place":{"town":"Leeds","code":"GB"},"raw":{"a":2,"b":1}}]`+"\n"; got != want {
		t.Errorf("json = %q, want %q", got, want)
	}
	want := "- zeta: 1\n  name: x\n  place:\n    town: Leeds\n    code: GB\n  raw:\n    a: 2\n    b: 1\n"
	if got := runWithArgs(t, "run", "-c", cfgFile, "-f", "yaml"); got != want {
		t.Errorf("yaml = %q, want %q", got, want)
	}
}

func TestRun_NdjsonOutputFormat(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `[{"id":1},{"id":2},{"id":3}]`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  format: ndjson
handlers:
  root:
    each: true
    fields:
      id: ".id"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile, "--limit", "2"); got != "{\"id\":1}\n{\"id\":2}\n" {
		t.Errorf("output = %q", got)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "-f", "json"); !strings.HasPrefix(got, "[\n") {
		t.Errorf("-f json should override ndjson: %q", got)
	}
}

const eachConfig = `
version: "1"
handlers:
  root:
    each: true
    fields:
      id: ".id"
`

func TestStdin_NdjsonOutputFlattensStreamedArrays(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", eachConfig)
	got := runWithStdin(t, "[{\"id\":1},{\"id\":2}]\n[{\"id\":3}]\n", "run", "-c", cfgFile, "-f", "ndjson")
	if got != "{\"id\":1}\n{\"id\":2}\n{\"id\":3}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestStdin_StreamLimitCountsElements(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", eachConfig)
	got := runWithStdin(t, "[{\"id\":1},{\"id\":2},{\"id\":3}]\n[{\"id\":4}]\n", "run", "-c", cfgFile, "-f", "ndjson", "--limit", "2")
	if got != "{\"id\":1}\n{\"id\":2}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRun_EmptyNdjsonTruncatesOutput(t *testing.T) {
	dataFile := writeTemp(t, "empty.ndjson", "")
	outFile := writeTemp(t, "out.json", "stale")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	runWithArgs(t, "run", "-c", cfgFile, "-o", outFile)
	if got, _ := os.ReadFile(outFile); len(got) != 0 {
		t.Errorf("output file = %q, want empty", got)
	}
}

func TestStdin_FlagSniffsUnlessFormatExplicit(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":"file"}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithStdin(t, "id: from-yaml\n", "run", "-c", cfgFile, "--stdin"); got != `{"id":"from-yaml"}`+"\n" {
		t.Errorf("sniffed output = %q", got)
	}
	explicit := writeTemp(t, "explicit.yaml", `
version: "1"
source:
  path: `+dataFile+`
  format: json
handlers:
  root:
    fields:
      id: ".id"
`)
	if err := runWithStdinExpectError(t, "id: from-yaml\n", "run", "-c", explicit, "--stdin"); !strings.Contains(err.Error(), "JSON") {
		t.Errorf("explicit json format should be kept: %v", err)
	}
}

func runWithStdinExpectError(t *testing.T, input string, args ...string) error {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		_ = r.Close()
	}()
	go func() {
		_, _ = io.WriteString(w, input)
		_ = w.Close()
	}()
	return runWithArgsExpectError(t, args...)
}

func TestRun_Vars(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `{"id":1}`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  indent: 0
vars:
  region: uk
  opts: {debug: false}
handlers:
  root:
    fields:
      id: ".id"
      region: "$region"
      debug: "$opts.debug"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile); got != `{"id":1,"region":"uk","debug":false}`+"\n" {
		t.Errorf("config vars: %q", got)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "--arg", "region=eu", "--argjson", `opts={"debug":true}`); got != `{"id":1,"region":"eu","debug":true}`+"\n" {
		t.Errorf("cli override: %q", got)
	}
	for _, bad := range [][]string{{"--arg", "region"}, {"--argjson", "opts={bad"}, {"--arg", "ENV=x"}} {
		if err := runWithArgsExpectError(t, append([]string{"run", "-c", cfgFile}, bad...)...); err == nil {
			t.Errorf("%v: expected error", bad)
		}
	}
}

func TestValidate_UsesArgs(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", stdinOnlyConfig+"      r: \"$region\"\n")
	if err := runWithArgsExpectError(t, "validate", "-c", cfgFile); !strings.Contains(err.Error(), "$region") {
		t.Errorf("err = %v", err)
	}
	runWithArgs(t, "validate", "-c", cfgFile, "--arg", "region=eu")
}

func TestRun_CSVSourceStreamsRows(t *testing.T) {
	dataFile := writeTemp(t, "people.csv", "id,name,age\n1,Ann,30\n2,Bob,\n3,Cy,41\n")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    filter: '.age != ""'
    fields:
      id: ".id | to_int"
      name: ".name"
      age: ".age | tonumber"
`)
	want := "{\"id\":1,\"name\":\"Ann\",\"age\":30}\n{\"id\":3,\"name\":\"Cy\",\"age\":41}\n"
	if got := runWithArgs(t, "run", "-c", cfgFile); got != want {
		t.Errorf("output = %q", got)
	}
	if got := runWithStdin(t, "id,name,age\n9,Zed,1\n", "run", "-c", cfgFile, "--stdin", "--limit", "1"); got != "{\"id\":9,\"name\":\"Zed\",\"age\":1}\n" {
		t.Errorf("--stdin keeps csv format: %q", got)
	}
}

func TestRun_CSVOutput(t *testing.T) {
	dataFile := writeTemp(t, "data.json", `[{"id":1,"n":"a, b","tags":["x"]},{"id":2,"n":"c","tags":[]}]`)
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
output:
  format: csv
handlers:
  root:
    each: true
    fields:
      name: ".n"
      id: ".id"
      tags: ".tags"
`)
	want := "name,id,tags\n\"a, b\",1,\"[\"\"x\"\"]\"\nc,2,[]\n"
	if got := runWithArgs(t, "run", "-c", cfgFile); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "--limit", "1"); got != "name,id,tags\n\"a, b\",1,\"[\"\"x\"\"]\"\n" {
		t.Errorf("limited output = %q", got)
	}
}

func TestRun_PaginatedSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"items":[{"id":1},{"id":2}],"next":"?cursor=b"}`)
		case "b":
			_, _ = io.WriteString(w, `{"items":[{"id":3}],"next":null}`)
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  url: `+srv.URL+`/api
  pagination:
    next: ".next"
    records: ".items"
handlers:
  root:
    fields:
      id: ".id"
`)
	if got := runWithArgs(t, "run", "-c", cfgFile); got != "{\"id\":1}\n{\"id\":2}\n{\"id\":3}\n" {
		t.Errorf("output = %q", got)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "-f", "csv", "--limit", "2"); got != "id\n1\n2\n" {
		t.Errorf("csv limited output = %q", got)
	}
	runWithArgs(t, "validate", "-c", cfgFile)
}

func TestValidate_BadPaginationExpr(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  url: https://example.com/api
  pagination:
    next: ".[[["
handlers:
  root:
    fields:
      id: ".id"
`)
	if err := runWithArgsExpectError(t, "validate", "-c", cfgFile); !strings.Contains(err.Error(), "pagination next") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_WorkersMatchSequential(t *testing.T) {
	var in strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&in, "{\"id\":%d,\"tags\":[\"a\",\"b\"]}\n", i)
	}
	dataFile := writeTemp(t, "data.ndjson", in.String())
	arrFile := writeTemp(t, "data.json", "["+strings.ReplaceAll(strings.TrimSpace(in.String()), "\n", ",")+"]")
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    filter: ".id % 7 != 3"
    fields:
      id: ".id"
      tags:
        handler: tag
        input: ".tags"
  tag:
    each: true
    fields:
      t: "."
`)
	for _, args := range [][]string{nil, {"--limit", "100"}, {"--limit", "2999"}, {"-f", "yaml"}} {
		base := append([]string{"run", "-c", cfgFile}, args...)
		want := runWithArgs(t, base...)
		for _, n := range []string{"4", "0"} {
			if got := runWithArgs(t, append(base, "--workers", n)...); got != want {
				t.Errorf("%v --workers %s: output differs (%d vs %d bytes)", args, n, len(got), len(want))
			}
		}
	}

	arrCfg := writeTemp(t, "arr.yaml", `
version: "1"
source:
  path: `+arrFile+`
output:
  indent: 0
handlers:
  root:
    each: true
    filter: ".id % 5 != 0"
    fields:
      id: ".id"
`)
	for _, args := range [][]string{nil, {"--limit", "777"}} {
		base := append([]string{"run", "-c", arrCfg}, args...)
		if want, got := runWithArgs(t, base...), runWithArgs(t, append(base, "--workers", "8")...); got != want {
			t.Errorf("batch %v: parallel output differs", args)
		}
	}
}

func TestRun_WorkersErrorOrder(t *testing.T) {
	var in strings.Builder
	for i := 0; i < 2000; i++ {
		switch i {
		case 600:
			in.WriteString("{\"x\":1}\n")
		case 1500:
			in.WriteString("{bad json\n")
		default:
			in.WriteString("{\"x\":\"ok\"}\n")
		}
	}
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
handlers:
  root:
    fields:
      x: ".x | ascii_upcase"
`)
	err := runWithStdinExpectError(t, in.String(), "run", "-c", cfgFile, "--workers", "4")
	if !strings.Contains(err.Error(), "(1)") {
		t.Errorf("err = %v, want the transform error of record 600", err)
	}
	in2 := strings.Replace(in.String(), "{\"x\":1}\n", "{\"x\":\"ok\"}\n", 1)
	err = runWithStdinExpectError(t, in2, "run", "-c", cfgFile, "--workers", "4")
	if !strings.Contains(err.Error(), "value 1501") {
		t.Errorf("err = %v, want the parse error of value 1501", err)
	}
	if got := runWithStdin(t, in2, "run", "-c", cfgFile, "--workers", "4", "--limit", "1000"); strings.Count(got, "\n") != 1000 {
		t.Errorf("limit before the bad value: %d lines", strings.Count(got, "\n"))
	}
}

func TestRun_WorkersInvalid(t *testing.T) {
	cfgFile := writeTemp(t, "alpakr.yaml", stdinOnlyConfig)
	if err := runWithArgsExpectError(t, "run", "-c", cfgFile, "--workers", "-1"); !strings.Contains(err.Error(), "--workers") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_WorkersNdjsonLineErrors(t *testing.T) {
	var in strings.Builder
	for i := 1; i <= 3000; i++ {
		if i == 2222 {
			in.WriteString("{oops\n")
			continue
		}
		fmt.Fprintf(&in, "{\"id\":%d}\n", i)
	}
	dataFile := writeTemp(t, "data.ndjson", in.String())
	cfgFile := writeTemp(t, "alpakr.yaml", `
version: "1"
source:
  path: `+dataFile+`
handlers:
  root:
    fields:
      id: ".id"
`)
	err := runWithArgsExpectError(t, "run", "-c", cfgFile, "--workers", "4")
	if !strings.Contains(err.Error(), "ndjson line 2222") {
		t.Errorf("err = %v", err)
	}
	if got := runWithArgs(t, "run", "-c", cfgFile, "--workers", "4", "--limit", "2000"); strings.Count(got, "\n") != 2000 || !strings.HasSuffix(got, "{\"id\":2000}\n") {
		t.Errorf("limited output wrong: %d lines", strings.Count(got, "\n"))
	}
}
