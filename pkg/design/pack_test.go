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

func TestBuildDesignSystemDocsDetectsAstryx(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "package.json", `{
  "dependencies": {
    "@astryxdesign/core": "1.0.0",
    "@astryxdesign/theme-neutral": "1.0.0"
  },
  "devDependencies": {
    "@astryxdesign/cli": "1.0.0"
  }
}`)

	docs, err := BuildDesignSystemDocs(root, DesignSystemDocsOptions{MaxRefs: 10})
	require.NoError(t, err)
	require.Len(t, docs.Providers, 1)
	assert.Equal(t, "astryx", docs.Providers[0].Name)
	assert.Contains(t, docs.Providers[0].Packages, "@astryxdesign/core")
	assert.Contains(t, docs.Providers[0].Preflight, "npx astryx component <Name> --dense")
	assert.NotContains(t, docs.Providers[0].Preflight, "npx astryx init --features agents --agent codex")
	assert.Equal(t, "https://astryx.atmeta.com/mcp", docs.Providers[0].MCP)
	assert.Contains(t, docs.Markdown(), "npx astryx template --list --dense")
}

func TestBuildDesignSystemDocsSkipsMalformedPackageManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies":{"@astryxdesign/core":"1.0.0"}}`)
	writeFile(t, root, "examples/broken/package.json", `{not-json`)

	docs, err := BuildDesignSystemDocs(root, DesignSystemDocsOptions{MaxRefs: 10})
	require.NoError(t, err)
	require.Len(t, docs.Providers, 1)
	assert.Equal(t, "astryx", docs.Providers[0].Name)
}

func TestBuildDesignSystemDocsDoesNotInferExplicitRadixFromLocalComponents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "src/components/ui/button.tsx", "export function Button() { return null }")

	docs, err := BuildDesignSystemDocs(root, DesignSystemDocsOptions{
		Providers: []string{"radix"},
		MaxRefs:   10,
	})
	require.NoError(t, err)
	assert.Empty(t, docs.Providers)
	assert.Contains(t, docs.SetupGaps, "design_system_docs_provider_missing")
}

func TestBuildDesignSystemDocsDetectsProjectLocalShadcnStack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "package.json", `{
  "dependencies": {
    "@radix-ui/react-dialog": "1.0.0",
    "tailwindcss": "4.0.0"
  }
}`)
	writeFile(t, root, "src/components/ui/button.tsx", "export function Button() { return null }")
	writeFile(t, root, "tokens/semantic/color.light.json", "{}")

	docs, err := BuildDesignSystemDocs(root, DesignSystemDocsOptions{MaxRefs: 10})
	require.NoError(t, err)
	require.Len(t, docs.Providers, 2)
	assert.Equal(t, "shadcn-radix-tailwind", docs.Providers[0].Name)
	assert.Equal(t, "local-design-sources", docs.Providers[1].Name)
	assert.Contains(t, docs.Providers[0].Packages, "@radix-ui/react-dialog")
	assert.Contains(t, docs.Providers[0].Packages, "tailwindcss")
	assert.Contains(t, docs.Providers[0].Preflight, "auto design pack --format markdown")
}

func TestBuildPackIncludesDesignSystemDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "# Design\n\nUse tokens.")
	writeFile(t, root, "package.json", `{"dependencies":{"@astryxdesign/core":"1.0.0"}}`)

	pack, err := BuildPack(root, PackOptions{ContextOptions: Options{Enabled: true}, MaxRefs: 10})
	require.NoError(t, err)
	require.NotEmpty(t, pack.DesignDocs.Providers)
	assert.Contains(t, pack.Markdown(), "Design-system docs")
	assert.Contains(t, pack.Markdown(), "astryx")
}
