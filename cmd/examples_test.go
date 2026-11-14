package cmd

import (
	"bufio"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suprbdev/alpakr/internal/config"
)

func TestExamples_Run(t *testing.T) {
	configs, err := filepath.Glob(filepath.Join("..", "examples", "*", "alpakr.yaml"))
	if err != nil || len(configs) == 0 {
		t.Fatalf("no example configs found: %v", err)
	}
	for _, path := range configs {
		cfg, err := config.Load(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		entries := []string{"root"}
		if _, ok := cfg.Handlers["root"]; !ok {
			entries = nil
			for name, h := range cfg.Handlers {
				if h.Source != "" {
					entries = append(entries, name)
				}
			}
		}
		for _, h := range entries {
			t.Run(filepath.Base(filepath.Dir(path))+"/"+h, func(t *testing.T) {
				out := runWithArgs(t, "run", "-c", path, "--handler", h, "-f", "json")
				if strings.TrimSpace(out) == "" {
					t.Fatal("empty output")
				}
				sc := bufio.NewScanner(strings.NewReader(out))
				if f := cfg.SourceFor(h).Format; f != "ndjson" && f != "csv" {
					if !json.Valid([]byte(out)) {
						t.Fatalf("invalid JSON output:\n%s", out)
					}
					return
				}
				for sc.Scan() {
					if !json.Valid(sc.Bytes()) {
						t.Fatalf("invalid NDJSON line: %s", sc.Text())
					}
				}
			})
		}
	}
}

func TestExamples_ReferenceConfigValidates(t *testing.T) {
	runWithArgs(t, "validate", "-c", filepath.Join("..", "examples", "alpakr.yaml"))
}
