package main

import "testing"

func TestResolveVersion_Stamped(t *testing.T) {
	old := version
	defer func() { version = old }()

	version = "v1.2.3"
	if got := resolveVersion(); got != "v1.2.3" {
		t.Errorf("resolveVersion() = %q, want v1.2.3", got)
	}
}

func TestResolveVersion_Fallback(t *testing.T) {
	old := version
	defer func() { version = old }()

	version = "dev"
	if got := resolveVersion(); got == "" || got == "(devel)" {
		t.Errorf("resolveVersion() = %q, want dev or a module version", got)
	}
}
