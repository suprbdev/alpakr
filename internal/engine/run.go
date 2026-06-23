package engine

import (
	"alpakr/internal/config"
	"fmt"

	"github.com/itchyny/gojq"
)

// Run processes data through the named handler and returns the result.
// Returns nil when the record is filtered out.
func (e *Engine) Run(handlerName string, data interface{}) (interface{}, error) {
	h, ok := e.cfg.Handlers[handlerName]
	if !ok {
		return nil, fmt.Errorf("handler %q not found", handlerName)
	}

	// Apply input selector
	if code, ok := e.codes.input[handlerName]; ok {
		var err error
		data, err = runFirst(code, data)
		if err != nil {
			return nil, fmt.Errorf("handler %q input: %w", handlerName, err)
		}
	}

	if h.Each {
		items, ok := data.([]interface{})
		if !ok {
			return nil, fmt.Errorf("handler %q: 'each: true' requires array input, got %T", handlerName, data)
		}
		results := make([]interface{}, 0, len(items))
		for _, item := range items {
			out, err := e.runOne(handlerName, h, item)
			if err != nil {
				return nil, err
			}
			if out != nil {
				results = append(results, out)
			}
		}
		return results, nil
	}

	return e.runOne(handlerName, h, data)
}

func (e *Engine) runOne(handlerName string, h config.HandlerConfig, data interface{}) (interface{}, error) {
	// Apply filter — nil return signals "skip this record"
	if code, ok := e.codes.filter[handlerName]; ok {
		pass, err := runFirst(code, data)
		if err != nil {
			return nil, fmt.Errorf("handler %q filter: %w", handlerName, err)
		}
		if !isTruthy(pass) {
			return nil, nil
		}
	}

	return e.buildObject(handlerName, h.Fields, data)
}

// buildObject evaluates a map of FieldConfigs against data and returns the output map.
// path is a dot-joined string used for error messages and compiled code lookup.
func (e *Engine) buildObject(path string, fields map[string]config.FieldConfig, data interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(fields))
	for fieldName, f := range fields {
		val, err := e.evalField(path+"."+fieldName, f, data)
		if err != nil {
			return nil, err
		}
		out[fieldName] = val
	}
	return out, nil
}

func (e *Engine) evalField(path string, f config.FieldConfig, data interface{}) (interface{}, error) {
	// Sub-handler reference
	if f.Handler != "" {
		subInput := data
		if f.Input != "" {
			var err error
			subInput, err = e.evalExpr(f.Input, data)
			if err != nil {
				return nil, fmt.Errorf("%s sub-input: %w", path, err)
			}
		}
		val, err := e.Run(f.Handler, subInput)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return val, nil
	}

	// Inline nested fields
	if f.Fields != nil {
		subData := data
		if f.Input != "" {
			var err error
			subData, err = e.evalExpr(f.Input, data)
			if err != nil {
				return nil, fmt.Errorf("%s input: %w", path, err)
			}
		}
		return e.buildObject(path, f.Fields, subData)
	}

	// jq expression — look up pre-compiled code
	code, ok := e.codes.fields[path]
	if !ok {
		return nil, fmt.Errorf("%s: missing compiled code", path)
	}
	val, err := runFirst(code, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return val, nil
}

// evalExpr compiles and runs a one-off jq expression (used for sub-handler input selectors).
func (e *Engine) evalExpr(expr string, data interface{}) (interface{}, error) {
	code, err := compileExpr(expr, e.fnOpts)
	if err != nil {
		return nil, err
	}
	return runFirst(code, data)
}

func runFirst(code *gojq.Code, v interface{}) (interface{}, error) {
	iter := code.Run(v)
	val, ok := iter.Next()
	if !ok {
		return nil, nil
	}
	if err, ok := val.(error); ok {
		return nil, err
	}
	return val, nil
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
