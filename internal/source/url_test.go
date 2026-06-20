package source

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestURLSource_GETDefault(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"alice"}`)
	}))
	defer srv.Close()

	u := &URLSource{URL: srv.URL, Format: "json"}
	v, err := u.Load()
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
		io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	u := &URLSource{URL: srv.URL, Format: "json", Method: "DELETE"}
	if _, err := u.Load(); err != nil {
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
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	u := &URLSource{
		URL:    srv.URL,
		Format: "json",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"Accept":        "application/json",
		},
	}
	if _, err := u.Load(); err != nil {
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
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	u := &URLSource{
		URL:    srv.URL,
		Format: "json",
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"filter":"active"}`,
	}
	if _, err := u.Load(); err != nil {
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

	u := &URLSource{URL: srv.URL, Format: "json"}
	_, err := u.Load()
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestURLSource_YAMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "name: bob\nage: 25\n")
	}))
	defer srv.Close()

	u := &URLSource{URL: srv.URL, Format: "yaml"}
	v, err := u.Load()
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
