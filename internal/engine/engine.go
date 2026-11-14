package engine

import (
	"github.com/suprbdev/alpakr/internal/config"
)

// Engine runs compiled handler pipelines. It is safe for sequential reuse
// across many records.
type Engine struct {
	handlers map[string]*compiledHandler
}

// New compiles every handler in cfg. It fails if any jq expression is invalid
// or any field references an undefined handler.
func New(cfg *config.Config) (*Engine, error) {
	handlers, err := compileHandlers(cfg.Handlers, customFunctions())
	if err != nil {
		return nil, err
	}
	return &Engine{handlers: handlers}, nil
}
