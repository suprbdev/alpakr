package engine

import (
	"github.com/itchyny/gojq"
	"github.com/suprbdev/alpakr/internal/config"
)

type Engine struct {
	cfg    *config.Config
	codes  *compiledCodes
	fnOpts []gojq.CompilerOption
}

func New(cfg *config.Config) (*Engine, error) {
	fnOpts := customFunctions()
	codes, err := compileHandlers(cfg.Handlers, fnOpts)
	if err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg, codes: codes, fnOpts: fnOpts}, nil
}
