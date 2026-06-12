package run

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// TestFirstArg covers the firstArg helper for empty and populated slices.
func TestFirstArg(t *testing.T) {
	if got := firstArg(nil); got != "" {
		t.Fatalf("firstArg(nil) = %q, want empty", got)
	}
	if got := firstArg([]string{"a", "b"}); got != "a" {
		t.Fatalf("firstArg([a b]) = %q, want a", got)
	}
}

// TestFirstLane covers firstLane for empty and populated lanes.
func TestFirstLane(t *testing.T) {
	if got := firstLane(journey.Pack{}); got != "" {
		t.Fatalf("firstLane(empty) = %q, want empty", got)
	}
	if got := firstLane(journey.Pack{Lanes: []string{"golden", "smoke"}}); got != "golden" {
		t.Fatalf("firstLane = %q, want golden", got)
	}
}

// TestEmptyArtifactValue covers each type branch of emptyArtifactValue.
func TestEmptyArtifactValue(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want bool
	}{
		{"nil", nil, true},
		{"blank string", "   ", true},
		{"nonblank string", "x", false},
		{"empty slice", []any{}, true},
		{"nonempty slice", []any{1}, false},
		{"empty map", map[string]any{}, true},
		{"nonempty map", map[string]any{"k": 1}, false},
		{"other type", 42, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := emptyArtifactValue(c.val); got != c.want {
				t.Fatalf("emptyArtifactValue(%v) = %v, want %v", c.val, got, c.want)
			}
		})
	}
}

// TestGUIScreenLabel covers the ID, Path, and fallback branches.
func TestGUIScreenLabel(t *testing.T) {
	if got := guiScreenLabel(journey.GUIScreenMatrixRow{ID: "home"}); got != "home" {
		t.Fatalf("label with ID = %q, want home", got)
	}
	if got := guiScreenLabel(journey.GUIScreenMatrixRow{Path: "/settings"}); got != "/settings" {
		t.Fatalf("label with Path = %q, want /settings", got)
	}
	if got := guiScreenLabel(journey.GUIScreenMatrixRow{}); got != "screen" {
		t.Fatalf("label fallback = %q, want screen", got)
	}
}

// TestAppendNodeRequire covers both the empty-existing and append branches.
func TestAppendNodeRequire(t *testing.T) {
	if got := appendNodeRequire("", "/g.js"); got != "--require=/g.js" {
		t.Fatalf("empty existing = %q", got)
	}
	if got := appendNodeRequire("--foo", "/g.js"); got != "--foo --require=/g.js" {
		t.Fatalf("append = %q", got)
	}
}

// TestDefaultPlaywrightBrowsersPath covers the per-OS path computation for the
// current platform.
func TestDefaultPlaywrightBrowsersPath(t *testing.T) {
	got := defaultPlaywrightBrowsersPath("/home/u")
	var want string
	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join("/home/u", "Library", "Caches", "ms-playwright")
	case "windows":
		want = filepath.Join("/home/u", "AppData", "Local", "ms-playwright")
	default:
		want = filepath.Join("/home/u", ".cache", "ms-playwright")
	}
	if got != want {
		t.Fatalf("defaultPlaywrightBrowsersPath = %q, want %q", got, want)
	}
}

// TestAppendDefaultEnv covers the env-set fallback branch (unset path uses the
// fallback value).
func TestAppendDefaultEnv(t *testing.T) {
	// Use a name that is virtually never set to exercise the fallback branch.
	env := appendDefaultEnv(nil, "AUTOPUS_QA_RUN_FAKE_VAR_XYZ", "fallback-val")
	if len(env) != 1 || env[0] != "AUTOPUS_QA_RUN_FAKE_VAR_XYZ=fallback-val" {
		t.Fatalf("appendDefaultEnv fallback = %v", env)
	}

	t.Setenv("AUTOPUS_QA_RUN_FAKE_VAR_XYZ", "real-val")
	env = appendDefaultEnv(nil, "AUTOPUS_QA_RUN_FAKE_VAR_XYZ", "fallback-val")
	if len(env) != 1 || env[0] != "AUTOPUS_QA_RUN_FAKE_VAR_XYZ=real-val" {
		t.Fatalf("appendDefaultEnv set = %v", env)
	}
}
