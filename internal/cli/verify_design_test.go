package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/design"
)

func TestFilterUIChangedFiles_UsesSharedDesignDetector(t *testing.T) {
	t.Parallel()

	files := []string{
		"src/components/Button.tsx",
		"src/styles/app.css",
		"src/theme.ts",
		"docs/design-system/baseline.md",
		"internal/server.go",
	}

	got := filterUIChangedFiles(files, nil)
	assert.Equal(t, []string{
		"src/components/Button.tsx",
		"src/styles/app.css",
		"src/theme.ts",
		"docs/design-system/baseline.md",
	}, got)
}

func TestBuildVerifyDesignContextReport_FoundAndSkip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte("# Design\n\n## Components\nButtons use icons."), 0o644))

	report := buildVerifyDesignContextReport(dir, []string{"src/Button.tsx"}, design.Options{Enabled: true, MaxContextLines: 20})
	assert.Contains(t, report, "design context: DESIGN.md")
	assert.Contains(t, report, "Buttons use icons.")

	skip := buildVerifyDesignContextReport(t.TempDir(), []string{"internal/server.go"}, design.Options{Enabled: true, MaxContextLines: 20})
	assert.Contains(t, skip, "design context: skipped")
}

func TestBuildVerifyDesignContextReport_ConfiguredPathPrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte("# Root\n\n## Palette\nRoot colors."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.md"), []byte("# Custom\n\n## Palette\nCustom colors."), 0o644))

	report := buildVerifyDesignContextReport(dir, []string{"src/Button.tsx"}, design.Options{
		Enabled:         true,
		Paths:           []string{"custom.md"},
		MaxContextLines: 20,
	})
	assert.Contains(t, report, "design context: custom.md")
	assert.Contains(t, report, "Custom colors.")
	assert.NotContains(t, report, "Root colors.")
}

func TestBuildVerifyDesignContextReport_IncludesDiagnostics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte(`---
source_of_truth:
  - ../secret.md
  - docs/design-system/baseline.md
---
# Project Design
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "design-system"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "design-system", "baseline.md"), []byte("# Baseline\n\n## Typography\nUse clear hierarchy."), 0o644))

	report := buildVerifyDesignContextReport(dir, []string{"src/Button.tsx"}, design.Options{
		Enabled:         true,
		MaxContextLines: 20,
	})
	assert.Contains(t, report, "design context: DESIGN.md -> docs/design-system/baseline.md")
	assert.Contains(t, report, "Diagnostics:")
	assert.Contains(t, report, "parent_traversal")
}
