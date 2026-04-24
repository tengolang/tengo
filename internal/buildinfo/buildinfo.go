// Package buildinfo exposes version and VCS information stamped into the
// binary by the Go toolchain at build time.
//
// The version string can be overridden at link time for CI/release pipelines:
//
//	go build -ldflags "-X github.com/ganehag/tengo/v3/internal/buildinfo.version=v3.x.y"
package buildinfo

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// version may be set at link time to override the value read from build info.
var version string

// Version returns a human-readable build version string. Examples:
//
//	v3.1.0                                    tagged release, clean
//	v3.1.0 (dirty)                            tagged release, local modifications
//	v3.0.1-0.20260424075157-b249db11b830      untagged build, clean
//	dev (git: b249db1, dirty)                 go run / no VCS tag
func Version() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	v := info.Main.Version
	isDev := v == "" || v == "(devel)"

	// Collect VCS settings stamped in by the Go toolchain (requires go 1.18+).
	var commit, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "dirty"
			}
		}
	}

	if isDev {
		// go run or build outside a module tag — show git detail if available.
		switch {
		case commit != "" && dirty != "":
			return fmt.Sprintf("dev (git: %s, %s)", commit, dirty)
		case commit != "":
			return fmt.Sprintf("dev (git: %s)", commit)
		default:
			return "dev"
		}
	}

	// For proper module versions the Go toolchain appends "+dirty" to the
	// version string. Strip it; we report the flag ourselves for consistency.
	v = strings.TrimSuffix(v, "+dirty")

	if dirty != "" {
		return v + " (dirty)"
	}
	return v
}
