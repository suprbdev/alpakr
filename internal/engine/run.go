package engine

import (
	"errors"
	"fmt"
)

// Run processes data through the named handler and returns the result.
// Returns nil when the record is filtered out.
func (e *Engine) Run(handlerName string, data interface{}) (interface{}, error) {
	h, ok := e.handlers[handlerName]
	if !ok {
		return nil, fmt.Errorf("handler %q not found", handlerName)
	}
	return h.run(data, 0, 0)
}

// RunLimit is like Run, but an 'each' handler stops once limit records have
// been produced. limit <= 0 means no limit.
func (e *Engine) RunLimit(handlerName string, data interface{}, limit int) (interface{}, error) {
	h, ok := e.handlers[handlerName]
	if !ok {
		return nil, fmt.Errorf("handler %q not found", handlerName)
	}
	return h.run(data, limit, 0)
}

// maxDepth bounds nested handler calls so a handler that references itself
// without a terminating input fails cleanly instead of overflowing the stack.
const maxDepth = 1000

// depthError is returned unwrapped through every level so the message stays
// short rather than repeating the field path maxDepth times.
type depthError struct {
	handler string
}

func (e *depthError) Error() string {
	return fmt.Sprintf("handler %q: nested more than %d levels deep (recursive handler reference?)", e.handler, maxDepth)
}

func (h *compiledHandler) run(data interface{}, limit, depth int) (interface{}, error) {
	if depth > maxDepth {
		return nil, &depthError{handler: h.name}
	}
	if h.input != nil {
		var err error
		data, err = h.input.eval(data)
		if err != nil {
			return nil, fmt.Errorf("handler %q input: %w", h.name, err)
		}
	}

	if !h.each {
		return h.runOne(data, depth)
	}
	items, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("handler %q: 'each: true' requires array input, got %T", h.name, data)
	}
	n := len(items)
	if limit > 0 && limit < n {
		n = limit
	}
	results := make([]interface{}, 0, n)
	for _, item := range items {
		out, err := h.runOne(item, depth)
		if err != nil {
			return nil, err
		}
		if out != nil {
			results = append(results, out)
			if len(results) == limit {
				break
			}
		}
	}
	return results, nil
}

// runOne applies the filter and builds the output object for a single record.
// It returns nil when the filter rejects the record.
func (h *compiledHandler) runOne(data interface{}, depth int) (interface{}, error) {
	if h.filter != nil {
		pass, err := h.filter.eval(data)
		if err != nil {
			return nil, fmt.Errorf("handler %q filter: %w", h.name, err)
		}
		if !isTruthy(pass) {
			return nil, nil
		}
	}
	return buildObject(h.fields, data, depth)
}

func buildObject(fields []compiledField, data interface{}, depth int) (interface{}, error) {
	out := make(map[string]interface{}, len(fields))
	for i := range fields {
		val, err := fields[i].eval(data, depth)
		if err != nil {
			return nil, err
		}
		out[fields[i].name] = val
	}
	return out, nil
}

func (f *compiledField) eval(data interface{}, depth int) (interface{}, error) {
	if f.code != nil {
		val, err := f.code.eval(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		return val, nil
	}

	if f.input != nil {
		var err error
		data, err = f.input.eval(data)
		if err != nil {
			if f.handler != nil {
				return nil, fmt.Errorf("%s sub-input: %w", f.path, err)
			}
			return nil, fmt.Errorf("%s input: %w", f.path, err)
		}
	}

	if f.handler != nil {
		val, err := f.handler.run(data, 0, depth+1)
		if err != nil {
			var de *depthError
			if errors.As(err, &de) {
				return nil, err
			}
			return nil, fmt.Errorf("%s: %w", f.path, err)
		}
		return val, nil
	}
	return buildObject(f.fields, data, depth)
}

// eval returns the first output of x for input v, or nil if it produces none.
func (x *expr) eval(v interface{}) (interface{}, error) {
	if x.path != nil {
		if val, ok := lookupPath(x.path, v); ok {
			return val, nil
		}
	}
	iter := x.code.Run(v)
	val, ok := iter.Next()
	if !ok {
		return nil, nil
	}
	if err, ok := val.(error); ok {
		return nil, err
	}
	return val, nil
}

// lookupPath follows keys through nested objects with jq semantics: missing
// keys and null yield null. It reports false for any other value, leaving jq
// to produce the type error.
func lookupPath(keys []string, v interface{}) (interface{}, bool) {
	for _, k := range keys {
		switch m := v.(type) {
		case nil:
			return nil, true
		case map[string]interface{}:
			v = m[k]
		default:
			return nil, false
		}
	}
	return v, true
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return true
}
