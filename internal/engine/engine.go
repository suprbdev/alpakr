package engine

import (
	"github.com/suprbdev/alpakr/internal/config"
)

// Engine runs compiled handler pipelines. It is safe for sequential reuse
// across many records.
type Engine struct {
	handlers map[string]*compiledHandler
	workers  int
}

// Option configures an Engine.
type Option func(*options)

type options struct {
	vars    map[string]interface{}
	workers int
}

// WithWorkers sets how many goroutines an entry handler with 'each: true'
// uses for its records (values below 2 mean sequential). Output order and
// error reporting are the same as sequential processing.
func WithWorkers(n int) Option {
	return func(o *options) { o.workers = n }
}

// WithVars makes each value available to every jq expression as $name.
func WithVars(vars map[string]interface{}) Option {
	return func(o *options) { o.vars = vars }
}

// New compiles every handler in cfg. It fails if any jq expression is invalid
// (including references to undefined variables) or any field references an
// undefined handler.
func New(cfg *config.Config, opts ...Option) (*Engine, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	handlers, err := compileHandlers(cfg.Handlers, newCompiler(o.vars))
	if err != nil {
		return nil, err
	}
	return &Engine{handlers: handlers, workers: o.workers}, nil
}
