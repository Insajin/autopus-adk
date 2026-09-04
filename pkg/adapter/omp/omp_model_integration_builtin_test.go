package omp

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	builtinFable  = "anthropic/" + config.ClaudeFableModel + ":max"
	builtinOpus   = "anthropic/" + config.ClaudeOpusModel + ":xhigh"
	builtinSonnet = "anthropic/" + config.ClaudeSonnetModel + ":medium"
	builtinAstra  = "openai-codex/" + config.CodexAstraModel + ":max"
	builtinSol    = "openai-codex/" + config.CodexSolModel + ":xhigh"
)

// TestOMPModelIntegration_S5_UltraBuiltinProjectsEachAgentTier proves the
// derived ultra profile keeps every agent on its own preset rung: sharing a
// capability with a fable agent no longer promotes an opus agent.
func TestOMPModelIntegration_S5_UltraBuiltinProjectsEachAgentTier(t *testing.T) {
	t.Parallel()

	integration := prepareBuiltinIntegration(t, "ultra")
	assert.Equal(t, "ultra", integration.profileName)
	assert.Equal(t, config.RoleModelCatalogTrustOperatorAttested, integration.profile.CatalogTrust)
	assert.Equal(t, []string{"autopus_reviewer", "autopus_security_auditor"},
		integration.profile.FamilyDiversity.Roles)

	assert.Equal(t, map[string]string{
		"autopus_annotator": builtinOpus, "autopus_architect": builtinFable,
		"autopus_debugger": builtinFable, "autopus_deep_worker": builtinFable,
		"autopus_devops": builtinOpus, "autopus_executor": builtinOpus,
		"autopus_explorer": builtinOpus, "autopus_frontend_specialist": builtinOpus,
		"autopus_perf_engineer": builtinOpus, "autopus_planner": builtinFable,
		"autopus_reviewer": builtinAstra, "autopus_security_auditor": builtinAstra,
		"autopus_spec_writer": builtinFable, "autopus_tester": builtinOpus,
		"autopus_ux_validator": builtinOpus, "autopus_validator": builtinOpus,
	}, builtinSelectorsByRole(integration.projection))
}

func TestOMPModelIntegration_S5_BalancedBuiltinKeepsSiblingTiersApart(t *testing.T) {
	t.Parallel()

	selectors := builtinSelectorsByRole(prepareBuiltinIntegration(t, "balanced").projection)
	require.Len(t, selectors, len(config.CanonicalAgentNames()))
	// executor/tester and reviewer/security-auditor share a capability but
	// sit on different preset rungs; each keeps its own.
	assert.Equal(t, builtinOpus, selectors["autopus_executor"])
	assert.Equal(t, builtinSonnet, selectors["autopus_tester"])
	assert.Equal(t, builtinSol, selectors["autopus_reviewer"])
	assert.Equal(t, builtinAstra, selectors["autopus_security_auditor"])
	assert.Equal(t, builtinFable, selectors["autopus_planner"])
	assert.Equal(t, builtinOpus, selectors["autopus_spec_writer"])
}

// TestOMPModelIntegration_S6_ProjectsAgentRolesOnly proves the rendered
// surfaces carry exactly the sixteen agent roles and no OMP native role key.
func TestOMPModelIntegration_S6_ProjectsAgentRolesOnly(t *testing.T) {
	t.Parallel()

	integration := prepareBuiltinIntegration(t, "ultra")
	overlay, err := OMPModelOverlayFromProjection(integration.projection)
	require.NoError(t, err)

	keys := make([]string, 0, len(overlay.ModelRoles))
	for role := range overlay.ModelRoles {
		keys = append(keys, role)
	}
	sort.Strings(keys)
	want := make([]string, 0, len(config.CanonicalAgentNames()))
	for _, agent := range config.CanonicalAgentNames() {
		want = append(want, config.OMPAgentRoleName(agent))
	}
	assert.Equal(t, want, keys)
	for _, native := range []string{
		"default", "smol", "slow", "plan", "vision", "designer", "commit", "tiny", "task", "advisor",
	} {
		assert.NotContains(t, overlay.ModelRoles, native)
	}
	assert.Equal(t, map[string][]string{
		builtinFable: {builtinOpus},
		builtinOpus:  {builtinSonnet},
		builtinAstra: {builtinSol},
	}, overlay.FallbackChains)

	byPath := integrationMappingsByPath(integration.agents)
	assert.Contains(t, string(byPath[".omp/agents/executor.md"].Content),
		"model: '@autopus_executor'\nthinking: xhigh\n")
	assert.Contains(t, string(byPath[".omp/agents/debugger.md"].Content),
		"model: '@autopus_debugger'\nthinking: max\n")
	assert.Contains(t, string(byPath[".omp/agents/reviewer.md"].Content),
		"model: '@autopus_reviewer'\nthinking: max\n")
}

