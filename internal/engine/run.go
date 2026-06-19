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

	out := make(map[string]interface{}, len(h.Fields))
	for fieldName, f := range h.Fields {
		var val interface{}
		var err error

		if f.Handler != "" {
			subInput := data
			if f.Input != "" {
				subInput, err = e.evalExpr(f.Input, data)
				if err != nil {
					return nil, fmt.Errorf("handler %q field %q sub-input: %w", handlerName, fieldName, err)
				}
			}
			val, err = e.Run(f.Handler, subInput)
			if err != nil {
				return nil, fmt.Errorf("handler %q field %q: %w", handlerName, fieldName, err)
			}
		} else {
			key := handlerName + "." + fieldName
			code, ok := e.codes.fields[key]
			if !ok {
				return nil, fmt.Errorf("handler %q field %q: missing compiled code", handlerName, fieldName)
			}
			val, err = runFirst(code, data)
			if err != nil {
				return nil, fmt.Errorf("handler %q field %q: %w", handlerName, fieldName, err)
			}
		}

		out[fieldName] = val
	}
	return out, nil
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
