package omp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileOMPModelProjection_InvalidContractsFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		edit func(*OMPModelProjectionInput)
	}{
		{
			name: "unknown capability", code: "capability_unknown",
			edit: func(input *OMPModelProjectionInput) { input.Capabilities[0].Capability = "unknown" },
		},
		{
			name: "duplicate capability", code: "capability_duplicate",
			edit: func(input *OMPModelProjectionInput) {
				input.Capabilities = append(input.Capabilities, input.Capabilities[0])
			},
		},
		{
			name: "invalid fallback", code: "selector_invalid",
			edit: func(input *OMPModelProjectionInput) {
				input.Capabilities[1].Fallbacks[0].Selector = "sonnet"
			},
		},
		{
			name: "duplicate agent", code: "agent_role_duplicate",
			edit: func(input *OMPModelProjectionInput) {
				input.AgentNames = append(input.AgentNames, input.AgentNames[0])
			},
		},
		{
			name: "missing agent", code: "agent_role_missing",
			edit: func(input *OMPModelProjectionInput) { input.AgentNames = input.AgentNames[1:] },
		},
		{
			name: "conflicting selector chain", code: "fallback_chain_conflict",
			edit: func(input *OMPModelProjectionInput) {
				input.Capabilities[0].Selector = "anthropic/alpha-reasoner"
				input.Capabilities[0].Thinking = "xhigh"
				input.Capabilities[0].Fallbacks = []OMPProjectionCandidate{
					{Selector: "google/gamma-vision", Thinking: "high"},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := ompProjectionFixture(t)
			tc.edit(&input)
			_, err := CompileOMPModelProjection(input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.code)
		})
	}
}

func TestOMPModelOverlayFromProjection_InvalidCompiledShapeFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		edit func(*OMPModelProjection)
	}{
		{
			name: "role order", code: "model_role_order_mismatch",
			edit: func(projection *OMPModelProjection) {
				projection.ModelRoles[0], projection.ModelRoles[1] = projection.ModelRoles[1], projection.ModelRoles[0]
			},
		},
		{
			name: "role selector", code: "selector_invalid",
			edit: func(projection *OMPModelProjection) { projection.ModelRoles[0].Selector = "sonnet" },
		},
		{
			name: "fallback selector", code: "selector_invalid",
			edit: func(projection *OMPModelProjection) { projection.FallbackChains[0].Selector = "opus" },
		},
		{
			name: "fallback candidate", code: "selector_invalid",
			edit: func(projection *OMPModelProjection) { projection.FallbackChains[0].Candidates[0] = "haiku" },
		},
		{
			name: "duplicate chain", code: "fallback_chain_duplicate",
			edit: func(projection *OMPModelProjection) {
				projection.FallbackChains = append(projection.FallbackChains, projection.FallbackChains[0])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
			require.NoError(t, err)
			tc.edit(&projection)
			_, err = OMPModelOverlayFromProjection(projection)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.code)
		})
	}
}

func TestPrepareAgentMappingsWithProjection_InvalidAgentShapeFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		edit func(*OMPModelProjection)
	}{
		{
			name: "duplicate", code: "agent_duplicate",
			edit: func(projection *OMPModelProjection) {
				projection.Agents = append(projection.Agents, projection.Agents[0])
			},
		},
		{
			name: "unknown", code: "agent_role_unmapped",
			edit: func(projection *OMPModelProjection) { projection.Agents[0].Agent = "future-agent" },
		},
		{
			name: "wrong role", code: "role_capability_mismatch",
			edit: func(projection *OMPModelProjection) { projection.Agents[0].Role = "task" },
		},
		{
			name: "missing", code: "agent_role_set_mismatch",
			edit: func(projection *OMPModelProjection) { projection.Agents = projection.Agents[1:] },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projection, err := CompileOMPModelProjection(ompProjectionFixture(t))
			require.NoError(t, err)
			tc.edit(&projection)
			_, err = NewWithRoot(t.TempDir()).prepareAgentMappingsWithProjection(projection)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.code)
		})
	}
}
