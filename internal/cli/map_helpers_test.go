package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/setup"
)

// TestDisplayRepoPath_EmptyAndDot verifies dot / empty normalization.
func TestDisplayRepoPath_EmptyAndDot(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".", displayRepoPath(""))
	assert.Equal(t, ".", displayRepoPath("."))
	assert.Equal(t, ".", displayRepoPath("   "))
	assert.Equal(t, "a/b", displayRepoPath("a/b"))
}

// TestDefaultMapText_FallbackOnEmpty returns fallback when value is blank.
func TestDefaultMapText_FallbackOnEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Unknown", defaultMapText("", "Unknown"))
	assert.Equal(t, "Unknown", defaultMapText("  ", "Unknown"))
	assert.Equal(t, "Go", defaultMapText("Go", "Unknown"))
}

// TestMapLanguages_SortedWithVersion verifies language formatting and sort order.
func TestMapLanguages_SortedWithVersion(t *testing.T) {
	t.Parallel()

	langs := []setup.Language{
		{Name: "TypeScript", Version: "5.0"},
		{Name: "Go", Version: "1.22"},
		{Name: "Python"},
	}
	got := mapLanguages(langs)
	assert.Equal(t, []string{"Go 1.22", "Python", "TypeScript 5.0"}, got)
}

// TestMapLanguages_Empty returns empty slice for no languages.
func TestMapLanguages_Empty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, mapLanguages(nil))
}

// TestMapFrameworks_SortedWithVersion verifies framework formatting and order.
func TestMapFrameworks_SortedWithVersion(t *testing.T) {
	t.Parallel()

	fw := []setup.Framework{
		{Name: "React", Version: "18"},
		{Name: "Cobra"},
	}
	got := mapFrameworks(fw)
	assert.Equal(t, []string{"Cobra", "React 18"}, got)
}

// TestMapBuildFiles_ForwardSlash verifies paths use forward slashes.
func TestMapBuildFiles_ForwardSlash(t *testing.T) {
	t.Parallel()

	files := []setup.BuildFile{{Path: "cmd/main.go"}, {Path: "Makefile"}}
	got := mapBuildFiles(files)
	assert.Equal(t, []string{"Makefile", "cmd/main.go"}, got)
}

// TestMapEntryPoints_ForwardSlash verifies entry-point paths use forward slashes.
func TestMapEntryPoints_ForwardSlash(t *testing.T) {
	t.Parallel()

	eps := []setup.EntryPoint{{Path: "cmd/main.go"}}
	got := mapEntryPoints(eps)
	assert.Equal(t, []string{"cmd/main.go"}, got)
}

// TestWriteMapText_RendersBasicFields verifies the text renderer writes key fields.
func TestWriteMapText_RendersBasicFields(t *testing.T) {
	t.Parallel()

	cmd := newMapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	payload := mapPayload{
		ProjectDir:  "/proj",
		ProjectName: "myproject",
		MultiRepo:   false,
		Languages:   []string{"Go 1.22"},
		Frameworks:  []string{"Cobra"},
	}
	warnings := []jsonMessage{{Code: "repo_dirty", Message: "repo has uncommitted changes"}}
	writeMapText(cmd, payload, warnings)

	out := buf.String()
	assert.Contains(t, out, "myproject")
	assert.Contains(t, out, "/proj")
	assert.Contains(t, out, "Go 1.22")
	assert.Contains(t, out, "Cobra")
	assert.Contains(t, out, "repo has uncommitted changes")
}

// TestWriteMapText_RendersRepoAndContextLines verifies repo and context blocks.
func TestWriteMapText_RendersRepoAndContextLines(t *testing.T) {
	t.Parallel()

	cmd := newMapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	payload := mapPayload{
		ProjectDir:          "/proj",
		ProjectName:         "myproject",
		ProjectContextDir:   "/proj/.autopus/project",
		ProjectContextFiles: []string{"product.md", "architecture.md"},
		Repositories: []mapRepoPayload{
			{
				Path:           ".",
				Role:           "main",
				PrimaryLang:    "Go",
				Dirty:          true,
				Branch:         "main",
				TrackedIgnored: []string{"secret.env"},
			},
		},
	}
	writeMapText(cmd, payload, nil)

	out := buf.String()
	assert.Contains(t, out, ".autopus/project")
	assert.Contains(t, out, "2 file(s)")
	assert.Contains(t, out, "dirty")
	assert.Contains(t, out, "[main]")
	assert.Contains(t, out, "tracked ignored")
	assert.Contains(t, out, "secret.env")
}

// TestNewMapCmd_JSONOutput verifies the map command emits valid JSON against a real dir.
func TestNewMapCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := newMapCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--json", dir})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, `"schema_version"`)
	assert.Contains(t, out, `"project_dir"`)
}
