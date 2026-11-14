package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/suprbdev/alpakr/internal/config"
	"github.com/suprbdev/alpakr/internal/engine"
	"github.com/suprbdev/alpakr/internal/source"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config and compile all expressions",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	vars, err := buildVars(cfg)
	if err != nil {
		return err
	}
	if _, err := engine.New(cfg, engine.WithVars(vars)); err != nil {
		return err
	}
	sources := map[string]config.SourceConfig{"source": cfg.Source}
	for name, src := range cfg.Sources {
		sources["sources."+name] = src
	}
	for label, src := range sources {
		if p := src.Pagination; p != nil {
			if _, err := source.CompilePagination(p.Next, p.Records, vars); err != nil {
				return fmt.Errorf("%s: %w", label, err)
			}
		}
	}

	fmt.Printf("OK — %d handler(s) compiled successfully\n", len(cfg.Handlers))
	return nil
}
