package source

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type URLSource struct {
	URL     string
	Format  string
	Method  string
	Headers map[string]string
	Body    string
}

func (u *URLSource) Load() (interface{}, error) {
	method := u.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if u.Body != "" {
		bodyReader = strings.NewReader(u.Body)
	}

	req, err := http.NewRequest(method, u.URL, bodyReader) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("building request for %q: %w", u.URL, err)
	}

	for k, v := range u.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
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
