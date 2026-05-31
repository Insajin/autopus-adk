package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPackCollectsDesignSources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "# Design\n\n## Palette\nUse semantic color roles.\n\nFigma https://figma.com/design/abc/Product")
	writeFile(t, root, "src/tokens/colors.ts", "export const primary = '#000'")
	writeFile(t, root, "src/components/ui/Button.tsx", "export function Button() { return null }")
	writeFile(t, root, "e2e/example.spec.ts-snapshots/home-chromium-darwin.png", "png")
	writeFile(t, root, "src/components/Button.figma.tsx", "export default {}")

	pack, err := BuildPack(root, PackOptions{
		ContextOptions: Options{Enabled: true, MaxContextLines: 20},
		MaxRefs:        10,
	})
	require.NoError(t, err)
	assert.True(t, pack.DesignContext.Found)
	assert.NotEmpty(t, pack.TokenRefs)
	assert.NotEmpty(t, pack.ComponentRefs)
	assert.NotEmpty(t, pack.ScreenshotRefs)
	assert.NotEmpty(t, pack.FigmaRefs)
	assert.Equal(t, "detected", pack.CodeConnect.Status)
	assert.Contains(t, pack.Markdown(), "Design Source Pack")
}

func TestBuildPackCollectsDeclaredSourceOfTruthGlobs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", `---
source_of_truth:
  - packages/web/tokens/**
  - packages/web/src/components/ui/**
---
# Design

## Source Of Truth
Use the declared token and primitive sources.
`)
	writeFile(t, root, "packages/web/tokens/semantic/color.light.json", "{}")
	writeFile(t, root, "packages/web/src/components/ui/Button.tsx", "export function Button() { return null }")

	pack, err := BuildPack(root, PackOptions{
		ContextOptions: Options{Enabled: true, MaxContextLines: 20},
		MaxRefs:        10,
	})
	require.NoError(t, err)

	assert.Contains(t, pack.TokenRefs, SourceRef{
		Path:   "packages/web/tokens/semantic/color.light.json",
		Kind:   "token_or_theme",
		Reason: "source_of_truth",
	})
	assert.Contains(t, pack.ComponentRefs, SourceRef{
		Path:   "packages/web/src/components/ui/Button.tsx",
		Kind:   "component",
		Reason: "source_of_truth",
	})
	assert.NotContains(t, pack.SetupGaps, "token_refs_missing")
	assert.NotContains(t, pack.SetupGaps, "component_refs_missing")
}

func TestBuildPackReportsGapsWhenDesignMissing(t *testing.T) {
	t.Parallel()

	pack, err := BuildPack(t.TempDir(), PackOptions{ContextOptions: Options{Enabled: true}, MaxRefs: 10})
	require.NoError(t, err)
	assert.False(t, pack.DesignContext.Found)
	assert.Contains(t, pack.SetupGaps, "design_context_missing")
	assert.Contains(t, pack.SetupGaps, "token_refs_missing")
	assert.Contains(t, pack.SetupGaps, "component_refs_missing")
}
