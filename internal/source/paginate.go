package source

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/itchyny/gojq"
)

// Pagination holds the compiled jq expressions that walk a paginated API.
type Pagination struct {
	next    *gojq.Code
	records *gojq.Code
}

// CompilePagination compiles the next-URL expression and the optional
// records expression. Both see the page body as input and $response, an
// object with the page's url, status and headers (lower-cased names, first
// value of each).
func CompilePagination(next, records string) (*Pagination, error) {
	opts := []gojq.CompilerOption{gojq.WithVariables([]string{"$response"}), gojq.WithEnvironLoader(os.Environ)}
	p := &Pagination{}
	var err error
	if p.next, err = compilePaginationExpr("next", next, opts); err != nil {
		return nil, err
	}
	if records != "" {
		if p.records, err = compilePaginationExpr("records", records, opts); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func compilePaginationExpr(name, src string, opts []gojq.CompilerOption) (*gojq.Code, error) {
	q, err := gojq.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("pagination %s: invalid jq expression %q: %w", name, src, err)
	}
	code, err := gojq.Compile(q, opts...)
	if err != nil {
		return nil, fmt.Errorf("pagination %s: compiling jq expression %q: %w", name, src, err)
	}
	return code, nil
}

// Pages streams records from a paginated URL source. Each page is fetched
// only when the records of the previous one have been consumed.
type Pages struct {
	// Source is the first page's request; later pages reuse its method,
	// headers, body and timeout with the URL returned by next.
	Source   URLSource
	Format   string // json or yaml
	MaxPages int    // 0 = unlimited
	// BeforeFetch, if set, runs before each page request.
	BeforeFetch func() error
	// Warn, if set, receives a message when MaxPages stops pagination early.
	Warn func(string)

	p       *Pagination
	nextURL string
	pages   int
	queue   []interface{}
	done    bool
	seen    map[string]bool
}

// NewPages returns an iterator over the records of src's pages.
func NewPages(src URLSource, format string, p *Pagination, maxPages int) *Pages {
	return &Pages{Source: src, Format: format, MaxPages: maxPages, p: p, nextURL: src.URL, seen: map[string]bool{}}
}

// Next returns the next record, fetching further pages as needed.
func (ps *Pages) Next() (interface{}, bool, error) {
	for len(ps.queue) == 0 {
		if ps.done {
			return nil, false, nil
		}
		if err := ps.fetch(); err != nil {
			return nil, false, err
		}
	}
	v := ps.queue[0]
	ps.queue[0] = nil
	ps.queue = ps.queue[1:]
	return v, true, nil
}

func (ps *Pages) fetch() error {
	if ps.BeforeFetch != nil {
		if err := ps.BeforeFetch(); err != nil {
			return err
		}
	}
	req := ps.Source
	req.URL = ps.nextURL
	ps.seen[req.URL] = true
	ps.pages++

	resp, err := req.Fetch()
	if err != nil {
		return fmt.Errorf("page %d: %w", ps.pages, err)
	}
	page, err := readPage(resp, ps.Format)
	if err != nil {
		return fmt.Errorf("page %d (%s): %w", ps.pages, req.URL, err)
	}
	meta := responseValue(resp)

	if ps.p.records == nil {
		ps.queue = append(ps.queue, page)
	} else {
		recs, err := runPaginationExpr(ps.p.records, page, meta)
		if err != nil {
			return fmt.Errorf("page %d: pagination records: %w", ps.pages, err)
		}
		switch r := recs.(type) {
		case nil:
		case []interface{}:
			ps.queue = append(ps.queue, r...)
		default:
			return fmt.Errorf("page %d: pagination records must produce an array, got %s", ps.pages, jqTypeName(recs))
		}
	}

	next, err := runPaginationExpr(ps.p.next, page, meta)
	if err != nil {
		return fmt.Errorf("page %d: pagination next: %w", ps.pages, err)
	}
	switch n := next.(type) {
	case nil, bool:
		if n == true {
			return fmt.Errorf("page %d: pagination next must produce a URL string or null, got true", ps.pages)
		}
		ps.done = true
		return nil
	case string:
		if n == "" {
			ps.done = true
			return nil
		}
		u, err := resolveURL(resp.URL, n)
		if err != nil {
			return fmt.Errorf("page %d: pagination next %q: %w", ps.pages, n, err)
		}
		if ps.seen[u] {
			return fmt.Errorf("page %d: pagination next %q was already fetched (pagination loop)", ps.pages, u)
		}
		if ps.MaxPages > 0 && ps.pages >= ps.MaxPages {
			ps.done = true
			if ps.Warn != nil {
				ps.Warn(fmt.Sprintf("stopped after max_pages (%d); more pages remain at %s", ps.MaxPages, u))
			}
			return nil
		}
		ps.nextURL = u
		return nil
	}
	return fmt.Errorf("page %d: pagination next must produce a URL string or null, got %s", ps.pages, jqTypeName(next))
}

func readPage(resp *Response, format string) (interface{}, error) {
	defer func() { _ = resp.Body.Close() }()
	vals, err := NewValues(resp.Body, format)
	if err != nil {
		return nil, err
	}
	v, ok, err := vals.Next()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if _, more, err := vals.Next(); err != nil || more {
		if err == nil {
			err = fmt.Errorf("page holds more than one %s value", format)
		}
		return nil, err
	}
	return v, nil
}

func responseValue(resp *Response) map[string]interface{} {
	headers := make(map[string]interface{}, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	return map[string]interface{}{"url": resp.URL, "status": resp.Status, "headers": headers}
}

func runPaginationExpr(code *gojq.Code, page interface{}, meta map[string]interface{}) (interface{}, error) {
	iter := code.Run(page, meta)
	v, ok := iter.Next()
	if !ok {
		return nil, nil
	}
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v, nil
}

func resolveURL(base, ref string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}

func jqTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	}
	return "number"
}
