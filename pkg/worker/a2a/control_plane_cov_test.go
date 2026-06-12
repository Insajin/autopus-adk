package a2a

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedControlPlaneEnforced_EnvDriven(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "")
	assert.False(t, SignedControlPlaneEnforced())
	t.Setenv(PolicySigningSecretEnv, "shhh")
	assert.True(t, SignedControlPlaneEnforced())
}

func TestHasIterationBudget(t *testing.T) {
	t.Parallel()
	assert.False(t, hasIterationBudget(nil))
	assert.False(t, hasIterationBudget(&IterationBudget{Limit: 0}))
	assert.True(t, hasIterationBudget(&IterationBudget{Limit: 5}))
}

func TestHasCapability(t *testing.T) {
	t.Parallel()
	caps := []string{CapabilityServerModelV1, CapabilityIterationBudgetV1}
	assert.True(t, hasCapability(caps, CapabilityServerModelV1))
	assert.False(t, hasCapability(caps, CapabilitySignedPolicyV1))
	assert.False(t, hasCapability(nil, CapabilityServerModelV1))
}

func TestCloneIterationBudget(t *testing.T) {
	t.Parallel()
	assert.Nil(t, cloneIterationBudget(nil))
	src := &IterationBudget{Limit: 7, WarnThreshold: 0.5}
	clone := cloneIterationBudget(src)
	require.NotNil(t, clone)
	assert.Equal(t, 7, clone.Limit)
	assert.Equal(t, 0.5, clone.WarnThreshold)
	// Mutating the clone does not affect the source.
	clone.Limit = 99
	assert.Equal(t, 7, src.Limit)
}

func TestApplyControlPlaneCapabilities_EmptyPassesThrough(t *testing.T) {
	t.Parallel()
	budget := &IterationBudget{Limit: 3}
	model, phases, instr, prompts, gotBudget := applyControlPlaneCapabilities(
		"  gpt-x  ",
		[]string{"plan", "build"},
		map[string]string{"plan": "do"},
		map[string]string{"build": "tmpl"},
		budget,
		nil, // empty capabilities -> pass-through
	)
	assert.Equal(t, "gpt-x", model)
	assert.Equal(t, []string{"plan", "build"}, phases)
	assert.Equal(t, map[string]string{"plan": "do"}, instr)
	assert.Equal(t, map[string]string{"build": "tmpl"}, prompts)
	require.NotNil(t, gotBudget)
	assert.Equal(t, 3, gotBudget.Limit)
}

func TestApplyControlPlaneCapabilities_FiltersUnauthorized(t *testing.T) {
	t.Parallel()
	// Only the server-model capability is granted; everything else is dropped.
	model, phases, instr, prompts, gotBudget := applyControlPlaneCapabilities(
		"gpt-x",
		[]string{"plan"},
		map[string]string{"plan": "do"},
		map[string]string{"plan": "tmpl"},
		&IterationBudget{Limit: 3},
		[]string{CapabilityServerModelV1},
	)
	assert.Equal(t, "gpt-x", model)
	assert.Nil(t, phases)
	assert.Nil(t, instr)
	assert.Nil(t, prompts)
	assert.Nil(t, gotBudget)
}

func TestParamsFromRawPolledTask_CopiesFields(t *testing.T) {
	t.Parallel()
	task := PollResult{
		ID:                       "task-1",
		Model:                    "gpt-x",
		PipelinePhases:           []string{"plan"},
		PipelineInstructions:     map[string]string{"plan": "do"},
		PipelinePromptTemplates:  map[string]string{"plan": "tmpl"},
		IterationBudget:          &IterationBudget{Limit: 4},
		ControlPlaneCapabilities: []string{CapabilityServerModelV1},
		ControlPlaneSignature:    "cp-sig",
		PolicySignature:          "pol-sig",
		Payload:                  json.RawMessage(`{"k":"v"}`),
	}
	params := paramsFromRawPolledTask(task)
	assert.Equal(t, "task-1", params.TaskID)
	assert.Equal(t, "gpt-x", params.Model)
	assert.Equal(t, []string{"plan"}, params.PipelinePhases)
	assert.Equal(t, map[string]string{"plan": "do"}, params.PipelineInstructions)
	assert.Equal(t, "cp-sig", params.ControlPlaneSignature)
	assert.Equal(t, "pol-sig", params.PolicySignature)
	require.NotNil(t, params.IterationBudget)
	assert.Equal(t, 4, params.IterationBudget.Limit)
	assert.JSONEq(t, `{"k":"v"}`, string(params.Payload))
}

func TestMarshalJSON_Success(t *testing.T) {
	t.Parallel()
	raw, err := marshalJSON(map[string]int{"a": 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(raw))
}

func TestRemoveIfExists_NoError(t *testing.T) {
	t.Parallel()
	// Non-existent path is a no-op (no panic, no error surfaced).
	assert.NotPanics(t, func() {
		removeIfExists(filepath.Join(t.TempDir(), "ghost.json"))
	})
}

func TestCloneStringMap_EmptyAndCopy(t *testing.T) {
	t.Parallel()
	assert.Nil(t, cloneStringMap(nil))
	assert.Nil(t, cloneStringMap(map[string]string{}))
	src := map[string]string{"a": "1"}
	clone := cloneStringMap(src)
	assert.Equal(t, src, clone)
	clone["a"] = "2"
	assert.Equal(t, "1", src["a"])
}
