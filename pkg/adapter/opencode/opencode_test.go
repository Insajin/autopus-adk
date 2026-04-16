package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestNew_DefaultRoot(t *testing.T) {
	t.Parallel()
	a := New()
	assert.Equal(t, ".", a.root)
}

func TestNewWithRoot(t *testing.T) {
	t.Parallel()
	a := NewWithRoot("/some/path")
	assert.Equal(t, "/some/path", a.root)
}

func TestAdapter_Accessors(t *testing.T) {
	t.Parallel()
	a := New()
	assert.Equal(t, "opencode", a.Name())
	assert.Equal(t, "1.0.0", a.Version())
	assert.Equal(t, "opencode", a.CLIBinary())
	assert.True(t, a.SupportsHooks())
}

func TestAdapter_Detect_NoError(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	_, err := a.Detect(context.Background())
	assert.NoError(t, err)
}

func TestAdapter_Generate_CreatesOpenCodeFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")

	pf, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, pf.Files)

	assert.FileExists(t, filepath.Join(dir, "AGENTS.md"))
	assert.FileExists(t, filepath.Join(dir, "opencode.json"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "commands", "auto.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "commands", "auto-setup.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "commands", "auto-plan.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "planner.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "plugins", "autopus-hooks.js"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "planning", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "opencode-manifest.json"))

	autoSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSkill), "얇은 라우터")
	assert.Contains(t, string(autoSkill), "상세 스킬")
	assert.Contains(t, string(autoSkill), "## Router Contract")
	assert.Contains(t, string(autoSkill), "## Context Load")
	assert.Contains(t, string(autoSkill), "## SPEC Path Resolution")
	assert.Contains(t, string(autoSkill), "## Autopus Branding")
	assert.Contains(t, string(autoSkill), "🐙 Autopus ─────────────────────────")
	assert.NotContains(t, string(autoSkill), "mode =")

	autoIdeaSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-idea", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoIdeaSkill), "auto orchestra brainstorm")
	assert.Contains(t, string(autoIdeaSkill), "## OpenCode Invocation")
	assert.Contains(t, string(autoIdeaSkill), "## Autopus Branding")

	autoSetupSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-setup", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSetupSkill), "ARCHITECTURE.md")
	assert.Contains(t, string(autoSetupSkill), "explorer")
	assert.Contains(t, string(autoSetupSkill), "## OpenCode Invocation")

	agentPipelineSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "agent-pipeline", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentPipelineSkill), "Context7 MCP")
	assert.Contains(t, string(agentPipelineSkill), "web search")

	autoGoSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto-go", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoGoSkill), "SPEC Path Resolution")
	assert.Contains(t, string(autoGoSkill), "{SPEC_PATH}")
	assert.Contains(t, string(autoGoSkill), "WORKING_DIR")
	assert.Contains(t, string(autoGoSkill), "--model <provider/model>")
	assert.Contains(t, string(autoGoSkill), "--variant <value>")
	autoCommand, err := os.ReadFile(filepath.Join(dir, ".opencode", "commands", "auto.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoCommand), "Immediately load skill `auto`")
	assert.Contains(t, string(autoCommand), "Preserve `--model <provider/model>`")
	assert.Contains(t, string(autoCommand), "Do not restate or expand the arguments")

	agentsData, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentsData), markerBegin)
	assert.Contains(t, string(agentsData), "플랫폼")
	assert.Contains(t, string(agentsData), "## Execution Model")
	assert.Contains(t, string(agentsData), "task(...)")
	assert.Contains(t, string(agentsData), "openai/gpt-5.4")
	assert.NotContains(t, string(agentsData), "Codex Rules: .codex/rules/autopus/")

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	instructions := jsonStringSlice(configDoc["instructions"])
	assert.NotEmpty(t, instructions)
	assert.Contains(t, instructions, ".opencode/rules/autopus/branding.md")

	context7Rule, err := os.ReadFile(filepath.Join(dir, ".opencode", "rules", "autopus", "context7-docs.md"))
	require.NoError(t, err)
	assert.Contains(t, string(context7Rule), "web search")
	assert.Contains(t, string(context7Rule), "official docs")
}

