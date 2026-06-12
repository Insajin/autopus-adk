package domainreadiness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readyEvidenceInput returns a single passing scenario evidence input that yields
// a ready, denominator-included domain row.
func readyEvidenceInput(domain string) ScenarioEvidenceInput {
	return ScenarioEvidenceInput{
		Scenario:                sampleScenario("scn-"+domain, domain),
		Result:                  ScenarioResultPassed,
		SourceRefs:              []string{"source:" + domain},
		ActualEvidenceRefs:      []string{"evidence:" + domain},
		DeterministicOracleRefs: []string{"oracle:" + domain},
		Freshness:               EvidenceFreshnessCurrent,
		EvidenceCapturedAt:      time.Now(),
		RedactionStatus:         RedactionStatusPassed,
		RetentionClass:          RetentionClassMetadataOnly,
	}
}

func buildOne(t *testing.T, in ScenarioEvidenceInput) DomainReadinessEvidence {
	t.Helper()
	row, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "suite",
		RunID:       "run",
		WorkspaceID: "workspace",
		Scenarios:   []ScenarioEvidenceInput{in},
	})
	require.NoError(t, err)
	return row
}

// TestBuildEvidenceReadyState asserts a fully passing scenario stays ready and
// counts toward the denominator.
func TestBuildEvidenceReadyState(t *testing.T) {
	t.Parallel()

	row := buildOne(t, readyEvidenceInput("core"))

	assert.Equal(t, DomainReadinessStateReady, row.DomainReadinessState)
	assert.True(t, row.DenominatorIncluded)
	assert.Empty(t, row.ExclusionReason)
	require.Len(t, row.ScenarioResults, 1)
	assert.Equal(t, ScenarioResultPassed, row.ScenarioResults[0].ScenarioResult)
	assert.Equal(t, "oracle:core", row.ScenarioResults[0].DeterministicOracleRef)
}

// TestBuildEvidenceSetupGapWhenSourceRefsMissing asserts a missing source ref
// downgrades the row to setup_gap and excludes it.
func TestBuildEvidenceSetupGapWhenSourceRefsMissing(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.SourceRefs = nil

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateSetupGap, row.DomainReadinessState)
	assert.False(t, row.DenominatorIncluded)
	assert.Equal(t, string(UnsafeReasonSourceEvidenceMissing), row.ExclusionReason)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonSourceEvidenceMissing)
}

// TestBuildEvidenceStaleState asserts stale freshness produces a stale row.
func TestBuildEvidenceStaleState(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.Freshness = EvidenceFreshnessStale

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateStale, row.DomainReadinessState)
	assert.False(t, row.DenominatorIncluded)
	assert.Contains(t, row.Blockers, string(UnsafeReasonStaleEvidence))
	require.Len(t, row.ScenarioResults, 1)
	assert.Equal(t, ScenarioResultStale, row.ScenarioResults[0].ScenarioResult)
}

// TestBuildEvidenceUnsafeOnProviderWrite asserts provider writes reject the row.
func TestBuildEvidenceUnsafeOnProviderWrite(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.ProviderWriteCallCount = 2

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateUnsafe, row.DomainReadinessState)
	assert.Equal(t, 2, row.ProviderWriteCallCount)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonProviderWriteNotAllowed)
	assert.Equal(t, string(UnsafeReasonProviderWriteNotAllowed), row.ExclusionReason)
}

// TestBuildEvidenceUnsafeOnRedactionFailure asserts failed redaction rejects the
// row and forces redaction-failed freshness.
func TestBuildEvidenceUnsafeOnRedactionFailure(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.RedactionStatus = RedactionStatusFailed

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateUnsafe, row.DomainReadinessState)
	assert.Equal(t, EvidenceFreshnessRedactionFailed, row.Freshness)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonRedactionFailed)
}

