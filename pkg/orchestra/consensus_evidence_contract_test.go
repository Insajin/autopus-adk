package orchestra

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsensusFindingCluster_PreservesProviderEvidenceAndCriticalOrderIndependently(t *testing.T) {
	responses := []ProviderResponse{
		{Provider: "claude", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "major", Category: "security", ScopeRef: "pkg/key.go",
			Description: "Rotate the exposed key", Suggestion: "schedule rotation",
		}, "open")},
		{Provider: "codex", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "critical", Category: "security", ScopeRef: "pkg/key.go:44",
			Description: "Rotate the exposed key", Suggestion: "rotate immediately",
		}, "open")},
		{Provider: "gemini", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "major", Category: "security", ScopeRef: "pkg/key.go#L91",
			Description: "Rotate the exposed key", Suggestion: "revoke old credentials",
		}, "open")},
	}

	for _, ordered := range [][]ProviderResponse{responses, {responses[2], responses[1], responses[0]}} {
		merged, summary := MergeConsensus(ordered, 0.67)
		result := FinalizeOrchestrationResult(&OrchestraResult{
			Strategy: StrategyConsensus, Responses: ordered, Merged: merged, Summary: summary,
		}, OrchestraConfig{
			Strategy:  StrategyConsensus,
			Providers: []ProviderConfig{{Name: "claude"}, {Name: "codex"}, {Name: "gemini"}},
		})

		assert.True(t, result.Veto)
		assert.Equal(t, TerminalBlocked, result.TerminalState)
		assert.Contains(t, merged, "critical")
		assert.Contains(t, merged, "schedule rotation")
		assert.Contains(t, merged, "rotate immediately")
		assert.Contains(t, merged, "revoke old credentials")
		require.NotNil(t, result.ConsensusMetrics)
		require.Len(t, result.ConsensusMetrics.FindingClaims, 1)
		assert.Len(t, result.ConsensusMetrics.FindingClaims[0].Evidence, 3)
	}
}

func TestConsensusFindingCluster_ResolvedCritical_DoesNotVeto(t *testing.T) {
	responses := []ProviderResponse{
		{Provider: "claude", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "critical", Category: "security", ScopeRef: "pkg/key.go",
			Description: "Rotate the exposed key", Suggestion: "done",
		}, "resolved")},
		{Provider: "codex", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "critical", Category: "security", ScopeRef: "pkg/key.go:40",
			Description: "Rotate the exposed key", Suggestion: "verified",
		}, "resolved")},
	}

	merged, summary := MergeConsensus(responses, 0.67)
	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy: StrategyConsensus, Responses: responses, Merged: merged, Summary: summary,
	}, OrchestraConfig{
		Strategy: StrategyConsensus, Providers: []ProviderConfig{{Name: "claude"}, {Name: "codex"}},
	})

	assert.False(t, result.Veto)
	assert.NotEqual(t, TerminalBlocked, result.TerminalState)
	assert.NotContains(t, merged, "Critical veto")
}

func TestConsensusFindingCluster_OpenEvidenceOutranksResolvedCritical(t *testing.T) {
	responses := []ProviderResponse{
		{Provider: "claude", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "critical", Category: "security", ScopeRef: "pkg/key.go",
			Description: "Rotate the exposed key", Suggestion: "verified fixed",
		}, "resolved")},
		{Provider: "codex", Output: reviewerEvidenceJSON(t, Finding{
			ID: "KEY-1", Severity: "major", Category: "security", ScopeRef: "pkg/key.go",
			Description: "Rotate the exposed key", Suggestion: "audit revocation",
		}, "open")},
	}

	merged, _ := MergeConsensus(responses, 0.5)

	assert.Contains(t, merged, "[major/security]")
	assert.NotContains(t, merged, "Critical veto")
}

func TestConsensusFindingClaims_AreSortedByStableIdentity(t *testing.T) {
	responses := []ProviderResponse{
		{Provider: "claude", Output: reviewerEvidenceJSON(t, Finding{
			ID: "ZETA", Severity: "minor", Category: "maintainability",
			ScopeRef: "pkg/z.go", Description: "Later identity",
		}, "open")},
		{Provider: "codex", Output: reviewerEvidenceJSON(t, Finding{
			ID: "ALPHA", Severity: "major", Category: "correctness",
			ScopeRef: "pkg/a.go", Description: "Earlier identity",
		}, "open")},
	}

	for _, ordered := range [][]ProviderResponse{responses, {responses[1], responses[0]}} {
		result := FinalizeOrchestrationResult(&OrchestraResult{
			Strategy: StrategyConsensus, Responses: ordered,
		}, OrchestraConfig{
			Strategy:  StrategyConsensus,
			Providers: []ProviderConfig{{Name: "claude"}, {Name: "codex"}},
		})

		require.NotNil(t, result.ConsensusMetrics)
		require.Len(t, result.ConsensusMetrics.FindingClaims, 2)
		assert.Equal(t, []string{"id|alpha", "id|zeta"}, []string{
			result.ConsensusMetrics.FindingClaims[0].Identity,
			result.ConsensusMetrics.FindingClaims[1].Identity,
		})
	}
}

func reviewerEvidenceJSON(t *testing.T, finding Finding, status string) string {
	t.Helper()
	body, err := json.Marshal(ReviewerOutput{
		Findings: []Finding{finding}, Verdict: "REVISE", Summary: "evidence",
		FindingStatus: []ReviewerFindingStatusOut{{ID: finding.ID, Status: status}},
	})
	require.NoError(t, err)
	return string(body)
}
