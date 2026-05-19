package qualityloop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityLoop_QAMESHAdmissionDisablesUnsafeOrUnsupportedRepair(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  FailureInput
		reason string
		status string
	}{
		{
			name: "passed evidence is not a repair handoff",
			input: qameshInput(FailureInput{
				QAMESHStatus: "passed",
			}),
			reason: "qamesh_evidence_not_failed",
			status: StatusReplayFailed,
		},
		{
			name: "non deterministic evidence is not a repair handoff",
			input: qameshInput(FailureInput{
				EvidenceStrength: "non_deterministic",
			}),
			reason: "qamesh_non_deterministic",
			status: StatusReplayFailed,
		},
		{
			name: "redaction failure is quarantined",
			input: qameshInput(FailureInput{
				RedactionStatus: "failed",
			}),
			reason: "redaction_failed",
			status: StatusQuarantined,
		},
		{
			name: "unsafe path is quarantined",
			input: qameshInput(FailureInput{
				OwnedPaths: []string{"/Users/alice/private/Checkout.tsx"},
			}),
			reason: "unsafe_evidence_ref",
			status: StatusQuarantined,
		},
		{
			name: "unsafe source hash is quarantined",
			input: qameshInput(FailureInput{
				SourceHashes: []string{"workspace:ws-other:sha256:bad"},
			}),
			reason: "cross_workspace_ref",
			status: StatusQuarantined,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := NormalizeFailures([]FailureInput{tt.input})
			require.NoError(t, err)
			require.Len(t, result.Candidates, 1)
			got := result.Candidates[0]
			assert.Contains(t, got.ReasonCodes, tt.reason)
			assert.Equal(t, tt.status, got.Status)
			assert.False(t, got.RepairActionEnabled)
			assert.False(t, got.ApplyEnabled)
			assert.False(t, got.Verified)
		})
	}
}

func TestQualityLoop_RepeatedADKFailuresAggregateToQuarantinedSkillCandidate(t *testing.T) {
	t.Parallel()

	inputs := []FailureInput{
		repeatedADKInput("run-1", "sha256:one"),
		repeatedADKInput("run-2", "sha256:two"),
		repeatedADKInput("run-3", "sha256:three"),
	}

	result, err := NormalizeFailures(inputs)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	got := result.Candidates[0]

	assert.Equal(t, KindSkillEvolveCandidate, got.CandidateKind)
	assert.Equal(t, StatusQuarantined, got.Status)
	assert.False(t, got.Active)
	assert.Equal(t, []string{"agent_eval:run-1", "agent_eval:run-2", "agent_eval:run-3"}, got.SourceFailureRefs)
	assert.ElementsMatch(t, []string{"sha256:one", "sha256:two", "sha256:three"}, got.SourceHashes)
	assert.Contains(t, got.AffectedRefs, "autopus-adk/pkg/qa/oracle.go")
	assert.Contains(t, got.AffectedAcceptanceIDs, "AC-QIL-004")
	assert.NotEmpty(t, got.ProposedDigest)
	assert.NotEmpty(t, got.GenerationPromptDigest)
	assert.NotEmpty(t, got.ReplayPlan)
	assert.Equal(t, "passed", got.SafetyGate)
}

