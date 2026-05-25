package delivery

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliveryValidatePhaseResultRejectsUnknownPhaseAndInvalidStatus(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.Phase = "invent"
	envelope.Status = "done"
	envelope.RedactionStatus = "leaky"

	err := ValidatePhaseResultEnvelope(envelope)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown phase")
	assert.Contains(t, err.Error(), "invalid status")
	assert.Contains(t, err.Error(), "invalid redaction_status")
}

func TestDeliveryValidatePhaseResultRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"schema_version":"codeops.phase_result.v1",
		"request_id":"REQ-1",
		"phase":"plan",
		"status":"passed",
		"summary":"planned",
		"changed_files":[],
		"evidence_refs":[],
		"blockers":[],
		"next_required_action":"continue",
		"redaction_status":"clean"
	}`)

	_, err := ValidatePhaseResultJSON(data)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "test_status is required")
}

func TestDeliveryValidatePhaseResultRejectsGeneratedRuntimeDrift(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.ChangedFiles = []string{
		"pkg/source.go",
		".codex/agents/executor.md",
		".autopus/context/signatures.md",
		"config.toml",
	}

	err := ValidatePhaseResultEnvelope(envelope)

	require.Error(t, err)
	assert.Contains(t, err.Error(), ".codex/agents/executor.md")
	assert.Contains(t, err.Error(), ".autopus/context/signatures.md")
	assert.Contains(t, err.Error(), "config.toml")
}

func TestDeliveryValidatePhaseResultRejectsBlockedRedactionSuccessPath(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.RedactionStatus = RedactionBlocked

	err := ValidatePhaseResultEnvelope(envelope)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redaction_status blocked")
}

func TestDeliveryDryRunPlanUsesCanonicalPhaseOrder(t *testing.T) {
	t.Parallel()

	plan, err := BuildDryRunPlan(PlanOptions{
		Repository:   "autopus-adk",
		ProviderMode: ProviderCodexSubscriptionInteractive,
	})

	require.NoError(t, err)
	require.Len(t, plan.Phases, 7)
	for i, phase := range CanonicalPhases() {
		assert.Equal(t, phase, plan.Phases[i].Phase)
		assert.Equal(t, WorkflowMode, plan.Phases[i].WorkflowMode)
		assert.Equal(t, ProviderCodexSubscriptionInteractive, plan.Phases[i].ProviderMode)
		assert.Equal(t, DefaultProviderRoute, plan.Phases[i].ProviderRoute)
		assert.Equal(t, GateDecisionSchemaV1, plan.Phases[i].GateDecision.SchemaVersion)
	}
	assert.True(t, plan.Phases[2].GateDecision.BlocksOnDrift)
	assert.True(t, plan.Phases[4].GateDecision.BlocksOnDrift)
}

func TestDeliveryDryRunPlanRejectsUnknownProviderMode(t *testing.T) {
	t.Parallel()

	_, err := BuildDryRunPlan(PlanOptions{ProviderMode: "local_shell"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider mode")
}

func TestDeliveryDryRunPlanDefaultsAndAPIProviderRoute(t *testing.T) {
	t.Parallel()

	plan, err := BuildDryRunPlan(PlanOptions{ProviderMode: ProviderClaudeAPIHeadless})

	require.NoError(t, err)
	require.NotEmpty(t, plan.Phases)
	assert.Equal(t, ".", plan.Repository)
	assert.Equal(t, "headless_api", plan.Phases[0].ProviderRoute)
	assert.Equal(t, DefaultOwnerAgentID, plan.Phases[0].OwnerAgentID)
	assert.Equal(t, DefaultCorrelationID, plan.Phases[0].CorrelationID)
}

func TestDeliveryGeneratedRuntimeDenyPatternsAreSortedCopies(t *testing.T) {
	t.Parallel()

	patterns := GeneratedRuntimeDenyPatterns()
	require.NotEmpty(t, patterns)
	patterns[0] = "mutated"

	assert.NotEqual(t, "mutated", GeneratedRuntimeDenyPatterns()[0])
}

func TestDeliveryGeneratedRuntimePathMatching(t *testing.T) {
	t.Parallel()

	assert.True(t, IsGeneratedRuntimePath("./.autopus/foo-manifest.json"))
	assert.True(t, IsGeneratedRuntimePath(".autopus/runtime/session.json"))
	assert.True(t, IsGeneratedRuntimePath(".agents/plugins/marketplace.json"))
	assert.False(t, IsGeneratedRuntimePath("autopus-adk/pkg/delivery/types.go"))
	assert.NoError(t, ValidateNoGeneratedRuntimeDrift([]string{"autopus-adk/pkg/delivery/types.go"}))
	require.Error(t, ValidateNoGeneratedRuntimeDrift([]string{".claude/settings.json"}))
}

func TestDeliveryValidatePhaseResultAcceptsValidEnvelope(t *testing.T) {
	t.Parallel()

	err := ValidatePhaseResultEnvelope(validEnvelope())

	require.NoError(t, err)
}

func TestDeliveryValidatePhaseResultJSONRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(validEnvelope())
	require.NoError(t, err)
	payload = append(payload[:len(payload)-1], []byte(`,"extra":"nope"}`)...)

	_, err = ValidatePhaseResultJSON(payload)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestDeliveryValidatePhaseResultJSONRejectsTrailingValue(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(validEnvelope())
	require.NoError(t, err)
	payload = append(payload, []byte(`{}`)...)

	_, err = ValidatePhaseResultJSON(payload)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing JSON value")
}

func TestDeliveryValidatePhaseResultJSONRejectsTrailingGarbage(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(validEnvelope())
	require.NoError(t, err)
	payload = append(payload, []byte(` nope`)...)

	_, err = ValidatePhaseResultJSON(payload)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing JSON value")
}

func validEnvelope() PhaseResultEnvelope {
	return PhaseResultEnvelope{
		SchemaVersion:      PhaseResultSchemaV1,
		RequestID:          "REQ-1",
		Phase:              PhasePlan,
		Status:             StatusPassed,
		Summary:            "phase completed",
		ChangedFiles:       []string{"pkg/source.go"},
		TestStatus:         "passed",
		EvidenceRefs:       []string{"evidence://phase/REQ-1"},
		Blockers:           []string{},
		NextRequiredAction: "continue",
		RedactionStatus:    RedactionClean,
	}
}
