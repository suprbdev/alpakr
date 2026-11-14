package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func resolveConfigPath() string {
	for _, name := range []string{"alpakr.yaml", "alpakr.yml"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return "alpakr.yaml" // fallback so error message is meaningful
}

var (
	configPath   string
	outputFormat string
	outputFile   string
)

var rootCmd = &cobra.Command{
	Use:   "alpakr",
	Short: "YAML-configured data transformation tool",
	Long: `alpakr reads data from a file, URL or stdin, applies YAML-configured
handler pipelines to transform and restructure it, and outputs
JSON or YAML to stdout or a file.`,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true
	},
}

// SetVersion wires the build-time version into the root command's --version flag.
func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", resolveConfigPath(), "path to config file (default: alpakr.yaml or alpakr.yml in current directory)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "", "output format override (json|yaml|ndjson)")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file path (default: stdout)")
}
