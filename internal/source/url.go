package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// URLSource fetches input over HTTP.
type URLSource struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
	// Timeout bounds every wait on the server: connecting and receiving the
	// response headers, then each read of the body. A body that keeps
	// arriving may take longer in total. Zero disables the timeout.
	Timeout time.Duration
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

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, method, u.URL, bodyReader) //nolint:gosec
	if err != nil {
		cancel()
		return nil, fmt.Errorf("building request for %q: %w", u.URL, err)
	}
	for k, v := range u.Headers {
		req.Header.Set(k, v)
	}

	var timer *time.Timer
	if u.Timeout > 0 {
		timer = time.AfterFunc(u.Timeout, cancel)
	}
	resp, err := http.DefaultClient.Do(req)
	timedOut := timer != nil && !timer.Stop()
	if err != nil || timedOut {
		if resp != nil {
			_ = resp.Body.Close()
		}
		cancel()
		if timedOut {
			return nil, fmt.Errorf("fetching %q: no response within %s", u.URL, u.Timeout)
		}
		return nil, fmt.Errorf("fetching %q: %w", u.URL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("fetching %q: HTTP %d", u.URL, resp.StatusCode)
	}
	return &timeoutBody{body: resp.Body, timer: timer, timeout: u.Timeout, cancel: cancel, url: u.URL}, nil
}

// timeoutBody cancels the request when a single Read blocks for longer than
// timeout.
type timeoutBody struct {
	body    io.ReadCloser
	timer   *time.Timer
	timeout time.Duration
	cancel  context.CancelFunc
	url     string
}

func (b *timeoutBody) Read(p []byte) (int, error) {
	if b.timer == nil {
		return b.body.Read(p)
	}
	b.timer.Reset(b.timeout)
	n, err := b.body.Read(p)
	if !b.timer.Stop() {
		return n, fmt.Errorf("reading %q: no data received for %s", b.url, b.timeout)
	}
	return n, err
}

func (b *timeoutBody) Close() error {
	if b.timer != nil {
		b.timer.Stop()
	}
	b.cancel()
	return b.body.Close()
}
