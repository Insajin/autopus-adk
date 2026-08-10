package omp

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOMPModelIntegration_BuiltinProfileCarriesQualityTiers proves the whole
// opt-in path: naming a built-in profile with no hand-written candidate list
// still routes every OMP agent to the model its quality preset tier implies.
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
		WithModelIntegrationRunner(newClaudeTierIntegrationRunner()).
		prepareModelIntegration(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, integration)
	assert.Equal(t, "balanced", integration.profileName)

	selectors := make(map[string]string, len(integration.projection.Agents))
	for _, agent := range integration.projection.Agents {
		selectors[agent.Agent] = agent.EffectiveSelector
	}
	const (
		opus   = "anthropic/" + config.ClaudeOpusModel + ":xhigh"
		sonnet = "anthropic/" + config.ClaudeSonnetModel + ":medium"
	)
	// tester and reviewer are sonnet agents that share a capability with an
	// opus agent (executor, security-auditor), so max-wins lifts them.
	for agent, want := range map[string]string{
		"planner": opus, "architect": opus, "executor": opus, "tester": opus,
		"reviewer": opus, "annotator": sonnet, "validator": sonnet,
		"ux-validator": sonnet, "frontend-specialist": sonnet,
	} {
		assert.Equal(t, want, selectors[agent], agent)
	}
	assert.Len(t, selectors, len(config.CanonicalAgentNames()))
}

func TestOMPModelIntegration_NoProfileIgnoresQualityPresets(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("builtin-omp-opt-in")
	cfg.Platforms = []string{"omp"}
	runner := newClaudeTierIntegrationRunner()

	integration, err := NewWithRoot(t.TempDir()).
		WithModelIntegrationRunner(runner).
		prepareModelIntegration(context.Background(), cfg)
	require.NoError(t, err)
	assert.Nil(t, integration, "quality presets alone must not activate model routing")
	assert.Empty(t, runner.calls, "an unselected policy must not probe the catalog")
}

// newClaudeTierIntegrationRunner serves a catalog whose only models are the
// Claude slugs a built-in profile derives, each at its own thinking level.
func newClaudeTierIntegrationRunner() *modelIntegrationFakeRunner {
	const capabilities = `["deep_reasoning","coding_tool_use","fast_validation","vision_design","independent_dissent","deterministic_transform"]`
	return &modelIntegrationFakeRunner{catalog: []byte(`{"models":[
{"provider":"anthropic","id":"` + config.ClaudeOpusModel + `","family":"anthropic","capabilities":` + capabilities + `,"thinking":["xhigh"],"auth_enabled":true},
{"provider":"anthropic","id":"` + config.ClaudeSonnetModel + `","family":"anthropic","capabilities":` + capabilities + `,"thinking":["medium"],"auth_enabled":true},
{"provider":"anthropic","id":"` + config.ClaudeHaikuModel + `","family":"anthropic","capabilities":` + capabilities + `,"thinking":["low"],"auth_enabled":true}]}`)}
}
