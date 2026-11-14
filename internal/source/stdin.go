package source

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

type StdinSource struct {
	Format string
	reader io.Reader // overridable in tests
}

func (s *StdinSource) Load() (interface{}, error) {
	r := s.reader
	if r == nil {
		r = os.Stdin
	}

	br := bufio.NewReader(r)
	format := s.Format
	if format == "" {
		var err error
		format, err = detectStdinFormat(br)
		if err != nil {
			return nil, fmt.Errorf("detecting stdin format: %w", err)
		}
	}

	data, err := io.ReadAll(br)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return parse(data, format)
}

// detectStdinFormat peeks at the first non-whitespace byte to guess the format.
// '{' or '[' → json; anything else → yaml.
func detectStdinFormat(r *bufio.Reader) (string, error) {
	if b, err := r.Peek(len(utf8BOM)); err == nil && bytes.Equal(b, utf8BOM) {
		if _, err := r.Discard(len(utf8BOM)); err != nil {
			return "", err
		}
	}
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "json", nil // empty input; json parse will handle it
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		if err := r.UnreadByte(); err != nil {
			return "", err
		}
		if b == '{' || b == '[' {
			return "json", nil
		}
		return "yaml", nil
	}
}
