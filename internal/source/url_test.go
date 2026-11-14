package source

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestURLSource_GETDefault(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"name":"alice"}`); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	format := "json"
	u := &URLSource{URL: srv.URL}
	v, err := loadURL(u, format)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "alice" {
		t.Errorf("name = %v, want alice", m["name"])
	}
}

func TestURLSource_CustomMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if _, err := io.WriteString(w, `[]`); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	format := "json"
	u := &URLSource{URL: srv.URL, Method: "DELETE"}
	if _, err := loadURL(u, format); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

func TestURLSource_Headers(t *testing.T) {
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		if _, err := io.WriteString(w, `{}`); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	u := &URLSource{
		URL: srv.URL,
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"Accept":        "application/json",
		},
	}
	if _, err := loadURL(u, "json"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotAuth != "Bearer token123" {
		t.Errorf("Authorization = %q, want Bearer token123", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestURLSource_POSTWithBody(t *testing.T) {
	var gotMethod, gotBody, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if _, err := io.WriteString(w, `{"ok":true}`); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	u := &URLSource{
		URL:    srv.URL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"filter":"active"}`,
	}
	if _, err := loadURL(u, "json"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != `{"filter":"active"}` {
		t.Errorf("body = %q, want {\"filter\":\"active\"}", gotBody)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestURLSource_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	format := "json"
	u := &URLSource{URL: srv.URL}
	_, err := loadURL(u, format)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestURLSource_YAMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "name: bob\nage: 25\n"); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	format := "yaml"
	u := &URLSource{URL: srv.URL}
	v, err := loadURL(u, format)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["name"] != "bob" {
		t.Errorf("name = %v, want bob", m["name"])
	}
}

func TestURLSource_Open(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "{\"a\":1}\n{\"a\":2}\n"); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	body, err := (&URLSource{URL: srv.URL}).Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"a\":1}\n{\"a\":2}\n" {
		t.Errorf("body = %q", data)
	}
}

func TestURLSource_OpenHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := (&URLSource{URL: srv.URL}).Open(); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want HTTP 404 error", err)
	}
}

func TestURLSource_NdjsonLoad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "{\"a\":1}\n{\"a\":2}\n"); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	v, err := loadURL(&URLSource{URL: srv.URL}, "ndjson")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if arr, ok := v.([]interface{}); !ok || len(arr) != 2 {
		t.Errorf("got %#v, want 2-element array", v)
	}
}

// loadURL fetches u and returns its single value, or all values as an array
// for ndjson.
func loadURL(u *URLSource, format string) (interface{}, error) {
	body, err := u.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	if format == "ndjson" {
		return readAll(body, format)
	}
	return readOne(body, format)
}

func TestURLSource_TimeoutWaitingForHeaders(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := (&URLSource{URL: srv.URL, Timeout: 50 * time.Millisecond}).Open()
	if err == nil || !strings.Contains(err.Error(), "no response within 50ms") {
		t.Fatalf("err = %v, want header timeout", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("timeout took %s", time.Since(start))
	}
}

func TestURLSource_TimeoutStalledBody(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"a\":1}\n")
		w.(http.Flusher).Flush()
		<-release
	}))
	defer srv.Close()
	defer close(release)

	body, err := (&URLSource{URL: srv.URL, Timeout: 50 * time.Millisecond}).Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()
	_, err = io.ReadAll(body)
	if err == nil || !strings.Contains(err.Error(), "no data received for 50ms") {
		t.Fatalf("err = %v, want stalled body error", err)
	}
}

func TestURLSource_TimeoutIsPerRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 5; i++ {
			_, _ = fmt.Fprintf(w, "{\"i\":%d}\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}))
	defer srv.Close()

	got, err := loadURL(&URLSource{URL: srv.URL, Timeout: 100 * time.Millisecond}, "ndjson")
	if err != nil {
		t.Fatalf("slow but steady body should not time out: %v", err)
	}
	if n := len(got.([]interface{})); n != 5 {
		t.Errorf("got %d records, want 5", n)
	}
}

func TestURLSource_ZeroTimeoutDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	if _, err := loadURL(&URLSource{URL: srv.URL}, "json"); err != nil {
		t.Fatal(err)
	}
}
