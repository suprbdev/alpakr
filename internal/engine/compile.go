package engine

import (
	"alpakr/internal/config"
	"fmt"

	"github.com/itchyny/gojq"
)

type compiledCodes struct {
	input  map[string]*gojq.Code // keyed by handler name
	filter map[string]*gojq.Code // keyed by handler name
	fields map[string]*gojq.Code // keyed by "handlerName.fieldName"
}

func compileHandlers(handlers map[string]config.HandlerConfig, fnOpts []gojq.CompilerOption) (*compiledCodes, error) {
	cc := &compiledCodes{
		input:  make(map[string]*gojq.Code),
		filter: make(map[string]*gojq.Code),
		fields: make(map[string]*gojq.Code),
	}

	for name, h := range handlers {
		if h.Input != "" && h.Input != "." {
			code, err := compileExpr(h.Input, fnOpts)
			if err != nil {
				return nil, fmt.Errorf("handler %q input: %w", name, err)
			}
			cc.input[name] = code
		}

		if h.Filter != "" {
			code, err := compileExpr(h.Filter, fnOpts)
			if err != nil {
				return nil, fmt.Errorf("handler %q filter: %w", name, err)
			}
			cc.filter[name] = code
		}

		for fieldName, f := range h.Fields {
			if f.Expr == "" {
				continue
			}
			code, err := compileExpr(f.Expr, fnOpts)
			if err != nil {
				return nil, fmt.Errorf("handler %q field %q: %w", name, fieldName, err)
			}
			cc.fields[name+"."+fieldName] = code
		}
	}

	return cc, nil
}

func compileExpr(expr string, fnOpts []gojq.CompilerOption) (*gojq.Code, error) {
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", expr, err)
	}
	code, err := gojq.Compile(q, fnOpts...)
	if err != nil {
		return nil, fmt.Errorf("compiling jq expression %q: %w", expr, err)
	}
	return code, nil
}
