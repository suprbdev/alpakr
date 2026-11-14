package main

import (
	"runtime/debug"

	"github.com/suprbdev/alpakr/cmd"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd.SetVersion(resolveVersion())
	cmd.Execute()
}

// resolveVersion returns the stamped version, falling back to the module
// version recorded in the binary (set by `go install module@version`).
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
