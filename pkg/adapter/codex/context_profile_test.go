package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestCodexAdapter_ContextProfilesKeepRouterCoreAndDetailsScoped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := codex.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-profile"))
	require.NoError(t, err)

	router := readCodexContextSurface(t, root, ".codex/prompts/auto.md")
	assert.Contains(t, router, "core")
	assert.Contains(t, router, "architecture")
	assert.Contains(t, router, "plan = core + architecture + relevant SPEC")
	assert.Contains(t, router, "test = core + test")
	assert.Contains(t, router, "canary = core + canary")
	assert.NotContains(t, router, "Always load every project context document")

	plan := readCodexContextSurface(t, root, ".agents/skills/auto-plan/SKILL.md")
	assert.Contains(t, plan, "Context Profile: plan")
	assert.Contains(t, plan, "relevant SPEC")
	assert.NotContains(t, plan, ".autopus/project/scenarios.md")
	assert.NotContains(t, plan, ".autopus/project/canary.md")

	testSkill := readCodexContextSurface(t, root, ".agents/skills/auto-test/SKILL.md")
	assert.Contains(t, testSkill, "Context Profile: test")
	assert.Contains(t, testSkill, ".autopus/project/scenarios.md")
	assert.NotContains(t, testSkill, ".autopus/project/canary.md")

	canary := readCodexContextSurface(t, root, ".agents/skills/auto-canary/SKILL.md")
	assert.Contains(t, canary, "Context Profile: canary")
	assert.Contains(t, canary, ".autopus/project/canary.md")
	assert.NotContains(t, canary, ".autopus/project/scenarios.md")
	assert.NotContains(t, canary, ".autopus/context/signatures.md")

	pipelineSkill, err := contentfs.FS.ReadFile("skills/agent-pipeline.md")
	require.NoError(t, err)
	assert.Contains(t, string(pipelineSkill), "## Scoped Context Receipt Contract")
	assert.Contains(t, string(pipelineSkill), "800 and 2,000")
	assert.Contains(t, string(pipelineSkill), "Outcome Lock")
	assert.Contains(t, string(pipelineSkill), "owned paths")
	assert.Contains(t, string(pipelineSkill), "prompt-manifest hash")
	assert.Contains(t, string(pipelineSkill), "Do not relay full repeated artifact bodies")
}

func readCodexContextSurface(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	return string(body)
}
