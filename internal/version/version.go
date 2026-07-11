// Package version exposes the Crabby version as a single value.
//
// The canonical source of truth is the repository's VERSION file. Release
// builds (see the Makefile and the release workflow) inject its contents at
// link time:
//
//	go build -ldflags "-X github.com/marioolf/crabby/internal/version.Version=$(cat VERSION)"
//
// For a plain `go build` we fall back to the module version recorded in the
// build info, and finally to "dev".
package version

import (
	"runtime/debug"
	"strings"
)

// Application identity, shared across the CLI and TUI so Crabby presents itself
// consistently everywhere.
const (
	App     = "Crabby"
	Author  = "marioolf"
	Repo    = "github.com/marioolf/crabby"
	Tagline = "Many Claudes. One shell."
)

// Version is populated at build time via -ldflags. Leave it empty here.
var Version = ""

// String returns the version number without a leading "v".
func String() string {
	v := Version
	if v == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if mv := info.Main.Version; mv != "" && mv != "(devel)" {
				v = mv
			}
		}
	}
	if v == "" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}
