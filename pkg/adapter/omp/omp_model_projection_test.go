package omp

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileOMPModelProjection_S2_ProjectsOneRolePerAgent(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)

	const (
		coder    = "openai/beta-coder:high"
		reasoner = "anthropic/alpha-reasoner:xhigh"
		dissent  = "anthropic/alpha-reasoner:high"
		vision   = "google/gamma-vision:high"
	)
	assert.Equal(t, []OMPModelRoleProjection{
		{Role: "autopus_annotator", Capability: "fast_validation", Selector: coder},
		{Role: "autopus_architect", Capability: "deep_reasoning", Selector: reasoner},
		{Role: "autopus_debugger", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_deep_worker", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_devops", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_executor", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_explorer", Capability: "fast_validation", Selector: coder},
		{Role: "autopus_frontend_specialist", Capability: "vision_design", Selector: vision},
		{Role: "autopus_perf_engineer", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_planner", Capability: "deep_reasoning", Selector: reasoner},
		{Role: "autopus_reviewer", Capability: "independent_dissent", Selector: dissent},
		{Role: "autopus_security_auditor", Capability: "independent_dissent", Selector: dissent},
		{Role: "autopus_spec_writer", Capability: "deep_reasoning", Selector: reasoner},
		{Role: "autopus_tester", Capability: "coding_tool_use", Selector: coder},
		{Role: "autopus_ux_validator", Capability: "vision_design", Selector: vision},
		{Role: "autopus_validator", Capability: "deterministic_transform", Selector: coder},
	}, projection.ModelRoles)

	require.Len(t, projection.Agents, 16)
	for index, agent := range projection.Agents {
		role := projection.ModelRoles[index]
		assert.Equal(t, config.CanonicalAgentNames()[index], agent.Agent)
		assert.Equal(t, role.Role, agent.Role, agent.Agent)
		assert.Equal(t, "@"+role.Role, agent.Model, agent.Agent)
		assert.Equal(t, role.Selector, agent.EffectiveSelector, agent.Agent)
		assert.True(t, strings.HasSuffix(role.Selector, ":"+agent.Thinking), agent.Agent)
	}
	assert.Equal(t, []OMPFallbackChainProjection{
		{Selector: reasoner, Candidates: []string{"openai/beta-coder:high", "anthropic/omega-reasoner:high"}},
	}, projection.FallbackChains)
}

func TestCompileOMPModelProjection_UnresolvedAgentsInheritRuntimeDefaults(t *testing.T) {
	t.Parallel()

	input := ompProjectionFixture(t)
	input.Agents = ompProjectionWithoutAgents(input.Agents, "reviewer", "security-auditor")
	projection, err := CompileOMPModelProjection(input)
	require.NoError(t, err)

	require.Len(t, projection.ModelRoles, 14)
	for _, role := range projection.ModelRoles {
		assert.NotEqual(t, "autopus_reviewer", role.Role)
		assert.NotEqual(t, "autopus_security_auditor", role.Role)
	}
	for _, agent := range projection.Agents {
		assert.NotEqual(t, "reviewer", agent.Agent)
		assert.NotEqual(t, "security-auditor", agent.Agent)
	}
	mappings, err := NewWithRoot(t.TempDir()).prepareAgentMappingsWithProjection(projection)
	require.NoError(t, err)
	require.Len(t, mappings, 16)
	for _, mapping := range mappings {
		if strings.HasSuffix(mapping.TargetPath, "/reviewer.md") ||
			strings.HasSuffix(mapping.TargetPath, "/security-auditor.md") {
			assert.NotContains(t, string(mapping.Content), "model:")
			assert.NotContains(t, string(mapping.Content), "thinking:")
		}
	}
}

func TestCompileOMPModelProjection_AcceptsNativeThinkingLevels(t *testing.T) {
	t.Parallel()

	for _, thinking := range []string{"off", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto"} {
		input := ompProjectionFixture(t)
		input.Agents[0].Thinking = thinking
		projection, err := CompileOMPModelProjection(input)
		require.NoError(t, err, thinking)
		assert.Equal(t, "openai/beta-coder:"+thinking, projection.ModelRoles[0].Selector)
		assert.Equal(t, thinking, projection.Agents[0].Thinking)
	}
}

func TestOMPModelOverlayFromProjection_S10_ReturnsDetachedCanonicalMaps(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)
	overlay, err := OMPModelOverlayFromProjection(projection)
	require.NoError(t, err)
	assert.Equal(t, "anthropic/alpha-reasoner:xhigh", overlay.ModelRoles["autopus_planner"])
	assert.Equal(t, []string{"openai/beta-coder:high", "anthropic/omega-reasoner:high"},
		overlay.FallbackChains["anthropic/alpha-reasoner:xhigh"])

	overlay.ModelRoles["autopus_planner"] = "mutated"
	overlay.FallbackChains["anthropic/alpha-reasoner:xhigh"][0] = "mutated"
	assert.Equal(t, "anthropic/alpha-reasoner:xhigh", projection.ModelRoles[9].Selector)
	assert.Equal(t, "openai/beta-coder:high", projection.FallbackChains[0].Candidates[0])
}

