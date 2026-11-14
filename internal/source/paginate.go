package source

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
)

// Pagination holds the compiled jq expressions that walk a paginated API.
type Pagination struct {
	next    *gojq.Code
	records *gojq.Code
	vars    []interface{}
}

// CompilePagination compiles the next-URL expression and the optional
// records expression. Both see the page body as input, $response (an object
// with the page's url, status and headers — lower-cased names, first value
// of each) and every variable in vars as $name.
func CompilePagination(next, records string, vars map[string]interface{}) (*Pagination, error) {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	jqNames := []string{"$response"}
	p := &Pagination{}
	for _, name := range names {
		jqNames = append(jqNames, "$"+name)
		p.vars = append(p.vars, vars[name])
	}
	opts := []gojq.CompilerOption{gojq.WithVariables(jqNames), gojq.WithEnvironLoader(os.Environ)}
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

// Pages streams records from a paginated URL source. A page is fetched,
// and the previous page's next expression evaluated, only once the records
// already fetched have been consumed.
type Pages struct {
	// Source is the first page's request. Later pages reuse its method,
	// body and timeout with the URL returned by next; its headers are only
	// sent to the first URL's scheme, host and port, so credentials are not
	// leaked to other origins a next link may name.
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
	last    *fetchedPage // most recent page; its next is not evaluated yet
}

type fetchedPage struct {
	body interface{}
	meta map[string]interface{}
	url  string
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
	if ps.last != nil {
		more, err := ps.advance()
		if err != nil || !more {
			ps.done = true
			return err
		}
	}
	if ps.BeforeFetch != nil {
		if err := ps.BeforeFetch(); err != nil {
			return err
		}
	}
	req := ps.Source
	req.URL = ps.nextURL
	if !sameOrigin(req.URL, ps.Source.URL) {
		req.Headers = nil
	}
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
	ps.last = &fetchedPage{body: page, meta: meta, url: resp.URL}

	if ps.p.records == nil {
		ps.queue = append(ps.queue, page)
		return nil
	}
	recs, err := ps.run(ps.p.records, page, meta)
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
	return nil
}

// advance evaluates next on the last page and sets nextURL. It reports
// false when pagination is over.
func (ps *Pages) advance() (bool, error) {
	last := ps.last
	ps.last = nil
	next, err := ps.run(ps.p.next, last.body, last.meta)
	if err != nil {
		return false, fmt.Errorf("page %d: pagination next: %w", ps.pages, err)
	}
	var ref string
	switch n := next.(type) {
	case nil:
		return false, nil
	case bool:
		if n {
			return false, fmt.Errorf("page %d: pagination next must produce a URL string or null, got true", ps.pages)
		}
		return false, nil
	case string:
		ref = n
	default:
		return false, fmt.Errorf("page %d: pagination next must produce a URL string or null, got %s", ps.pages, jqTypeName(next))
	}
	if ref == "" {
		return false, nil
	}
	u, err := resolveURL(last.url, ref)
	if err != nil {
		return false, fmt.Errorf("page %d: pagination next %q: %w", ps.pages, ref, err)
	}
	if ps.seen[u] {
		return false, fmt.Errorf("page %d: pagination next %q was already fetched (pagination loop)", ps.pages, u)
	}
	if ps.MaxPages > 0 && ps.pages >= ps.MaxPages {
		if ps.Warn != nil {
			ps.Warn(fmt.Sprintf("stopped after max_pages (%d); more pages remain at %s", ps.MaxPages, u))
		}
		return false, nil
	}
	ps.nextURL = u
	return true, nil
}

func (ps *Pages) run(code *gojq.Code, page interface{}, meta map[string]interface{}) (interface{}, error) {
	iter := code.Run(page, append([]interface{}{meta}, ps.p.vars...)...)
	v, ok := iter.Next()
	if !ok {
		return nil, nil
	}
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v, nil
}

// sameOrigin reports whether a and b share scheme, host and port.
func sameOrigin(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host)
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