func TestQualityLoop_ReplayFailureAndApprovalGatesPreventUnsafeVerification(t *testing.T) {
	t.Parallel()

	base := ImprovementCandidate{
		CandidateID:        "ic-replay-gate",
		Status:             StatusAwaitingReplay,
		WorkspaceID:        "ws-quality",
		RecommendedRoute:   KindQAMESHRepairHandoff,
		FailureFingerprint: "qamesh.checkout.submit",
		MaxReplayAttempts:  2,
	}
	for _, signal := range []LifecycleSignal{
		{ReplayRunIndexMissing: true},
		{ReplayOutsideProject: true},
		{ReplayNonDeterministic: true},
		{ReplayMissingACMapping: true},
		{ReplayFreshness: "stale"},
		{ReplayStatus: "failed"},
	} {
		next := TransitionLifecycle(base, signal)
		assert.Equal(t, StatusReplayFailed, next.Status)
		assert.False(t, next.Active)
		assert.False(t, next.ApplyEnabled)
		assert.False(t, next.Verified)
	}

	unapproved := TransitionLifecycle(base, LifecycleSignal{
		ApplyCompleted:         true,
		OriginalBlockerCleared: true,
		PostApplyEvidenceRefs:  []string{"qamesh:passed"},
	})
	assert.Equal(t, StatusApprovalRequired, unapproved.Status)
	assert.False(t, unapproved.Verified)

	unsafeRef := TransitionLifecycle(base, LifecycleSignal{ReplayEvidenceRefs: []string{"/Users/alice/private/run-index.json"}})
	assert.Equal(t, StatusQuarantined, unsafeRef.Status)
	assert.Contains(t, unsafeRef.SafetyReasonCodes, "unsafe_evidence_ref")
}

func TestQualityLoop_ModelRoutingPolicyIsPlannedAndApprovalGated(t *testing.T) {
	t.Parallel()

	result, err := NormalizeFailures([]FailureInput{{
		SourceArtifactType:    "agent_role_evaluation_scorecard.v1",
		SourceID:              "model-route-001",
		WorkspaceID:           "ws-quality",
		FailureFingerprint:    "model.latency.quality.regression",
		ReasonCode:            "cost_latency_mismatch",
		DeterministicEvidence: true,
		EvidenceRefs:          []string{"workspace:ws-quality:model-route-001"},
	}})
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	got := result.Candidates[0]

	assert.Equal(t, KindModelRoutingPolicy, got.CandidateKind)
	assert.Contains(t, got.RouteMetadata, "[NEW] planned addition")
	assert.Equal(t, PolicyApprovalRequired, got.RepairActionPolicy)
	assert.False(t, got.ApplyEnabled)
	assert.False(t, got.Verified)
	assert.Contains(t, got.ApprovalGate, "model selection")
}

func qameshInput(overrides FailureInput) FailureInput {
	input := FailureInput{
		SourceArtifactType:    "qamesh.evidence.v2",
		SourceID:              "qa-run-001",
		WorkspaceID:           "ws-quality",
		FailureFingerprint:    "qamesh.checkout.submit",
		ReasonCode:            "qamesh_failed_check",
		DeterministicEvidence: true,
		QAMESHStatus:          "failed",
		RedactionStatus:       RedactionRedacted,
		EvidenceRefs:          []string{".autopus/qa/runs/qa-run-001/manifest.json"},
		OwnedPaths:            []string{"Autopus/frontend/src/components/Checkout.tsx"},
		AffectedAcceptanceIDs: []string{"AC-QIL-003"},
	}
	mergeFailureInput(&input, overrides)
	return input
}

func repeatedADKInput(sourceID, hash string) FailureInput {
	return FailureInput{
		SourceArtifactType:    "agent_eval",
		SourceID:              sourceID,
		WorkspaceID:           "ws-quality",
		FailureFingerprint:    "oracle.structural_only.missing_semantic_output",
		ReasonCode:            "oracle_structural_only",
		DeterministicEvidence: true,
		RedactionStatus:       RedactionRedacted,
		SourceHashes:          []string{hash},
		AffectedRefs:          []string{"autopus-adk/pkg/qa/oracle.go"},
		AffectedAcceptanceIDs: []string{"AC-QIL-004"},
	}
}

func mergeFailureInput(input *FailureInput, overrides FailureInput) {
	if overrides.QAMESHStatus != "" {
		input.QAMESHStatus = overrides.QAMESHStatus
	}
	if overrides.RedactionStatus != "" {
		input.RedactionStatus = overrides.RedactionStatus
	}
	if overrides.OwnedPaths != nil {
		input.OwnedPaths = overrides.OwnedPaths
	}
	if overrides.SourceHashes != nil {
		input.SourceHashes = overrides.SourceHashes
	}
	if overrides.EvidenceStrength == "non_deterministic" {
		input.DeterministicEvidence = false
	}
}
