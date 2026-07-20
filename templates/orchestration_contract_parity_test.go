package templates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrationSemanticContract_CodexAndClaudeBindCanonicalPolicy(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	canonical := readContractSurface(t, filepath.Join(root, "shared", "orchestration-contract.md.tmpl"))
	codexGo := readContractSurface(t, filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"))
	codexReview := readContractSurface(t, filepath.Join(root, "codex", "skills", "auto-review.md.tmpl"))
	claude := readContractSurface(t, filepath.Join(root, "claude", "commands", "auto-workflows.md.tmpl"))

	requiredSemanticTokens := []string{
		"orchestration-contract.v1",
		"orchestration_run_receipt.v1",
		"requested_providers",
		"configured_providers",
		"attempted_providers",
		"usable_providers",
		"failed_providers",
		"degraded_reasons",
		"critical_veto",
		"analysis_verdict",
		"gate_status",
	}
	for _, token := range requiredSemanticTokens {
		assert.Contains(t, canonical, token, "canonical contract missing %s", token)
		assert.Contains(t, codexGo+codexReview, token, "Codex surface missing %s", token)
		assert.Contains(t, claude, token, "Claude surface missing %s", token)
	}

	for name, surface := range map[string]string{
		"codex":  codexGo + codexReview,
		"claude": claude,
	} {
		assert.Contains(t, surface, "auto orchestra review", "%s must use the risk-tier review entrypoint", name)
		assert.Contains(t, surface, "--risk-tier", "%s must forward risk tier", name)
		assert.NotContains(t, surface, "auto orchestra run \"{review topic}\"", "%s must not use generic debate for code review", name)
	}
}

func TestOrchestrationSemanticContract_SpecPromotionAndIdeaForwardingParity(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	claude := readContractSurface(t, filepath.Join(root, "claude", "commands", "auto-workflows.md.tmpl"))
	codexIdea := readContractSurface(t, filepath.Join(root, "codex", "skills", "auto-idea.md.tmpl"))
	canonicalIdea := readRepoSurface(t, filepath.Join("content", "skills", "idea.md"))
	canonicalReview := readRepoSurface(t, filepath.Join("content", "skills", "spec-review.md"))

	for _, surface := range []string{claude, canonicalReview} {
		for _, token := range []string{"status_changed", "degraded_reasons", "override_applied", "--allow-degraded"} {
			assert.Contains(t, surface, token, "SPEC review promotion contract missing %s", token)
		}
	}
	for _, surface := range []string{canonicalIdea, codexIdea, claude} {
		assert.Contains(t, surface, "--strategy", "idea strategy must be forwarded")
		assert.Contains(t, surface, "--providers", "idea providers must be forwarded")
		assert.Contains(t, surface, "orchestration-contract.v1", "idea surface must bind the canonical contract")
		assert.Contains(t, surface, "orchestra_unavailable", "idea fallback must fail closed without a native worker/judge surface")
	}
	assert.NotContains(t, codexIdea+claude, "Sequential Thinking으로 fallback")
}

func TestOrchestrationSemanticContract_CodexSourceContainsNoClaudeTeamPrimitives(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	codexTeam := readRepoSurface(t, filepath.Join("pkg", "adapter", "codex", "codex_extended_skill_rewrites_agents.go"))
	codexPipeline := readRepoSurface(t, filepath.Join("pkg", "adapter", "codex", "codex_extended_skill_rewrites_pipeline.go"))
	surface := codexTeam + codexPipeline

	for _, forbidden := range []string{"TeamCreate(", "TeamDelete(", "SendMessage(", "bypassPermissions", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"} {
		assert.NotContains(t, surface, forbidden)
	}
	assert.Contains(t, surface, "spawn_agent")
	assert.NotEmpty(t, root)
}

func readContractSurface(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read contract surface %s", path)
	return string(data)
}

func readRepoSurface(t *testing.T, relative string) string {
	t.Helper()
	path := filepath.Join("..", relative)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read repository surface %s", path)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