// TestBuildEvidenceCrossWorkspaceBlocked asserts cross-workspace freshness rejects.
func TestBuildEvidenceCrossWorkspaceBlocked(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.Freshness = EvidenceFreshnessCrossWorkspaceBlocked

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateUnsafe, row.DomainReadinessState)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonCrossWorkspaceRef)
}

// TestBuildEvidenceBlockedOnAIOnly asserts an AI-only pass-fail support blocks the
// row when no deterministic evidence exists.
func TestBuildEvidenceBlockedOnAIOnly(t *testing.T) {
	t.Parallel()

	in := readyEvidenceInput("core")
	in.PassFailSupport = PassFailSupportAIOnly

	row := buildOne(t, in)

	assert.Equal(t, DomainReadinessStateBlocked, row.DomainReadinessState)
	assert.Contains(t, row.Blockers, "ai_only_pass_fail")
}

// TestBuildEvidencePartialWithMixedScenarios asserts ready plus a setup-gap scenario
// in the same domain yields partial.
func TestBuildEvidencePartialWithMixedScenarios(t *testing.T) {
	t.Parallel()

	good := readyEvidenceInput("core")
	gap := readyEvidenceInput("core")
	gap.Scenario.ScenarioID = "scn-core-gap"
	gap.Freshness = EvidenceFreshnessStale

	row, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "suite",
		RunID:       "run",
		WorkspaceID: "workspace",
		Scenarios:   []ScenarioEvidenceInput{good, gap},
	})
	require.NoError(t, err)

	assert.Equal(t, DomainReadinessStatePartial, row.DomainReadinessState)
	assert.False(t, row.DenominatorIncluded)
	assert.Len(t, row.ScenarioResults, 2)
}

// TestBuildEvidenceFailedAndPartial asserts a failed result without ready peers is
// blocked, and with a ready peer is partial.
func TestBuildEvidenceFailedState(t *testing.T) {
	t.Parallel()

	failed := readyEvidenceInput("core")
	failed.Result = ScenarioResultFailed

	row := buildOne(t, failed)
	assert.Equal(t, DomainReadinessStateBlocked, row.DomainReadinessState)

	good := readyEvidenceInput("core")
	failed.Scenario.ScenarioID = "scn-core-failed"
	mixed, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "suite",
		RunID:       "run",
		WorkspaceID: "workspace",
		Scenarios:   []ScenarioEvidenceInput{good, failed},
	})
	require.NoError(t, err)
	assert.Equal(t, DomainReadinessStatePartial, mixed.DomainReadinessState)
}

// TestBuildEvidenceRejectsMixedDomains asserts cross-domain inputs error out.
func TestBuildEvidenceRejectsMixedDomains(t *testing.T) {
	t.Parallel()

	_, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "suite",
		RunID:       "run",
		WorkspaceID: "workspace",
		Scenarios: []ScenarioEvidenceInput{
			readyEvidenceInput("core"),
			readyEvidenceInput("auth"),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixed domains")
}

// TestBuildEvidenceValidatesRequiredHeaders asserts missing identity fields error.
func TestBuildEvidenceValidatesRequiredHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input EvidenceBuildInput
		want  string
	}{
		{"missing suite", EvidenceBuildInput{RunID: "r", WorkspaceID: "w", Scenarios: []ScenarioEvidenceInput{readyEvidenceInput("core")}}, "missing suite id"},
		{"missing run", EvidenceBuildInput{SuiteID: "s", WorkspaceID: "w", Scenarios: []ScenarioEvidenceInput{readyEvidenceInput("core")}}, "missing run id"},
		{"missing workspace", EvidenceBuildInput{SuiteID: "s", RunID: "r", Scenarios: []ScenarioEvidenceInput{readyEvidenceInput("core")}}, "missing workspace id"},
		{"missing scenarios", EvidenceBuildInput{SuiteID: "s", RunID: "r", WorkspaceID: "w"}, "missing scenarios"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDomainReadinessEvidence(tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
