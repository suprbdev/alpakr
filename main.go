package main

import "github.com/suprbdev/alpakr/cmd"

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
