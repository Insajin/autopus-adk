package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
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

func TestGenerateConfig_PreservesExistingCodexModelSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	configPath := filepath.Join(dir, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`model = "gpt-5.4"
model_reasoning_effort = "xhigh"
model_reasoning_summary = "detailed"
model_verbosity = "high"
approval_policy = "never"
`), 0644))

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	content := string(files[0].Content)

	assert.Contains(t, content, `model = "gpt-5.4"`)
	assert.Contains(t, content, `model_reasoning_effort = "xhigh"`)
	assert.Contains(t, content, `model_reasoning_summary = "detailed"`)
	assert.Contains(t, content, `model_verbosity = "high"`)
	assert.Contains(t, content, `approval_policy = "on-request"`)
}

func TestGenerateConfig_UsesUltraQualityEffort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")
	cfg.Quality.Default = "ultra"

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	content := string(files[0].Content)

	rootSection := strings.SplitN(content, "[agents]", 2)[0]
	assert.Contains(t, rootSection, `model = "gpt-5.5"`)
	assert.Contains(t, rootSection, `model_reasoning_effort = "xhigh"`)
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
	assert.NotContains(t, content, "[mcp_servers.autopus]")
	assert.NotContains(t, content, `args = ["mcp", "server"]`)
	assert.Contains(t, content, "[mcp_servers.context7]")
	assert.Contains(t, content, `command = "npx"`)
	assert.Contains(t, content, `args = ["-y", "@upstash/context7-mcp@latest"]`)
	assert.NotContains(t, content, "@anthropic-ai/context7-mcp")
	assert.Contains(t, content, `model = "gpt-5.5"`)
	assert.Contains(t, content, `approval_policy = "on-request"`)
	assert.Contains(t, content, `sandbox_mode = "workspace-write"`)
	assert.Contains(t, content, `web_search = "cached"`)
	assert.Contains(t, content, "project_doc_max_bytes = 262144")
	assert.Contains(t, content, "[agents]")
	assert.Contains(t, content, "max_threads = 6")
	assert.Contains(t, content, "max_depth = 1")
	assert.Contains(t, content, "[features]")
	assert.Contains(t, content, "goals = true")
	assert.Contains(t, content, "multi_agent = true")
	assert.NotContains(t, content, "features.collab")
}

func TestGenerateConfig_EnablesBundledBrowserUsePlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.generateConfig(cfg)
	require.NoError(t, err)
	content := string(files[0].Content)

	assert.Contains(t, content, `[plugins."browser-use@openai-bundled"]`)
	assert.Contains(t, content, "enabled = true")
}

func TestValidateConfig_WarnsWhenBundledBrowserUsePluginMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	configPath := filepath.Join(dir, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`model = "gpt-5.5"
model_reasoning_effort = "medium"
approval_policy = "on-request"
sandbox_mode = "workspace-write"
web_search = "cached"
project_doc_max_bytes = 262144
`), 0644))

	var errs []adapter.ValidationError
	a.validateConfig(&errs)

	found := false
	for _, e := range errs {
		if e.File == codexConfigRelPath && e.Message == "Codex bundled browser-use plugin이 enabled 상태가 아님" {
			found = true
		}
	}
	assert.True(t, found, "missing browser-use plugin enablement should warn")
}

func TestValidateConfig_WarnsWhenGoalOrMultiAgentFeatureMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	configPath := filepath.Join(dir, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`model = "gpt-5.5"
model_reasoning_effort = "medium"
approval_policy = "on-request"
sandbox_mode = "workspace-write"
web_search = "cached"
project_doc_max_bytes = 262144

[features]
shell_tool = true

[plugins."browser-use@openai-bundled"]
enabled = true
`), 0644))

	var errs []adapter.ValidationError
	a.validateConfig(&errs)

	messages := make(map[string]bool)
	for _, e := range errs {
		messages[e.Message] = true
	}
	assert.True(t, messages["Codex goals feature가 enabled 상태가 아님"])
	assert.True(t, messages["Codex multi_agent feature가 enabled 상태가 아님"])
}