func TestCompileOMPModelProjection_S2_RejectsUnmappedAgent(t *testing.T) {
	t.Parallel()

	input := ompProjectionFixture(t)
	input.Agents = append(input.Agents, OMPProjectionAgent{
		Agent: "future-agent", Role: config.OMPAgentRoleName("future-agent"),
		Capability: config.CapabilityCodingToolUse, Selector: "openai/beta-coder", Thinking: "high",
	})
	_, err := CompileOMPModelProjection(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent_role_unmapped")
}

func TestCompileOMPModelProjection_S17_IsDeterministicAcrossInputOrder(t *testing.T) {
	t.Parallel()

	want, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)

	for seed := uint64(0); seed < 100; seed++ {
		shuffled := ompProjectionFixture(t)
		rng := rand.New(rand.NewPCG(seed, 0)) // #nosec G404 -- deterministic test permutation only.
		rng.Shuffle(len(shuffled.Agents), func(i, j int) {
			shuffled.Agents[i], shuffled.Agents[j] = shuffled.Agents[j], shuffled.Agents[i]
		})
		got, compileErr := CompileOMPModelProjection(shuffled)
		require.NoError(t, compileErr)
		assert.Equal(t, want, got, "seed=%d", seed)
	}
}

func TestCompileOMPModelProjection_SecurityRejectsRawTierAndInjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		selector string
		thinking string
	}{
		{name: "raw opus", selector: "opus", thinking: "high"},
		{name: "raw sonnet", selector: "sonnet", thinking: "high"},
		{name: "raw haiku", selector: "haiku", thinking: "low"},
		{name: "selector newline", selector: "openai/beta\nmodel: evil", thinking: "high"},
		{name: "thinking newline", selector: "openai/beta-coder", thinking: "high\ntools: [bash]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ompProjectionFixture(t)
			input.Agents[0].Selector = tt.selector
			input.Agents[0].Thinking = tt.thinking
			_, err := CompileOMPModelProjection(input)
			require.Error(t, err)
		})
	}
}

func TestPrepareAgentMappingsWithProjection_S2_RendersRoleAndThinking(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)
	mappings, err := NewWithRoot(t.TempDir()).prepareAgentMappingsWithProjection(projection)
	require.NoError(t, err)
	require.Len(t, mappings, 16)

	byAgent := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		name := strings.TrimSuffix(strings.TrimPrefix(mapping.TargetPath, ".omp/agents/"), ".md")
		byAgent[name] = string(mapping.Content)
	}
	assert.Contains(t, byAgent["planner"], "model: '@autopus_planner'\nthinking: xhigh\n")
	assert.Contains(t, byAgent["executor"], "model: '@autopus_executor'\nthinking: high\n")
	assert.Contains(t, byAgent["deep-worker"], "model: '@autopus_deep_worker'\nthinking: high\n")
	assert.NotContains(t, byAgent["planner"], "model: opus")
	assert.NotContains(t, byAgent["executor"], "model: sonnet")
}

func TestPrepareAgentMappingsWithProjection_RejectsTupleMismatch(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)
	projection.Agents[0].EffectiveSelector = "openai/different-model:high"
	_, err = NewWithRoot(t.TempDir()).prepareAgentMappingsWithProjection(projection)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent_projection_mismatch")
}

// ompProjectionFixture resolves every canonical agent through a
// capability-keyed selection table, in canonical agent order.
func ompProjectionFixture(t *testing.T) OMPModelProjectionInput {
	t.Helper()
	byCapability := map[string]OMPProjectionAgent{
		config.CapabilityCodingToolUse: {Selector: "openai/beta-coder", Thinking: "high"},
		config.CapabilityDeepReasoning: {Selector: "anthropic/alpha-reasoner", Thinking: "xhigh", Fallbacks: []OMPProjectionCandidate{
			{Selector: "openai/beta-coder", Thinking: "high"},
			{Selector: "anthropic/omega-reasoner", Thinking: "high"},
		}},
		config.CapabilityFastValidation:         {Selector: "openai/beta-coder", Thinking: "high"},
		config.CapabilityVisionDesign:           {Selector: "google/gamma-vision", Thinking: "high"},
		config.CapabilityIndependentDissent:     {Selector: "anthropic/alpha-reasoner", Thinking: "high"},
		config.CapabilityDeterministicTransform: {Selector: "openai/beta-coder", Thinking: "high"},
	}
	names := config.CanonicalAgentNames()
	agents := make([]OMPProjectionAgent, 0, len(names))
	for _, name := range names {
		capability, err := config.OMPAgentCapability(name)
		require.NoError(t, err)
		entry := byCapability[capability]
		entry.Agent, entry.Role, entry.Capability = name, config.OMPAgentRoleName(name), capability
		entry.Fallbacks = append([]OMPProjectionCandidate(nil), entry.Fallbacks...)
		agents = append(agents, entry)
	}
	return OMPModelProjectionInput{Agents: agents}
}

func ompProjectionWithoutAgents(agents []OMPProjectionAgent, names ...string) []OMPProjectionAgent {
	excluded := make(map[string]struct{}, len(names))
	for _, name := range names {
		excluded[name] = struct{}{}
	}
	kept := make([]OMPProjectionAgent, 0, len(agents))
	for _, agent := range agents {
		if _, skip := excluded[agent.Agent]; !skip {
			kept = append(kept, agent)
		}
	}
	return kept
}
