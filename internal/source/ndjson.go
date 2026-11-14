package source

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// NdjsonSource streams newline-delimited JSON records one at a time.
// It implements Load() for compatibility but also exposes Stream() for
// memory-efficient processing of large inputs.
type NdjsonSource struct {
	Reader io.Reader // if nil, reads from os.Stdin
}

func (s *NdjsonSource) r() io.Reader {
	if s.Reader != nil {
		return s.Reader
	}
	return os.Stdin
}

// Load reads all records into memory. Prefer Stream for large inputs.
func (s *NdjsonSource) Load() (interface{}, error) {
	var records []interface{}
	err := s.Stream(func(v interface{}) error {
		records = append(records, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// maxLine is the longest NDJSON line Stream accepts.
const maxLine = 10 << 20

// Stream calls fn for each decoded record. Blank and whitespace-only lines are
// skipped. fn returning an error stops iteration and that error is returned.
func (s *NdjsonSource) Stream(fn func(interface{}) error) error {
	scanner := bufio.NewScanner(s.r())
	scanner.Buffer(make([]byte, 1024*1024), maxLine)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if lineNum == 1 {
			line = trimBOM(line)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		v, err := parseJSON(line)
		if err != nil {
			return fmt.Errorf("ndjson line %d: %w", lineNum, err)
		}
		if err := fn(v); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("ndjson line %d: exceeds %d MB limit", lineNum+1, maxLine>>20)
		}
		return fmt.Errorf("reading ndjson: %w", err)
	}
	return nil
}
