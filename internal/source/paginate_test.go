package source

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// pagedServer serves /items?page=N for N in 1..pages with two items each.
func pagedServer(t *testing.T, pages int, requests *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(requests, 1)
		var n int
		_, _ = fmt.Sscanf(r.URL.Query().Get("page"), "%d", &n)
		if n == 0 {
			n = 1
		}
		next := "null"
		if n < pages {
			next = fmt.Sprintf(`"/items?page=%d"`, n+1)
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?page=%d>; rel="next"`, "http://"+r.Host, n+1))
		}
		_, _ = fmt.Fprintf(w, `{"data":[{"id":%d},{"id":%d}],"links":{"next":%s}}`, 2*n-1, 2*n, next)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func collect(t *testing.T, ps *Pages) []interface{} {
	t.Helper()
	var out []interface{}
	for {
		v, ok, err := ps.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func ids(vs []interface{}) string {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprint(&b, v.(map[string]interface{})["id"], " ")
	}
	return strings.TrimSpace(b.String())
}

func TestPages_BodyLinkWithRecords(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 3, &reqs)
	p, err := CompilePagination(".links.next", ".data")
	if err != nil {
		t.Fatal(err)
	}
	got := collect(t, NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 0))
	if ids(got) != "1 2 3 4 5 6" || reqs != 3 {
		t.Errorf("records %q after %d requests", ids(got), reqs)
	}
}

func TestPages_LinkHeader(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 2, &reqs)
	p, err := CompilePagination(`$response.headers.link // "" | capture("<(?<u>[^>]+)>; rel=\"next\"").u // null`, ".data")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(collect(t, NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 0))); got != "1 2 3 4" {
		t.Errorf("records %q", got)
	}
}

func TestPages_WholePageWithoutRecords(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 2, &reqs)
	p, _ := CompilePagination(".links.next", "")
	got := collect(t, NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 0))
	if len(got) != 2 {
		t.Fatalf("got %d pages, want 2", len(got))
	}
}

func TestPages_MaxPagesWarns(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 5, &reqs)
	p, _ := CompilePagination(".links.next", ".data")
	ps := NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 2)
	var warned string
	ps.Warn = func(msg string) { warned = msg }
	if got := ids(collect(t, ps)); got != "1 2 3 4" || reqs != 2 {
		t.Errorf("records %q after %d requests", got, reqs)
	}
	if !strings.Contains(warned, "max_pages (2)") {
		t.Errorf("warning = %q", warned)
	}
}

func TestPages_FetchesLazily(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 10, &reqs)
	p, _ := CompilePagination(".links.next", ".data")
	ps := NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 0)
	for i := 0; i < 3; i++ {
		if _, _, err := ps.Next(); err != nil {
			t.Fatal(err)
		}
	}
	if reqs != 2 {
		t.Errorf("3 records needed %d requests, want 2", reqs)
	}
}

func TestPages_Errors(t *testing.T) {
	var reqs int32
	srv := pagedServer(t, 3, &reqs)
	for next, want := range map[string]string{
		`"/items?page=1"`: "pagination loop",
		"42":              "got number",
		"true":            "got true",
	} {
		p, err := CompilePagination(next, ".data")
		if err != nil {
			t.Fatal(err)
		}
		ps := NewPages(URLSource{URL: srv.URL + "/items?page=1"}, "json", p, 0)
		var gotErr error
		for gotErr == nil {
			var ok bool
			if _, ok, gotErr = ps.Next(); !ok && gotErr == nil {
				break
			}
		}
		if gotErr == nil || !strings.Contains(gotErr.Error(), want) {
			t.Errorf("next %s: err = %v, want %q", next, gotErr, want)
		}
	}
	p, _ := CompilePagination("null", ".links")
	if _, _, err := NewPages(URLSource{URL: srv.URL + "/items"}, "json", p, 0).Next(); err == nil || !strings.Contains(err.Error(), "must produce an array") {
		t.Errorf("non-array records: err = %v", err)
	}
	if _, err := CompilePagination(".[[[", ""); err == nil {
		t.Error("expected compile error")
	}
}