func TestAdapter_Generate_AutoRouterUsesThinOpenCodeContract(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)

	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	autoCommand, readErr := os.ReadFile(filepath.Join(dir, ".opencode", "commands", "auto.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(autoCommand), "Immediately load skill `auto`")
	assert.NotContains(t, string(autoCommand), "## SPEC Path Resolution")
	assert.NotContains(t, string(autoCommand), "Codex용 canonical router surface")

	autoSkill, readErr := os.ReadFile(filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(autoSkill), "## Router Contract")
	assert.Contains(t, string(autoSkill), "## Context Load")
	assert.Contains(t, string(autoSkill), "## SPEC Path Resolution")
	assert.Contains(t, string(autoSkill), "지원 서브커맨드")
	assert.Contains(t, string(autoSkill), "/auto-canary")
	assert.NotContains(t, string(autoSkill), "Codex용 canonical router surface")
}

func TestAdapter_Generate_NilConfig(t *testing.T) {
	t.Parallel()
	a := NewWithRoot(t.TempDir())
	_, err := a.Generate(context.Background(), nil)
	assert.Error(t, err)
}

func TestAdapter_Update_PreservesMergedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom Header\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"share":"manual"}`), 0644))

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	updated, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, updated.Files)

	agentsData, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agentsData), "# Custom Header")
	assert.Contains(t, string(agentsData), markerBegin)

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	assert.Equal(t, "manual", configDoc["share"])
	assert.Contains(t, jsonStringSlice(configDoc["instructions"]), ".opencode/rules/autopus/branding.md")
}

func TestAdapter_Validate_AfterGenerate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestAdapter_Clean_RemovesGeneratedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom Header\n"), 0644))
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	err = a.Clean(context.Background())
	require.NoError(t, err)

	assert.NoDirExists(t, filepath.Join(dir, ".opencode"))
	assert.NoFileExists(t, filepath.Join(dir, "opencode.json"))
	agentsData, readErr := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(agentsData), "# Custom Header")
	assert.NotContains(t, string(agentsData), markerBegin)
}

func TestAdapter_InstallHooks_WritesPlugin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	hooks := []adapter.HookConfig{{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet --warn-only", Timeout: 30}}

	err := a.InstallHooks(context.Background(), hooks, nil)
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(dir, ".opencode", "plugins", "autopus-hooks.js"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "auto check --arch --quiet --warn-only")
}

func TestInjectOrchestraPlugin_MergesPluginArray(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"plugin":["existing-plugin"]}`), 0644))

	err := a.InjectOrchestraPlugin("/path/to/script.js")
	require.NoError(t, err)

	configDoc := readConfigJSON(t, filepath.Join(dir, "opencode.json"))
	plugins := jsonStringSlice(configDoc["plugin"])
	assert.Contains(t, plugins, "existing-plugin")
	assert.Contains(t, plugins, "/path/to/script.js")
	assert.Contains(t, jsonStringSlice(configDoc["instructions"]), ".opencode/rules/autopus/branding.md")
}

func TestAdapter_Generate_WorkflowSkillsUseOpenCodeSurface(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	bannedInSkills := []string{
		"spawn_agent",
		"Agent(",
		"mode =",
		"permissionMode",
		"bypassPermissions",
	}

	for _, spec := range workflowSpecs {
		skillPath := filepath.Join(dir, ".agents", "skills", spec.Name, "SKILL.md")
		data, readErr := os.ReadFile(skillPath)
		require.NoError(t, readErr, skillPath)
		content := string(data)

		for _, banned := range bannedInSkills {
			assert.NotContains(t, content, banned, "%s should not contain %q", skillPath, banned)
		}

		if spec.Name == "auto" {
			assert.Contains(t, content, "얇은 라우터", skillPath)
			continue
		}

		assert.Contains(t, content, "## OpenCode Invocation", skillPath)
		assert.True(t, strings.Contains(content, "/auto "+strings.TrimPrefix(spec.Name, "auto-")) || strings.Contains(content, "/auto "+spec.Name), "%s should describe /auto invocation", skillPath)
	}

	bannedInCommands := []string{"spawn_agent", "Agent(", "mode =", "permissionMode"}
	for _, spec := range workflowSpecs {
		cmdPath := filepath.Join(dir, ".opencode", "commands", spec.Name+".md")
		data, readErr := os.ReadFile(cmdPath)
		require.NoError(t, readErr, cmdPath)
		content := string(data)
		assert.Contains(t, content, "`$ARGUMENTS`", cmdPath)
		assert.Contains(t, content, "Do not restate or expand the arguments", cmdPath)
		for _, banned := range bannedInCommands {
			assert.NotContains(t, content, banned, "%s should not contain %q", cmdPath, banned)
		}
	}
}

func readConfigJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	return doc
}
