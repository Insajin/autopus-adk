package omp

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOMPModelIntegration_BuiltinProfileCarriesQualityTiers proves the complete
// opt-in path from a derived mixed-family profile to every projected OMP agent.
func TestOMPModelIntegration_BuiltinProfileCarriesQualityTiers(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("builtin-omp")
	cfg.Platforms = []string{"omp"}
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1,
		Profile: "balanced",
	}
	require.NoError(t, cfg.Validate())

	integration, err := NewWithRoot(t.TempDir()).
		WithModelIntegrationRunner(newBuiltinTierIntegrationRunner()).
		prepareModelIntegration(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, integration)
	assert.Equal(t, "balanced", integration.profileName)
	assert.Equal(t, config.RoleModelCatalogTrustOperatorAttested, integration.profile.CatalogTrust)

	selectors := make(map[string]string, len(integration.projection.Agents))
	for _, agent := range integration.projection.Agents {
		selectors[agent.Agent] = agent.EffectiveSelector
	}
	const (
		fable  = "anthropic/" + config.ClaudeFableModel + ":max"
		opus   = "anthropic/" + config.ClaudeOpusModel + ":xhigh"
		astra  = "openai-codex/" + config.CodexAstraModel + ":max"
		sonnet = "anthropic/" + config.ClaudeSonnetModel + ":medium"
	)
	for agent, want := range map[string]string{
		"architect": fable, "planner": fable, "spec-writer": fable,
		"debugger": opus, "deep-worker": opus, "devops": opus,
		"executor": opus, "perf-engineer": opus, "tester": opus,
		"reviewer": astra, "security-auditor": astra,
		"annotator": sonnet, "explorer": sonnet,
		"frontend-specialist": sonnet, "ux-validator": sonnet,
		"validator": sonnet,
	} {
		assert.Equal(t, want, selectors[agent], agent)
	}
	assert.Len(t, selectors, len(config.CanonicalAgentNames()))
}

func TestOMPModelIntegration_NoProfileIgnoresQualityPresets(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("builtin-omp-opt-in")
	cfg.Platforms = []string{"omp"}
	runner := newBuiltinTierIntegrationRunner()

	integration, err := NewWithRoot(t.TempDir()).
		WithModelIntegrationRunner(runner).
		prepareModelIntegration(context.Background(), cfg)
	require.NoError(t, err)
	assert.Nil(t, integration, "quality presets alone must not activate model routing")
	assert.Empty(t, runner.calls, "an unselected policy must not probe the catalog")
}

// newBuiltinTierIntegrationRunner mirrors the metadata-light catalog emitted by
// OMP 18.1.10. The derived profile supplies family and capability attestations.
func newBuiltinTierIntegrationRunner() *modelIntegrationFakeRunner {
	return &modelIntegrationFakeRunner{catalog: []byte(`{"models":[
{"provider":"anthropic","id":"` + config.ClaudeFableModel + `","selector":"anthropic/` + config.ClaudeFableModel + `","thinking":["max"]},
{"provider":"anthropic","id":"` + config.ClaudeOpusModel + `","selector":"anthropic/` + config.ClaudeOpusModel + `","thinking":["xhigh"]},
{"provider":"anthropic","id":"` + config.ClaudeSonnetModel + `","selector":"anthropic/` + config.ClaudeSonnetModel + `","thinking":["medium"]},
{"provider":"anthropic","id":"` + config.ClaudeHaikuModel + `","selector":"anthropic/` + config.ClaudeHaikuModel + `","thinking":["low"]},
{"provider":"openai-codex","id":"` + config.CodexAstraModel + `","selector":"openai-codex/` + config.CodexAstraModel + `","thinking":["max"]},
{"provider":"openai-codex","id":"` + config.CodexSolModel + `","selector":"openai-codex/` + config.CodexSolModel + `","thinking":["xhigh"]},
{"provider":"openai-codex","id":"` + config.CodexTerraModel + `","selector":"openai-codex/` + config.CodexTerraModel + `","thinking":["medium"]},
{"provider":"openai-codex","id":"` + config.CodexLunaModel + `","selector":"openai-codex/` + config.CodexLunaModel + `","thinking":["low"]}]}`)}
}
