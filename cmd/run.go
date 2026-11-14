package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suprbdev/alpakr/internal/config"
	"github.com/suprbdev/alpakr/internal/engine"
	"github.com/suprbdev/alpakr/internal/output"
	"github.com/suprbdev/alpakr/internal/source"
)

var (
	handlerName  string
	limitRecords int
	readStdin    bool
	workers      int
)

var errLimitReached = errors.New("limit reached")

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the transformation pipeline",
	RunE:  runRun,
}

func init() {
	runCmd.Flags().StringVar(&handlerName, "handler", "", "entry-point handler name (defaults to 'root' if it exists)")
	runCmd.Flags().IntVar(&limitRecords, "limit", 0, "maximum number of records to process (0 = no limit)")
	runCmd.Flags().BoolVar(&readStdin, "stdin", false, "read input from stdin instead of the configured source")
	runCmd.Flags().IntVar(&workers, "workers", 1, "records to process in parallel (0 = number of CPUs); output order is kept")
	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// CLI flags override config
	if outputFormat != "" {
		cfg.Output.Format = strings.ToLower(outputFormat)
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

	vars, err := buildVars(cfg)
	if err != nil {
		return err
	}
	if workers < 0 {
		return fmt.Errorf("--workers must be >= 0")
	}
	if workers == 0 {
		workers = runtime.NumCPU()
	}
	eng, err := engine.New(cfg, engine.WithVars(vars), engine.WithWorkers(workers))
	if err != nil {
		return err
	}

	srcCfg := cfg.SourceFor(handlerName)
	hasSource := srcCfg.Path != "" || srcCfg.URL != ""
	fromStdin := readStdin || (!hasSource && stdinIsPipe())
	if !fromStdin && !hasSource {
		return fmt.Errorf("no source configured for handler %q; pipe data via stdin or pass --stdin", handlerName)
	}
	format := srcCfg.Format
	if fromStdin && srcCfg.FormatDetected && format != "csv" {
		format = ""
	}

	live := &liveOutput{}
	if srcCfg.Pagination != nil && !fromStdin {
		pages, err := newPages(srcCfg, vars)
		if err != nil {
			return err
		}
		pages.BeforeFetch = live.BeforeInput
		pages.Warn = func(msg string) { fmt.Fprintln(os.Stderr, "Warning:", msg) }
		return process(cfg, eng, pages, true, live, nil)
	}

	in, err := openInput(srcCfg, fromStdin)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	opts := []source.Option{source.CSVDelimiter(srcCfg.DelimiterRune()), source.CSVHeader(srcCfg.HasHeader())}
	vals, err := source.NewValues(&flushingReader{r: in, out: live}, format, opts...)
	if err != nil {
		return err
	}
	return process(cfg, eng, vals, format == "ndjson" || format == "csv", live, inputInfo(srcCfg, fromStdin))
}

func urlSource(srcCfg config.SourceConfig) source.URLSource {
	return source.URLSource{
		URL:     srcCfg.URL,
		Method:  srcCfg.Method,
		Headers: srcCfg.Headers,
		Body:    srcCfg.Body,
		Timeout: srcCfg.TimeoutDuration(),
	}
}

func newPages(srcCfg config.SourceConfig, vars map[string]interface{}) (*source.Pages, error) {
	p := srcCfg.Pagination
	compiled, err := source.CompilePagination(p.Next, p.Records, vars)
	if err != nil {
		return nil, err
	}
	return source.NewPages(urlSource(srcCfg), srcCfg.Format, compiled, p.MaxPagesLimit()), nil
}

// openInput opens stdin, the configured URL's response body or the
// configured file.
func openInput(srcCfg config.SourceConfig, fromStdin bool) (io.ReadCloser, error) {
	switch {
	case fromStdin:
		return io.NopCloser(os.Stdin), nil
	case srcCfg.URL != "":
		u := urlSource(srcCfg)
		return u.Open()
	}
	f, err := os.Open(srcCfg.Path)
	if err != nil {
		return nil, fmt.Errorf("opening source: %w", err)
	}
	return f, nil
}

// inputInfo returns the file behind the input, or nil for URLs and
// non-file stdin.
func inputInfo(srcCfg config.SourceConfig, fromStdin bool) os.FileInfo {
	var fi os.FileInfo
	var err error
	switch {
	case fromStdin:
		fi, err = os.Stdin.Stat()
	case srcCfg.Path != "":
		fi, err = os.Stat(srcCfg.Path)
	}
	if err != nil {
		return nil
	}
	return fi
}

// process runs the handler over vals. A single JSON value or YAML document is
// transformed as a whole and written as one document. When stream is set
// (NDJSON, CSV, paginated sources), or the input holds several values or
// documents, each value is a record whose result is written as soon as it
// is produced.
func process(cfg *config.Config, eng *engine.Engine, vals source.Values, stream bool, live *liveOutput, inInfo os.FileInfo) (err error) {
	first, ok, err := vals.Next()
	if err != nil {
		return err
	}
	if !ok {
		return writeOutput(cfg, nil, func(output.Writer, func() error) error { return nil })
	}

	var second interface{}
	hasSecond := false
	if !stream {
		if second, hasSecond, err = vals.Next(); err != nil {
			return err
		}
		if !hasSecond {
			result, err := eng.RunLimit(handlerName, first, limitRecords)
			if err != nil {
				return err
			}
			return writeOutput(cfg, nil, func(w output.Writer, _ func() error) error { return w.Write(result) })
		}
	}

	if err := checkNotInput(inInfo, cfg.Output.File); err != nil {
		return err
	}
	pending := []interface{}{first}
	if hasSecond {
		pending = append(pending, second)
	}
	if workers > 1 {
		return writeOutput(cfg, nil, func(w output.Writer, flush func() error) error {
			return streamParallel(eng, handlerName, limitRecords, w, flush, live, pending, vals, workers)
		})
	}
	return writeOutput(cfg, live, func(w output.Writer, _ func() error) error {
		written := 0
		emit := func(record interface{}) error {
			remaining := 0
			if limitRecords > 0 {
				remaining = limitRecords - written
			}
			result, err := eng.RunLimit(handlerName, record, remaining)
			if err != nil || result == nil {
				return err
			}
			if err := w.WriteOne(result); err != nil {
				return err
			}
			if a, ok := result.([]interface{}); ok {
				written += len(a)
			} else {
				written++
			}
			if limitRecords > 0 && written >= limitRecords {
				return errLimitReached
			}
			return nil
		}
		var err error
		for _, v := range pending {
			if err = emit(v); err != nil {
				break
			}
		}
		for err == nil {
			var v interface{}
			if v, ok, err = vals.Next(); err != nil || !ok {
				break
			}
			err = emit(v)
		}
		if errors.Is(err, errLimitReached) {
			return nil
		}
		return err
	})
}

// writeOutput opens the configured output, runs write against it and
// flushes. When live is non-nil it is pointed at the output for the
// duration, so streamed results are flushed while waiting on slow input.
func writeOutput(cfg *config.Config, live *liveOutput, write func(w output.Writer, flush func() error) error) (err error) {
	sink, err := openSink(cfg.Output.File)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := sink.Close(); err == nil {
			err = cerr
		}
	}()
	if live != nil {
		live.w = sink.buf
		defer func() { live.w = nil }()
	}
	w, err := output.New(cfg.Output.Format, *cfg.Output.Indent, sink, output.Delimiter(cfg.Output.DelimiterRune()))
	if err != nil {
		return err
	}
	return write(w, (&liveOutput{w: sink.buf}).Flush)
}

