package engine

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/suprbdev/alpakr/internal/config"
)

// compiledHandler is a compiled config.HandlerConfig.
type compiledHandler struct {
	name   string
	each   bool
	input  *expr // nil = identity
	filter *expr // nil = keep every record
	obj    objectSpec
}

// objectSpec describes an output object: its fields in config order and
// their names, shared by every record.Object built from it.
type objectSpec struct {
	keys   []string
	fields []compiledField
}

// compiledField is a compiled config.FieldConfig. Exactly one of code, handler or
// obj is set.
type compiledField struct {
	name    string
	path    string // dotted location used in error messages, e.g. "root.loc.city"
	input   *expr  // narrows data for handler/fields; nil = identity
	code    *expr
	handler *compiledHandler
	obj     *objectSpec
}

// compileHandlers compiles every jq expression in handlers up front so that
// invalid expressions are reported before any data is processed.
func compileHandlers(handlers map[string]config.HandlerConfig, opts []gojq.CompilerOption) (map[string]*compiledHandler, error) {
	compiled := make(map[string]*compiledHandler, len(handlers))
	for name, h := range handlers {
		compiled[name] = &compiledHandler{name: name, each: h.Each}
	}

	for _, name := range sortedKeys(handlers) {
		h, ch := handlers[name], compiled[name]
		var err error
		if ch.input, err = compileInput(h.Input, opts); err != nil {
			return nil, fmt.Errorf("handler %q input: %w", name, err)
		}
		if h.Filter != "" {
			if ch.filter, err = compileExpr(h.Filter, opts); err != nil {
				return nil, fmt.Errorf("handler %q filter: %w", name, err)
			}
		}
		if ch.obj, err = compileFields(name, h.Fields, compiled, opts); err != nil {
			return nil, err
		}
	}
	return compiled, nil
}

func compileFields(path string, fields config.Fields, handlers map[string]*compiledHandler, opts []gojq.CompilerOption) (objectSpec, error) {
	spec := objectSpec{keys: make([]string, 0, len(fields)), fields: make([]compiledField, 0, len(fields))}
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		name := f.Name
		cf := compiledField{name: name, path: path + "." + name}
		if seen[name] {
			return spec, fmt.Errorf("field %q: defined more than once", cf.path)
		}
		seen[name] = true
		var err error
		switch {
		case f.Handler != "":
			if cf.handler = handlers[f.Handler]; cf.handler == nil {
				return spec, fmt.Errorf("field %q: undefined handler %q", cf.path, f.Handler)
			}
			if cf.input, err = compileInput(f.Input, opts); err != nil {
				return spec, fmt.Errorf("field %q input: %w", cf.path, err)
			}
		case f.Fields != nil:
			if cf.input, err = compileInput(f.Input, opts); err != nil {
				return spec, fmt.Errorf("field %q input: %w", cf.path, err)
			}
			nested, err := compileFields(cf.path, f.Fields, handlers, opts)
			if err != nil {
				return spec, err
			}
			cf.obj = &nested
		case f.Expr != "":
			if cf.code, err = compileExpr(f.Expr, opts); err != nil {
				return spec, fmt.Errorf("field %q: %w", cf.path, err)
			}
		default:
			return spec, fmt.Errorf("field %q: empty jq expression", cf.path)
		}
		spec.keys = append(spec.keys, name)
		spec.fields = append(spec.fields, cf)
	}
	return spec, nil
}

// expr is a compiled jq expression. Plain paths such as ".a.b" also keep
// their keys so they can be evaluated without running the jq interpreter.
type expr struct {
	code *gojq.Code
	path []string
}

var plainPath = regexp.MustCompile(`^(\.[A-Za-z_][A-Za-z0-9_]*)+$`)

// compileInput compiles an input selector, returning nil for the identity.
func compileInput(src string, opts []gojq.CompilerOption) (*expr, error) {
	if src == "" || src == "." {
		return nil, nil
	}
	return compileExpr(src, opts)
}

func compileExpr(src string, opts []gojq.CompilerOption) (*expr, error) {
	q, err := gojq.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", src, err)
	}
	code, err := gojq.Compile(q, opts...)
	if err != nil {
		return nil, fmt.Errorf("compiling jq expression %q: %w", src, err)
	}
	x := &expr{code: code}
	switch {
	case src == ".":
		x.path = []string{}
	case plainPath.MatchString(src):
		x.path = strings.Split(src[1:], ".")
	}
	return x, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
