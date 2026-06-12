package design

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactVisualPath_Empty(t *testing.T) {
	if got := RedactVisualPath("/root", ""); got != "" {
		t.Errorf("empty path = %q, want empty", got)
	}
}

func TestRedactVisualPath_Relative(t *testing.T) {
	// Non-absolute path is slash-normalised as-is.
	got := RedactVisualPath("/root", "screenshots/home.png")
	if got != "screenshots/home.png" {
		t.Errorf("relative path = %q", got)
	}
}

func TestRedactVisualPath_InsideRoot(t *testing.T) {
	root := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var) so the path is consistent.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	abs := filepath.Join(resolvedRoot, "screenshots", "home.png")
	got := RedactVisualPath(resolvedRoot, abs)
	if got != "screenshots/home.png" {
		t.Errorf("inside-root path = %q, want screenshots/home.png", got)
	}
}

func TestRedactVisualPath_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	// A path clearly outside the root must be hash-prefixed.
	got := RedactVisualPath(root, "/tmp/other/secret.png")
	if !strings.HasPrefix(got, "external:") {
		t.Errorf("outside-root path = %q, want external: prefix", got)
	}
}

func TestWriteVisualGateReport(t *testing.T) {
	root := t.TempDir()
	report := VisualGateReport{Version: 1, Verdict: "PASS"}
	path, err := WriteVisualGateReport(root, report)
	if err != nil {
		t.Fatalf("WriteVisualGateReport: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(path), ".autopus/design/verify/latest.json") {
		t.Errorf("report path = %q", path)
	}
}

func TestMatchesGlob(t *testing.T) {
	cases := []struct {
		path, glob string
		want       bool
	}{
		{"src/tokens/colors.ts", "src/tokens/*", true},
		{"colors.ts", "*.ts", true}, // base name match
		{"deep/colors.ts", "deep/*", true},
		{"foo/bar.ts", "foo/baz*", false},
		{"any", "", false},                        // empty glob
		{"src/theme/base.ts", "src/theme/", true}, // prefix match via TrimSuffix("*")
	}
	for _, tc := range cases {
		got := matchesGlob(tc.path, tc.glob)
		if got != tc.want {
			t.Errorf("matchesGlob(%q, %q) = %v, want %v", tc.path, tc.glob, got, tc.want)
		}
	}
}

func TestRelPath(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolved = root
	}
	abs := filepath.Join(resolved, "src", "tokens.ts")
	got := relPath(resolved, abs)
	if got != "src/tokens.ts" {
		t.Errorf("relPath = %q, want src/tokens.ts", got)
	}
}

func TestIsInsideRoot(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolved = root
	}
	inside := filepath.Join(resolved, "a", "b")
	if !isInsideRoot(resolved, inside) {
		t.Error("sub-path must be inside root")
	}
	if !isInsideRoot(resolved, resolved) {
		t.Error("root itself must be inside root")
	}
	if isInsideRoot(resolved, filepath.Dir(resolved)) {
		t.Error("parent must not be inside root")
	}
}

func TestAddPackRef_AllKinds(t *testing.T) {
	pack := &Pack{}
	addPackRef("src/tokens/colors.ts", 10, pack)
	if len(pack.TokenRefs) != 1 {
		t.Errorf("token ref count = %d", len(pack.TokenRefs))
	}
	addPackRef("src/components/ui/Button.tsx", 10, pack)
	if len(pack.ComponentRefs) != 1 {
		t.Errorf("component ref count = %d", len(pack.ComponentRefs))
	}
	addPackRef("e2e/snapshots/home-screenshot.png", 10, pack)
	if len(pack.ScreenshotRefs) != 1 {
		t.Errorf("screenshot ref count = %d", len(pack.ScreenshotRefs))
	}
	// source_of_truth reason is set.
	if pack.TokenRefs[0].Reason != "source_of_truth" {
		t.Errorf("token reason = %q", pack.TokenRefs[0].Reason)
	}
}

func TestCollectDeclaredSourceRefs_FromDesignMD(t *testing.T) {
	root := t.TempDir()
	// Write a DESIGN.md with a source_of_truth entry pointing to a real token file.
	writeFile(t, root, "DESIGN.md", `---
source_of_truth:
  - src/tokens/colors.ts
---
# Design
`)
	writeFile(t, root, "src/tokens/colors.ts", "export const c = '#000'")

	pack := &Pack{}
	err := collectDeclaredSourceRefs(root, 10, pack)
	if err != nil {
		t.Fatalf("collectDeclaredSourceRefs: %v", err)
	}
	if len(pack.TokenRefs) == 0 {
		t.Error("expected token ref from source_of_truth, got none")
	}
}
