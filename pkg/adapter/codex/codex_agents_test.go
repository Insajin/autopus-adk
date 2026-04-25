package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAgents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateAgents(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 16, "should generate 16 TOML agent files")

	for _, f := range files {
		fullPath := filepath.Join(dir, f.TargetPath)
		assert.FileExists(t, fullPath)
		assert.Contains(t, f.TargetPath, ".codex/agents/")
		assert.Contains(t, string(f.Content), "test-project")
	}
}

func TestGenerateAgents_TOMLContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateAgents(cfg)
	require.NoError(t, err)

	for _, f := range files {
		content := string(f.Content)
		assert.Contains(t, content, "name =", "TOML %s should have name field", f.TargetPath)
		assert.Contains(t, content, "description =", "TOML %s should have description field", f.TargetPath)
		assert.Contains(t, content, `model = "gpt-5.5"`, "TOML %s should use the Codex frontier model", f.TargetPath)
		assert.Contains(t, content, "model_reasoning_effort =", "TOML %s should set effort explicitly", f.TargetPath)
		assert.Contains(t, content, "developer_instructions =", "TOML %s should have instructions", f.TargetPath)
		assert.Contains(t, content, "developer_instructions = '''", "TOML %s should use literal multiline strings", f.TargetPath)
		assert.NotContains(t, content, "[developer_instructions]", "TOML %s should use flat instructions field", f.TargetPath)
	}
}

func TestGenerateAgents_BalancedQualityUsesRoleEffort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateAgents(cfg)
	require.NoError(t, err)

	byPath := make(map[string]string, len(files))
	for _, f := range files {
		byPath[f.TargetPath] = string(f.Content)
	}

	assert.Contains(t, byPath[filepath.Join(".codex", "agents", "planner.toml")], `model_reasoning_effort = "xhigh"`)
	assert.Contains(t, byPath[filepath.Join(".codex", "agents", "reviewer.toml")], `model_reasoning_effort = "high"`)
	assert.Contains(t, byPath[filepath.Join(".codex", "agents", "executor.toml")], `model_reasoning_effort = "medium"`)
}

func TestGenerateAgents_UltraQualityUsesXHighEffort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	cfg.Quality.Default = "ultra"

	files, err := a.generateAgents(cfg)
	require.NoError(t, err)

	for _, f := range files {
		content := string(f.Content)
		assert.Contains(t, content, `model = "gpt-5.5"`, "TOML %s should use the Codex frontier model", f.TargetPath)
		assert.Contains(t, content, `model_reasoning_effort = "xhigh"`, "TOML %s should use xhigh in ultra mode", f.TargetPath)
	}
}

func TestPrepareAgentFiles_NoDiskWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.prepareAgentFiles(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 16)

	agentsDir := filepath.Join(dir, ".codex", "agents")
	_, err = os.Stat(agentsDir)
	assert.True(t, os.IsNotExist(err), "prepareAgentFiles should not create files on disk")
}

func TestRenderAgentsSection(t *testing.T) {
	t.Parallel()
	section, err := renderAgentsSection()
	require.NoError(t, err)
	assert.Contains(t, section, "## Agents")
	// Should contain at least some agent names from content/agents/.
	assert.NotEmpty(t, section)
}

func TestExtractAgentMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
	}{
		{
			name:     "heading and description",
			content:  "# Executor\n\nImplements code from SPEC.\n\n## Details",
			wantName: "Executor",
			wantDesc: "Implements code from SPEC.",
		},
		{
			name:     "with frontmatter",
			content:  "---\nname: test\n---\n# Reviewer\n\nReviews code quality.",
			wantName: "Reviewer",
			wantDesc: "Reviews code quality.",
		},
		{
			name:     "no heading",
			content:  "Just some text without heading.",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "heading only",
			content:  "# Solo Heading\n",
			wantName: "Solo Heading",
			wantDesc: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, desc := extractAgentMeta(tt.content)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantDesc, desc)
		})
	}
}
