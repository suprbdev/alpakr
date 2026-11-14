package source

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Values iterates over the top-level values of an input.
type Values interface {
	// Next returns the next value; ok is false once the input is exhausted.
	Next() (v interface{}, ok bool, err error)
}

// NewValues returns an iterator over the values in r. format is one of:
//
//   - "json": one or more JSON values, e.g. a single document, NDJSON,
//     or concatenated pretty-printed values such as jq output
//   - "yaml" or "yml": one or more YAML documents separated by "---"
//   - "ndjson": one JSON value per line; blank lines are skipped and
//     errors report the line number
//   - "": json if the first non-space byte is '{' or '[', otherwise yaml
//
// A leading UTF-8 byte-order mark is ignored.
func NewValues(r io.Reader, format string) (Values, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(len(utf8BOM)); err == nil && bytes.Equal(b, utf8BOM) {
		if _, err := br.Discard(len(utf8BOM)); err != nil {
			return nil, err
		}
	}
	var in io.Reader = br
	if format == "" {
		var ws []byte
		format, ws = detectFormat(br)
		in = io.MultiReader(bytes.NewReader(ws), br)
	}
	switch format {
	case "json":
		dec := json.NewDecoder(in)
		dec.UseNumber()
		return &jsonValues{dec: dec}, nil
	case "yaml", "yml":
		return &yamlValues{dec: yaml.NewDecoder(in)}, nil
	case "ndjson":
		sc := bufio.NewScanner(in)
		sc.Buffer(make([]byte, 64*1024), maxLine)
		return &ndjsonValues{sc: sc}, nil
	}
	return nil, fmt.Errorf("unknown input format %q", format)
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// detectFormat reads past leading whitespace to the first other byte: '{' or
// '[' → json, anything else (including empty input) → yaml. The whitespace
// consumed is returned so the caller can replay it; YAML indentation on the
// first line is significant.
func detectFormat(br *bufio.Reader) (string, []byte) {
	var ws []byte
	for {
		c, err := br.ReadByte()
		if err != nil {
			return "yaml", ws
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			ws = append(ws, c)
			continue
		}
		_ = br.UnreadByte()
		if c == '{' || c == '[' {
			return "json", ws
		}
		return "yaml", ws
	}
}

type jsonValues struct {
	dec *json.Decoder
	n   int
}

func (j *jsonValues) Next() (interface{}, bool, error) {
	var v interface{}
	err := j.dec.Decode(&v)
	if errors.Is(err, io.EOF) {
		if j.n == 0 {
			return nil, false, errors.New("parsing JSON: empty input")
		}
		return nil, false, nil
	}
	j.n++
	if err != nil {
		return nil, false, fmt.Errorf("parsing JSON value %d (offset %d): %w", j.n, j.dec.InputOffset(), err)
	}
	return normalizeJSON(v), true, nil
}

type yamlValues struct {
	dec *yaml.Decoder
	n   int
}

// Next returns the next document. Empty input yields a single null
// document, matching yaml.Unmarshal.
func (y *yamlValues) Next() (interface{}, bool, error) {
	var doc yaml.Node
	err := y.dec.Decode(&doc)
	if errors.Is(err, io.EOF) {
		if y.n == 0 {
			y.n++
			return nil, true, nil
		}
		return nil, false, nil
	}
	y.n++
	if err != nil {
		return nil, false, fmt.Errorf("parsing YAML document %d: %w", y.n, err)
	}
	v, err := yamlToValue(&doc)
	if err != nil {
		return nil, false, fmt.Errorf("parsing YAML document %d: %w", y.n, err)
	}
	return v, true, nil
}

// maxLine is the longest NDJSON line accepted.
const maxLine = 10 << 20

type ndjsonValues struct {
	sc   *bufio.Scanner
	line int
}

func (s *ndjsonValues) Next() (interface{}, bool, error) {
	for s.sc.Scan() {
		s.line++
		line := bytes.TrimSpace(s.sc.Bytes())
		if len(line) == 0 {
			continue
		}
		v, err := parseJSON(line)
		if err != nil {
			return nil, false, fmt.Errorf("ndjson line %d: %w", s.line, err)
		}
		return v, true, nil
	}
	if err := s.sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, false, fmt.Errorf("ndjson line %d: exceeds %d MB limit", s.line+1, maxLine>>20)
		}
		return nil, false, fmt.Errorf("reading ndjson: %w", err)
	}
	return nil, false, nil
}
