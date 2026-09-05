package cli

// Project-root resolution and the unresolved-root diagnostic, shared by the
// conditional dispatcher (`auto rules fire`) and the sticky dispatcher
// (`auto rules sticky`).
//
// Both commands derive their root from one upward walk, so what counts as
// evidence of a project — and what a walk that finds none is allowed to say —
// is pinned here once instead of twice.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveProjectRoot_RequiresMarkerAndStopsAtHome pins REQ-CONDRULE-FIRE-11
// root resolution. The bare working directory is an implicit redirection
// channel, so evidence is required; the directory markers are never accepted at
// the home directory even though `~/.claude` exists on most installs, and the
// regular harness config file is the one piece of evidence that is.
func TestResolveProjectRoot_RequiresMarkerAndStopsAtHome(t *testing.T) {
	tests := []struct {
		name        string
		markerDir   string // relative to home; "-" creates no marker at all
		markerName  string
		markerIsLnk bool   // ship the marker as a symlink, as a clone can
		markerIsDir bool   // create the marker as a directory
		startDir    string // relative to home
		wantDir     string // relative to home; empty means unresolved
	}{
		{name: "claude marker in the start directory", markerDir: "proj", markerName: ".claude", markerIsDir: true, startDir: "proj", wantDir: "proj"},
		{name: "git marker in an ancestor", markerDir: "proj", markerName: ".git", markerIsDir: true, startDir: "proj/pkg/sub", wantDir: "proj"},
		{name: "no marker anywhere", markerDir: "-", startDir: "work/sub"},
		{name: "home marker is not a project root", markerDir: ".", markerName: ".claude", markerIsDir: true, startDir: "work/sub"},
		// A symlinked marker is not evidence of a project root: accepting it
		// would anchor the conditional surface outside the checkout.
		{name: "symlinked claude marker is refused", markerDir: "proj", markerName: ".claude", markerIsDir: true, markerIsLnk: true, startDir: "proj"},
		// The config fallback: a project that has been configured but not yet
		// generated, and is not a git checkout, is still a project.
		{name: "config marker in an ancestor", markerDir: "proj", markerName: projectConfigMarker, startDir: "proj/pkg/sub", wantDir: "proj"},
		{name: "config marker at home is a project root", markerDir: ".", markerName: projectConfigMarker, startDir: "work/sub", wantDir: "."},
		{name: "symlinked config marker is refused", markerDir: "proj", markerName: projectConfigMarker, markerIsLnk: true, startDir: "proj"},
		{name: "directory named like the config is refused", markerDir: "proj", markerName: projectConfigMarker, markerIsDir: true, startDir: "proj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			start := filepath.Join(home, filepath.FromSlash(tt.startDir))
			require.NoError(t, os.MkdirAll(start, 0o755))
			if tt.markerDir != "-" {
				marker := filepath.Join(home, filepath.FromSlash(tt.markerDir), tt.markerName)
				require.NoError(t, os.MkdirAll(filepath.Dir(marker), 0o755))
				switch {
				case tt.markerIsLnk:
					outside := t.TempDir()
					if err := os.Symlink(outside, marker); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				case tt.markerIsDir:
					require.NoError(t, os.MkdirAll(marker, 0o755))
				default:
					require.NoError(t, os.WriteFile(marker, []byte("version: 1\n"), 0o644))
				}
			}

			got, ok := resolveProjectRoot(start)

			if tt.wantDir == "" {
				assert.False(t, ok, "resolution must fail open outside a project")
				assert.Empty(t, got)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, filepath.Join(home, filepath.FromSlash(tt.wantDir)), got)
		})
	}
}

// TestResolveStickyRoot_MarkerSetStaysNarrowAndTakesTheConfigFallback pins which
// evidence the sticky dispatcher accepts. REQ-STICKYRULE-FIRE-12 deliberately
// refuses a bare `.git` checkout, because a checkout with no generated harness
// has nothing installed to inject; the harness config is accepted because a
// configured project is a project whether or not it is a git checkout.
func TestResolveStickyRoot_MarkerSetStaysNarrowAndTakesTheConfigFallback(t *testing.T) {
	tests := []struct {
		name     string
		marker   string
		isDir    bool
		resolved bool
	}{
		{name: "claude directory resolves", marker: ".claude", isDir: true, resolved: true},
		{name: "git checkout alone stays unresolved", marker: ".git", isDir: true},
		{name: "harness config resolves", marker: projectConfigMarker, resolved: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			project := filepath.Join(home, "proj")
			start := filepath.Join(project, "pkg", "sub")
			require.NoError(t, os.MkdirAll(start, 0o755))
			if tt.isDir {
				require.NoError(t, os.MkdirAll(filepath.Join(project, tt.marker), 0o755))
			} else {
				require.NoError(t, os.WriteFile(filepath.Join(project, tt.marker),
					[]byte("version: 1\n"), 0o644))
			}

			got, ok := resolveStickyRoot(start)

			assert.Equal(t, tt.resolved, ok)
			if tt.resolved {
				assert.Equal(t, project, got)
				return
			}
			assert.Empty(t, got)
		})
	}
}

// TestReportUnresolvedRoot_HandlesEveryStreamShape pins the diagnostic format
// and the nil-stream guard for both dispatchers, which share one reporter.
//
// The guard is tested here rather than through fireStickyRules because that
// function recovers panics: a nil write target that panicked would be swallowed
// there and the missing guard would look identical to a working one.
func TestReportUnresolvedRoot_HandlesEveryStreamShape(t *testing.T) {
	lines := map[string]string{
		"sticky":            unresolvedRootLine,
		"conditional-rules": unresolvedConditionalRootLine,
	}

	for dispatcher, line := range lines {
		t.Run(dispatcher+" writes exactly one diagnostic line", func(t *testing.T) {
			var stderr bytes.Buffer

			reportUnresolvedRoot(&stderr, line)

			assert.Equal(t, dispatcher+" project_root_unresolved\n", stderr.String(),
				"REQ-STICKYRULE-FIRE-04: one newline-terminated line, naming no path")
		})
	}

	t.Run("a nil stream is a no-op, not a panic", func(t *testing.T) {
		assert.NotPanics(t, func() { reportUnresolvedRoot(nil, unresolvedRootLine) },
			"REQ-STICKYRULE-FIRE-03: a panic exits 2, which erases the user's prompt")
	})
}
