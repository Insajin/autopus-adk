package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, codexConfigRelPath, files[0].TargetPath)
	assert.FileExists(t, filepath.Join(dir, ".codex", "config.toml"))
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

	_, err = os.Stat(filepath.Join(dir, ".codex", "config.toml"))
	assert.True(t, os.IsNotExist(err))
}

func TestGenerateConfig_MCPServers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	content := string(files[0].Content)
	assert.Contains(t, content, "[mcp_servers.autopus]")
	assert.Contains(t, content, `command = "auto"`)
	assert.Contains(t, content, `args = ["mcp", "server"]`)
	assert.Contains(t, content, "[mcp_servers.context7]")
	assert.Contains(t, content, `model = "gpt-5.5"`)
	assert.Contains(t, content, `approval_policy = "on-request"`)
	assert.Contains(t, content, `sandbox_mode = "workspace-write"`)
	assert.Contains(t, content, `web_search = "cached"`)
	assert.Contains(t, content, "project_doc_max_bytes = 262144")
	assert.Contains(t, content, "[agents]")
	assert.Contains(t, content, "max_threads = 6")
	assert.Contains(t, content, "max_depth = 1")
	assert.NotContains(t, content, "features.collab")
}
