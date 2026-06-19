package cmd

import (
	"alpakr/internal/config"
	"alpakr/internal/engine"
	"fmt"

	"github.com/spf13/cobra"
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

	if _, err := engine.New(cfg); err != nil {
		return err
	}

	fmt.Printf("OK — %d handler(s) compiled successfully\n", len(cfg.Handlers))
	return nil
}
