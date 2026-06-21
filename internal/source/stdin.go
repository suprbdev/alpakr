package source

import (
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
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return parse(data, s.Format)
}
