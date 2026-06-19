package cmd

import (
	"alpakr/internal/config"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var listHandlersCmd = &cobra.Command{
	Use:   "list-handlers",
	Short: "List handler names defined in config",
	RunE:  runListHandlers,
}

func init() {
	rootCmd.AddCommand(listHandlersCmd)
}

func runListHandlers(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(cfg.Handlers))
	for name := range cfg.Handlers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
