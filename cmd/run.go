package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suprbdev/alpakr/internal/config"
	"github.com/suprbdev/alpakr/internal/engine"
	"github.com/suprbdev/alpakr/internal/output"
	"github.com/suprbdev/alpakr/internal/source"
)

var handlerName string
var limitRecords int

var errLimitReached = errors.New("limit reached")

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

func runRun(_ *cobra.Command, _ []string) error {
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
	if err := output.CheckFormat(cfg.Output.Format); err != nil {
		return err
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

	eng, err := engine.New(cfg)
	if err != nil {
		return err
	}

	srcCfg := cfg.SourceFor(handlerName)
	hasSource := srcCfg.Path != "" || srcCfg.URL != ""

	if !hasSource && !stdinIsPipe() {
		return fmt.Errorf("no source configured for handler %q and no data piped via stdin", handlerName)
	}

	if srcCfg.Format == "ndjson" {
		return runStream(cfg, eng, srcCfg)
	}
	return runBatch(cfg, eng, srcCfg, hasSource)
}

// runStream processes an NDJSON source one record at a time, writing each
// result as soon as it is produced.
func runStream(cfg *config.Config, eng *engine.Engine, srcCfg config.SourceConfig) (err error) {
	in, err := openStream(srcCfg)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	sink, err := openSink(cfg.Output.File)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := sink.Close(); err == nil {
			err = cerr
		}
	}()
	w, err := output.New(cfg.Output.Format, *cfg.Output.Indent, sink)
	if err != nil {
		return err
	}

	ndjson := &source.NdjsonSource{Reader: &flushingReader{r: in, w: sink.buf}}
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
		return w.WriteOne(result)
	})
	if errors.Is(err, errLimitReached) {
		return nil
	}
	return err
}

// openStream returns the raw NDJSON input: piped stdin, the configured URL's
// response body, or the configured file.
func openStream(srcCfg config.SourceConfig) (io.ReadCloser, error) {
	switch {
	case stdinIsPipe():
		return io.NopCloser(os.Stdin), nil
	case srcCfg.URL != "":
		return (&source.URLSource{URL: srcCfg.URL, Method: srcCfg.Method, Headers: srcCfg.Headers, Body: srcCfg.Body}).Open()
	}
	f, err := os.Open(srcCfg.Path)
	if err != nil {
		return nil, fmt.Errorf("opening ndjson file: %w", err)
	}
	return f, nil
}

// runBatch loads the full source, runs the handler once and writes the result.
func runBatch(cfg *config.Config, eng *engine.Engine, srcCfg config.SourceConfig, hasSource bool) (err error) {
	var src source.Source
	if stdinIsPipe() {
		// Pass explicit format only when a source was configured; otherwise auto-detect.
		stdinFormat := srcCfg.Format
		if !hasSource {
			stdinFormat = ""
		}
		src = &source.StdinSource{Format: stdinFormat}
	} else {
		src = buildSource(srcCfg)
	}
	data, err := src.Load()
	if err != nil {
		return err
	}
	result, err := eng.RunLimit(handlerName, data, limitRecords)
	if err != nil {
		return err
	}

	sink, err := openSink(cfg.Output.File)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := sink.Close(); err == nil {
			err = cerr
		}
	}()
	w, err := output.New(cfg.Output.Format, *cfg.Output.Indent, sink)
	if err != nil {
		return err
	}
	return w.Write(result)
}

// sink is a buffered output destination: stdout or a created file.
type sink struct {
	buf  *bufio.Writer
	file *os.File
}

// openSink opens path for writing, creating parent directories, or stdout when
// path is empty. Close must be called to flush buffered output.
func openSink(path string) (*sink, error) {
	if path == "" {
		return &sink{buf: bufio.NewWriterSize(os.Stdout, 64*1024)}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	return &sink{buf: bufio.NewWriterSize(f, 64*1024), file: f}, nil
}

func (s *sink) Write(p []byte) (int, error) { return s.buf.Write(p) }

// Close flushes buffered output and closes the file, if any.
func (s *sink) Close() error {
	err := s.buf.Flush()
	if s.file != nil {
		if cerr := s.file.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// flushingReader flushes w before every read from r. Streaming output is
// therefore batched while input is plentiful, yet never held back while the
// process waits on a slow or live input such as `tail -f`.
type flushingReader struct {
	r io.Reader
	w *bufio.Writer
}

func (f *flushingReader) Read(p []byte) (int, error) {
	if err := f.w.Flush(); err != nil {
		return 0, fmt.Errorf("writing output: %w", err)
	}
	return f.r.Read(p)
}

func stdinIsPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func buildSource(s config.SourceConfig) source.Source {
	if s.URL != "" {
		return &source.URLSource{URL: s.URL, Format: s.Format, Method: s.Method, Headers: s.Headers, Body: s.Body}
	}
	return &source.FileSource{Path: s.Path, Format: s.Format}
}

func handlerNames(cfg *config.Config) string {
	names := make([]string, 0, len(cfg.Handlers))
	for name := range cfg.Handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
