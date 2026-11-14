package source

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
)

// Option configures NewValues.
type Option func(*options)

type options struct {
	csvComma    rune
	csvNoHeader bool
}

// CSVDelimiter sets the CSV field delimiter (default ',').
func CSVDelimiter(r rune) Option { return func(o *options) { o.csvComma = r } }

// CSVHeader sets whether the first CSV row names the columns (default true).
// Without a header, each row is an array of strings.
func CSVHeader(header bool) Option { return func(o *options) { o.csvNoHeader = !header } }

type csvValues struct {
	r      *csv.Reader
	header []string
	noHdr  bool
}

func newCSVValues(in io.Reader, o options) *csvValues {
	r := csv.NewReader(in)
	if o.csvComma != 0 {
		r.Comma = o.csvComma
	}
	return &csvValues{r: r, noHdr: o.csvNoHeader}
}

// Next returns the next row: an object keyed by the header, or an array of
// strings when there is no header. Every value is a string.
func (c *csvValues) Next() (interface{}, bool, error) {
	if c.header == nil && !c.noHdr {
		row, err := c.r.Read()
		if errors.Is(err, io.EOF) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("parsing CSV header: %w", err)
		}
		seen := make(map[string]bool, len(row))
		for i, name := range row {
			if name == "" {
				return nil, false, fmt.Errorf("parsing CSV header: column %d has no name", i+1)
			}
			if seen[name] {
				return nil, false, fmt.Errorf("parsing CSV header: duplicate column %q", name)
			}
			seen[name] = true
		}
		c.header = row
	}

	row, err := c.r.Read()
	if errors.Is(err, io.EOF) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("parsing CSV: %w", err)
	}
	if c.noHdr {
		out := make([]interface{}, len(row))
		for i, v := range row {
			out[i] = v
		}
		return out, true, nil
	}
	out := make(map[string]interface{}, len(row))
	for i, name := range c.header {
		out[name] = row[i]
	}
	return out, true, nil
}
