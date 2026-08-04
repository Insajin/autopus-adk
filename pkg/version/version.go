// Package version provides build version info injected via ldflags or Go module metadata.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func contains(s, substr string) bool { return strings.Contains(s, substr) }

// Injected at build time via ldflags. Empty string means not set.
var (
	version      string
	commit       string
	date         string
	sourceCommit string
	sourceTree   string
)

func init() {
	if version != "" {
		return
	}
	// Fallback: read from Go module build info (works with `go install`).
	info, ok := debug.ReadBuildInfo()
	if !ok {
		version = "unknown"
		return
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = strings.TrimPrefix(info.Main.Version, "v")
	} else {
		version = "dev"
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if sourceCommit == "" {
				sourceCommit = s.Value
			}
			if len(s.Value) > 7 {
				commit = s.Value[:7]
			} else {
				commit = s.Value
			}
		case "vcs.time":
			date = s.Value
		case "vcs.modified":
			// Only append -dirty if not already present (Go module version may include +dirty).
			if s.Value == "true" && !contains(version, "dirty") {
				version += "-dirty"
			}
		}
	}
	if commit == "" {
		commit = "none"
	}
	if date == "" {
		date = "unknown"
	}
}

// Version returns the build version.
func Version() string { return version }

// Commit returns the build commit hash.
func Commit() string { return commit }

// Date returns the build date.
func Date() string { return date }

// SourceCommit returns the immutable full source revision injected for signed
// candidate binding. It intentionally differs from the display-only Commit.
func SourceCommit() string { return sourceCommit }

// SourceTree returns the immutable full source tree injected for signed
// candidate binding. Development builds normally leave it empty.
func SourceTree() string { return sourceTree }

// String returns the full version string.
func String() string {
	return fmt.Sprintf("auto %s (commit: %s, built: %s)", version, commit, date)
}
