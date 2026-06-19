package source

import (
	"fmt"
	"os"
)

type FileSource struct {
	Path   string
	Format string
}

func (f *FileSource) Load() (interface{}, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", f.Path, err)
	}
	return parse(data, f.Format)
}
