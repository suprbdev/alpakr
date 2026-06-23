package source

import (
	"bufio"
	"encoding/json"
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

// Stream calls fn for each decoded record. fn returning an error stops iteration.
func (s *NdjsonSource) Stream(fn func(interface{}) error) error {
	scanner := bufio.NewScanner(s.r())
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // up to 10 MB per line
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		lineNum++
		if len(line) == 0 {
			continue
		}
		var v interface{}
		if err := json.Unmarshal(line, &v); err != nil {
			return fmt.Errorf("ndjson line %d: %w", lineNum, err)
		}
		if err := fn(v); err != nil {
			return err
		}
	}
	return scanner.Err()
}
