package cmd

import (
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	SetVersion("v1.2.3-test")
	defer func() { rootCmd.Version = "" }()

	out := runWithArgs(t, "--version")

	if !strings.Contains(out, "v1.2.3-test") {
		t.Fatalf("expected version output to contain %q, got: %s", "v1.2.3-test", out)
	}
}
