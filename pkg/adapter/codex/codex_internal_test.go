package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Hooks tests ---

func TestGenerateHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateHooks(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, filepath.Join(".codex", "hooks.json"), files[0].TargetPath)
	assert.FileExists(t, filepath.Join(dir, ".codex", "hooks.json"))
	assert.Contains(t, string(files[0].Content), "SessionStart")
}

func TestPrepareHooksFile_NoDiskWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareHooksFile(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	_, err = os.Stat(filepath.Join(dir, ".codex", "hooks.json"))
	assert.True(t, os.IsNotExist(err))
}

// --- Settings/Config tests ---

func TestGenerateConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "config.toml", files[0].TargetPath)
	assert.FileExists(t, filepath.Join(dir, "config.toml"))
	assert.Contains(t, string(files[0].Content), "test-project")
	assert.Contains(t, string(files[0].Content), "context7")
}

func TestPrepareConfigFile_NoDiskWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareConfigFile(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	_, err = os.Stat(filepath.Join(dir, "config.toml"))
	assert.True(t, os.IsNotExist(err))
}

// --- Rules tests ---

func TestGenerateRuleFiles_Internal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateRuleFiles(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 8, "should produce 8 rule files")

	// Verify file-size-limit content
	for _, f := range files {
		if filepath.Base(f.TargetPath) == "file-size-limit.md" {
			assert.Contains(t, string(f.Content), "300 lines")
		}
	}
}

func TestPrepareRuleMappings_NoDiskWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareRuleMappings(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 8)

	// Should not write to disk
	_, err = os.Stat(filepath.Join(dir, ".codex", "rules", "autopus"))
	assert.True(t, os.IsNotExist(err))
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "with frontmatter",
			input: "---\nname: test\ncategory: workflow\n---\n\n# Content\n\nBody here.",
			want:  "\n# Content\n\nBody here.",
		},
		{
			name:  "no frontmatter",
			input: "# Just Content\n\nNo frontmatter.",
			want:  "# Just Content\n\nNo frontmatter.",
		},
		{
			name:  "incomplete frontmatter",
			input: "---\nname: test\nno closing",
			want:  "---\nname: test\nno closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripFrontmatter(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Marker tests ---

func TestInjectMarkerSection_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	result, err := a.injectMarkerSection(cfg)
	require.NoError(t, err)
	assert.Contains(t, result, markerBegin)
	assert.Contains(t, result, markerEnd)
	assert.Contains(t, result, "test-project")
}

func TestInjectMarkerSection_ExistingMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	existing := "# My Rules\n\n" + markerBegin + "\nold content\n" + markerEnd + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0644))

	result, err := a.injectMarkerSection(cfg)
	require.NoError(t, err)
	assert.Contains(t, result, "My Rules")
	assert.Contains(t, result, "test-project")
	assert.NotContains(t, result, "old content")
}

func TestReplaceMarkerSection(t *testing.T) {
	t.Parallel()
	content := "before\n" + markerBegin + "\nold\n" + markerEnd + "\nafter"
	newSection := markerBegin + "\nnew\n" + markerEnd
	result := replaceMarkerSection(content, newSection)
	assert.Contains(t, result, "before")
	assert.Contains(t, result, "new")
	assert.Contains(t, result, "after")
	assert.NotContains(t, result, "old")
}

func TestRemoveMarkerSection(t *testing.T) {
	t.Parallel()
	content := "header\n" + markerBegin + "\ncontent\n" + markerEnd + "\nfooter"
	result := removeMarkerSection(content)
	assert.Contains(t, result, "header")
	assert.Contains(t, result, "footer")
	assert.NotContains(t, result, markerBegin)
}

// --- SupportsHooks ---

func TestSupportsHooks_ReturnsTrue(t *testing.T) {
	t.Parallel()
	a := New()
	assert.True(t, a.SupportsHooks())
}

// --- Hooks JSON validity ---

func TestGenerateHooks_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateHooks(cfg)
	require.NoError(t, err)
	require.Len(t, files, 1)

	var parsed map[string]interface{}
	err = json.Unmarshal(files[0].Content, &parsed)
	require.NoError(t, err, "hooks.json should be valid JSON")

	hooks, ok := parsed["hooks"].(map[string]interface{})
	require.True(t, ok, "should have hooks key")
	assert.Contains(t, hooks, "SessionStart")
	assert.Contains(t, hooks, "Stop")
}

// --- Config mcp_servers ---

func TestGenerateConfig_MCPServers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	content := string(files[0].Content)
	assert.Contains(t, content, "[mcp_servers.context7]")
}

// --- Rules reference in AGENTS.md ---

func TestRulesReferenceInAgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "## Rules")
	assert.Contains(t, content, "See .codex/rules/autopus/ for detailed guidelines")
	// Should NOT contain inline rule content
	assert.NotContains(t, content, "IMPORTANT: No single file may exceed 300 lines")
}

func TestMarkerSection_Under32KB(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	result, err := a.injectMarkerSection(cfg)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(result), 32*1024,
		"marker section should be under 32KB, got %d bytes", len(result))
}