// checkNotInput refuses to stream into the file being read: creating the
// output would truncate the input before it is consumed.
func checkNotInput(inInfo os.FileInfo, outPath string) error {
	if inInfo == nil || outPath == "" {
		return nil
	}
	if outInfo, err := os.Stat(outPath); err == nil && os.SameFile(inInfo, outInfo) {
		return fmt.Errorf("output file %q is the input being streamed; write to a different file", outPath)
	}
	return nil
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

// liveOutput is the buffered output currently being streamed to, if any.
// BeforeInput runs before every input read or page fetch.
type liveOutput struct {
	w *bufio.Writer
	// beforeInput, when set, replaces the default flush in BeforeInput.
	beforeInput func() error
}

// BeforeInput is called before alpakr may block waiting for input: it
// flushes buffered output so results are not held back, or runs
// beforeInput if one is set.
func (l *liveOutput) BeforeInput() error {
	if l.beforeInput != nil {
		return l.beforeInput()
	}
	return l.Flush()
}

// Flush writes buffered output, if an output is open.
func (l *liveOutput) Flush() error {
	if l == nil || l.w == nil {
		return nil
	}
	if err := l.w.Flush(); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// flushingReader calls out.BeforeInput before every read from r. Streaming
// output is therefore batched while input is plentiful, yet never held back
// while the process waits on a slow or live input such as `tail -f`.
type flushingReader struct {
	r   io.Reader
	out *liveOutput
}

func (f *flushingReader) Read(p []byte) (int, error) {
	if err := f.out.BeforeInput(); err != nil {
		return 0, err
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

func handlerNames(cfg *config.Config) string {
	names := make([]string, 0, len(cfg.Handlers))
	for name := range cfg.Handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