// TestOMPModelIntegration_S7_ReceiptRowsAreKeyedByAgent proves the receipt
// carries one operator-attested row per agent with the agent's own role.
func TestOMPModelIntegration_S7_ReceiptRowsAreKeyedByAgent(t *testing.T) {
	t.Parallel()

	files, err := NewWithRoot(t.TempDir()).
		WithModelIntegrationRunner(newBuiltinTierIntegrationRunner()).
		prepareFiles(context.Background(), builtinIntegrationConfig("ultra"))
	require.NoError(t, err)

	var receipt OMPModelResolutionReceipt
	require.NoError(t, json.Unmarshal(
		integrationMappingsByPath(files)[OMPModelReceiptRelativePath].Content, &receipt))
	require.Len(t, receipt.Roles, len(config.CanonicalAgentNames()))
	byAgent := make(map[string]OMPModelRoleReceipt, len(receipt.Roles))
	for _, role := range receipt.Roles {
		capability, capabilityErr := config.OMPAgentCapability(role.Agent)
		require.NoError(t, capabilityErr, role.Agent)
		assert.Equal(t, config.OMPAgentRoleName(role.Agent), role.RequestedRole, role.Agent)
		assert.Equal(t, role.RequestedRole, role.EffectiveRole, role.Agent)
		assert.Equal(t, capability, role.Capability, role.Agent)
		assert.Equal(t, "operator_attested", role.EvidenceClass, role.Agent)
		byAgent[role.Agent] = role
	}
	assert.Equal(t, "anthropic/"+config.ClaudeOpusModel, byAgent["executor"].Selector)
	assert.Equal(t, "xhigh", byAgent["executor"].Thinking)
	assert.Equal(t, "anthropic/"+config.ClaudeFableModel, byAgent["debugger"].Selector)
	assert.Equal(t, "max", byAgent["debugger"].Thinking)
	assert.Equal(t, "satisfied", byAgent["reviewer"].FamilyDiversity.Status)
	assert.Equal(t, "not_applicable", byAgent["executor"].FamilyDiversity.Status)
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

func builtinIntegrationConfig(preset string) *config.HarnessConfig {
	cfg := config.DefaultFullConfig("builtin-omp-" + preset)
	cfg.Platforms = []string{"omp"}
	cfg.Quality.Default = preset
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1,
		Profile: preset,
		Family:  "anthropic",
	}
	return cfg
}

func prepareBuiltinIntegration(t *testing.T, preset string) *ompModelIntegration {
	t.Helper()
	cfg := builtinIntegrationConfig(preset)
	require.NoError(t, cfg.Validate())
	integration, err := NewWithRoot(t.TempDir()).
		WithModelIntegrationRunner(newBuiltinTierIntegrationRunner()).
		prepareModelIntegration(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, integration)
	return integration
}

func builtinSelectorsByRole(projection OMPModelProjection) map[string]string {
	selectors := make(map[string]string, len(projection.ModelRoles))
	for _, role := range projection.ModelRoles {
		selectors[role.Role] = role.Selector
	}
	return selectors
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
