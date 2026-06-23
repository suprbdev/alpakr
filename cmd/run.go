package cmd

import (
	"alpakr/internal/config"
	"alpakr/internal/engine"
	"alpakr/internal/output"
	"alpakr/internal/source"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var handlerName string
var limitRecords int

var errLimitReached = fmt.Errorf("limit reached")

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the transformation pipeline",
	RunE:  runRun,
}

func init() {
	runCmd.Flags().StringVar(&handlerName, "handler", "", "entry-point handler name (defaults to 'root' if it exists)")
	runCmd.Flags().IntVar(&limitRecords, "limit", 0, "maximum number of records to process (0 = no limit)")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// CLI flags override config
	if outputFormat != "" {
		cfg.Output.Format = outputFormat
	}
	if outputFile != "" {
		cfg.Output.File = outputFile
	}

	// Resolve handler name
	if handlerName == "" {
		if _, ok := cfg.Handlers["root"]; ok {
			handlerName = "root"
		} else {
			return fmt.Errorf("no --handler specified and no 'root' handler in config; available: %s", handlerNames(cfg))
		}
	}
	if _, ok := cfg.Handlers[handlerName]; !ok {
		return fmt.Errorf("handler %q not found in config; available: %s", handlerName, handlerNames(cfg))
	}

	// Run engine
	eng, err := engine.New(cfg)
	if err != nil {
		return err
	}

	srcCfg := cfg.SourceFor(handlerName)

	// NDJSON: stream records one at a time to avoid loading full file into memory
	if srcCfg.Format == "ndjson" {
		var ndjson *source.NdjsonSource
		if stdinIsPipe() {
			ndjson = &source.NdjsonSource{}
		} else {
			f, openErr := os.Open(srcCfg.Path)
			if openErr != nil {
				return fmt.Errorf("opening ndjson file: %w", openErr)
			}
			defer f.Close()
			ndjson = &source.NdjsonSource{Reader: f}
		}
		w, werr := buildWriter(cfg)
		if werr != nil {
			return werr
		}
		sw, ok := w.(streamWriter)
		if !ok {
			return fmt.Errorf("output format does not support streaming")
		}
		written := 0
		err = ndjson.Stream(func(record interface{}) error {
			if limitRecords > 0 && written >= limitRecords {
				return errLimitReached
			}
			result, rerr := eng.Run(handlerName, record)
			if rerr != nil {
				return rerr
			}
			if result == nil {
				return nil // filtered out
			}
			written++
			return sw.WriteOne(result)
		})
		if err == errLimitReached {
			return nil
		}
		return err
	}

	// Non-NDJSON: load full source then run
	var src source.Source
	if stdinIsPipe() {
		src = &source.StdinSource{Format: srcCfg.Format}
	} else {
		src, err = buildSource(srcCfg)
		if err != nil {
			return err
		}
	}
	data, err := src.Load()
	if err != nil {
		return err
	}
	result, err := eng.Run(handlerName, data)
	if err != nil {
		return err
	}

	// Apply limit to array results
	if limitRecords > 0 {
		if arr, ok := result.([]interface{}); ok && len(arr) > limitRecords {
			result = arr[:limitRecords]
		}
	}

	// Write output
	w, err := buildWriter(cfg)
	if err != nil {
		return err
	}
	return w.Write(result)
}

type writer interface {
	Write(v interface{}) error
}

type streamWriter interface {
	WriteOne(v interface{}) error
}

func buildWriter(cfg *config.Config) (writer, error) {
	var out io.Writer = os.Stdout
	if cfg.Output.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Output.File), 0o755); err != nil {
			return nil, fmt.Errorf("creating output directory: %w", err)
		}
		f, err := os.Create(cfg.Output.File)
		if err != nil {
			return nil, fmt.Errorf("creating output file: %w", err)
		}
		out = f
	}

	switch cfg.Output.Format {
	case "yaml", "yml":
		return &output.YAMLWriter{Out: out}, nil
	default:
		return &output.JSONWriter{Indent: cfg.Output.Indent, Out: out}, nil
	}
}

func stdinIsPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func buildSource(s config.SourceConfig) (source.Source, error) {
	if s.URL != "" {
		return &source.URLSource{URL: s.URL, Format: s.Format, Method: s.Method, Headers: s.Headers, Body: s.Body}, nil
	}
	return &source.FileSource{Path: s.Path, Format: s.Format}, nil
}

func handlerNames(cfg *config.Config) string {
	names := make([]string, 0, len(cfg.Handlers))
	for name := range cfg.Handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
