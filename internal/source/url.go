package source

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// URLSource fetches input over HTTP.
type URLSource struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

// Open performs the HTTP request and returns the response body. The caller
// must close it. Non-2xx responses are returned as errors.
func (u *URLSource) Open() (io.ReadCloser, error) {
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("fetching %q: HTTP %d", u.URL, resp.StatusCode)
	}
	return resp.Body, nil
}
