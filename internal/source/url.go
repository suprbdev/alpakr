package source

import (
	"fmt"
	"io"
	"net/http"
)

type URLSource struct {
	URL    string
	Format string
}

func (u *URLSource) Load() (interface{}, error) {
	resp, err := http.Get(u.URL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", u.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %q: HTTP %d", u.URL, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %q: %w", u.URL, err)
	}
	return parse(data, u.Format)
}
