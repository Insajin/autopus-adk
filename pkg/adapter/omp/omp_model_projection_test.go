package omp

import (
	"math/rand"
	"strings"
	"testing"

	contentfs "github.com/insajin/autopus-adk/content"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileOMPModelProjection_S2_ProjectsCanonicalRolesAndAgents(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)

	assert.Equal(t, []OMPModelRoleProjection{
		{Role: "default", Capability: "coding_tool_use", Selector: "openai/beta-coder:high"},
		{Role: "smol", Capability: "fast_validation", Selector: "openai/beta-coder:high"},
		{Role: "slow", Capability: "deep_reasoning", Selector: "anthropic/alpha-reasoner:xhigh"},
		{Role: "plan", Capability: "deep_reasoning", Selector: "anthropic/alpha-reasoner:xhigh"},
		{Role: "vision", Capability: "vision_design", Selector: "google/gamma-vision:high"},
		{Role: "designer", Capability: "vision_design", Selector: "google/gamma-vision:high"},
		{Role: "commit", Capability: "deterministic_transform", Selector: "openai/beta-coder:high"},
		{Role: "tiny", Capability: "deterministic_transform", Selector: "openai/beta-coder:high"},
		{Role: "task", Capability: "coding_tool_use", Selector: "openai/beta-coder:high"},
		{Role: "advisor", Capability: "independent_dissent", Selector: "anthropic/alpha-reasoner:high"},
	}, projection.ModelRoles)

	wantAgents := []OMPAgentModelProjection{
		{Agent: "annotator", Role: "smol", Model: "@smol", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "architect", Role: "slow", Model: "@slow", Thinking: "xhigh", EffectiveSelector: "anthropic/alpha-reasoner:xhigh"},
		{Agent: "debugger", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "deep-worker", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "devops", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "executor", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "explorer", Role: "smol", Model: "@smol", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "frontend-specialist", Role: "designer", Model: "@designer", Thinking: "high", EffectiveSelector: "google/gamma-vision:high"},
		{Agent: "perf-engineer", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "planner", Role: "plan", Model: "@plan", Thinking: "xhigh", EffectiveSelector: "anthropic/alpha-reasoner:xhigh"},
		{Agent: "reviewer", Role: "advisor", Model: "@advisor", Thinking: "high", EffectiveSelector: "anthropic/alpha-reasoner:high"},
		{Agent: "security-auditor", Role: "advisor", Model: "@advisor", Thinking: "high", EffectiveSelector: "anthropic/alpha-reasoner:high"},
		{Agent: "spec-writer", Role: "plan", Model: "@plan", Thinking: "xhigh", EffectiveSelector: "anthropic/alpha-reasoner:xhigh"},
		{Agent: "tester", Role: "task", Model: "@task", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
		{Agent: "ux-validator", Role: "vision", Model: "@vision", Thinking: "high", EffectiveSelector: "google/gamma-vision:high"},
		{Agent: "validator", Role: "tiny", Model: "@tiny", Thinking: "high", EffectiveSelector: "openai/beta-coder:high"},
	}
	assert.Equal(t, wantAgents, projection.Agents)
	assert.Equal(t, []OMPFallbackChainProjection{
		{Selector: "anthropic/alpha-reasoner:xhigh", Candidates: []string{
			"openai/beta-coder:high", "anthropic/omega-reasoner:high",
		}},
	}, projection.FallbackChains)
}

func TestOMPModelOverlayFromProjection_S10_ReturnsDetachedCanonicalMaps(t *testing.T) {
	t.Parallel()

	projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
	require.NoError(t, err)
	overlay, err := OMPModelOverlayFromProjection(projection)
	require.NoError(t, err)
	assert.Equal(t, "anthropic/alpha-reasoner:xhigh", overlay.ModelRoles["plan"])
	assert.Equal(t, []string{"openai/beta-coder:high", "anthropic/omega-reasoner:high"},
		overlay.FallbackChains["anthropic/alpha-reasoner:xhigh"])

	overlay.ModelRoles["plan"] = "mutated"
	overlay.FallbackChains["anthropic/alpha-reasoner:xhigh"][0] = "mutated"
	assert.Equal(t, "anthropic/alpha-reasoner:xhigh", projection.ModelRoles[3].Selector)
	assert.Equal(t, "openai/beta-coder:high", projection.FallbackChains[0].Candidates[0])
}

func TestCompileOMPModelProjection_S2_RejectsUnmappedAgent(t *testing.T) {
	t.Parallel()

	input := ompProjectionFixture(t)
	input.AgentNames = append(input.AgentNames, "future-agent")
	_, err := CompileOMPModelProjection(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent_role_unmapped")
}

func TestCompileOMPModelProjection_S17_IsDeterministicAcrossInputOrder(t *testing.T) {
	t.Parallel()

	input := ompProjectionFixture(t)
	want, err := CompileOMPModelProjection(input)
	require.NoError(t, err)

	for seed := int64(0); seed < 100; seed++ {
		shuffled := ompProjectionFixture(t)
		rng := rand.New(rand.NewSource(seed)) // #nosec G404 -- deterministic test permutation only.
		rng.Shuffle(len(shuffled.Capabilities), func(i, j int) {
			shuffled.Capabilities[i], shuffled.Capabilities[j] = shuffled.Capabilities[j], shuffled.Capabilities[i]
		})
		rng.Shuffle(len(shuffled.AgentNames), func(i, j int) {
			shuffled.AgentNames[i], shuffled.AgentNames[j] = shuffled.AgentNames[j], shuffled.AgentNames[i]
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
			input.Capabilities[0].Selector = tt.selector
			input.Capabilities[0].Thinking = tt.thinking
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
	assert.Contains(t, byAgent["planner"], "model: '@plan'\nthinking: xhigh\n")
	assert.Contains(t, byAgent["executor"], "model: '@task'\nthinking: high\n")
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

func ompProjectionFixture(t *testing.T) OMPModelProjectionInput {
	t.Helper()
	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	require.NoError(t, err)
	agents := make([]string, 0, len(sources))
	for _, source := range sources {
		agents = append(agents, source.Meta.Name)
	}
	return OMPModelProjectionInput{
		AgentNames: agents,
		Capabilities: []OMPProjectionCapability{
			{Capability: "coding_tool_use", Selector: "openai/beta-coder", Thinking: "high"},
			{Capability: "deep_reasoning", Selector: "anthropic/alpha-reasoner", Thinking: "xhigh", Fallbacks: []OMPProjectionCandidate{
				{Selector: "openai/beta-coder", Thinking: "high"},
				{Selector: "anthropic/omega-reasoner", Thinking: "high"},
			}},
			{Capability: "fast_validation", Selector: "openai/beta-coder", Thinking: "high"},
			{Capability: "vision_design", Selector: "google/gamma-vision", Thinking: "high"},
			{Capability: "independent_dissent", Selector: "anthropic/alpha-reasoner", Thinking: "high"},
			{Capability: "deterministic_transform", Selector: "openai/beta-coder", Thinking: "high"},
		},
	}
}
