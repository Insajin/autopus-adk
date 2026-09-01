package desktopobserve

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExactReasonTaxonomy_HasTwelveSafeNonEmptyNextSteps(t *testing.T) {
	t.Parallel()

	want := []ReasonCode{
		ReasonProviderUnavailable,
		ReasonCapabilityUnsupported,
		ReasonAccessibilityPermissionMissing,
		ReasonTargetAppNotFound,
		ReasonTargetWindowNotFound,
		ReasonStaleState,
		ReasonSemanticProjectionUnavailable,
		ReasonRedactionFailed,
		ReasonEvidenceQuarantined,
		ReasonProviderProtocolMismatch,
		ReasonDeclaredLandmarkNotFound,
		ReasonObservedTreeBoundExceeded,
	}
	assert.Equal(t, want, ReasonCodes())
	assert.Len(t, ReasonCodes(), 12)

	unsafe := regexp.MustCompile(`(?i)(/users/|/tmp/|pid\s*[=:]|socket\s*[=:]|handle\s*[=:]|raw[_ -]?title|secret\s*[=:]|\b(?:sh|bash|zsh)\s+-c\b|\brm\s+-rf\b)`)
	for _, reason := range ReasonCodes() {
		next := NextStep(reason)
		assert.NotEmpty(t, next, reason)
		assert.False(t, unsafe.MatchString(next), "%s next_step is unsafe: %s", reason, next)
	}
}

// SPEC-QAMESH-013 REQ-5: a missing landmark has to report itself. The two codes
// it must not collapse into are named here so a future merge of the taxonomy is
// a test failure rather than a silent misreport.
func TestDeclaredLandmarkNotFound_IsItsOwnCodeAndRoundTripsThroughReceiptValidate(t *testing.T) {
	t.Parallel()

	reason := ReasonDeclaredLandmarkNotFound
	assert.NotEqual(t, ReasonProviderUnavailable, reason)
	assert.NotEqual(t, ReasonSemanticProjectionUnavailable, reason)
	assert.Contains(t, ReasonCodes(), reason)
	assert.Equal(t,
		"Align the declared landmark role and name with the observed surface, then rerun.",
		NextStep(reason))

	receipt := successfulReceipt(RuntimeProviderOrca)
	receipt.ReasonCode = &reason
	next := NextStep(reason)
	receipt.NextStep = &next
	require.NoError(t, receipt.Validate())

	drifted := receipt
	other := NextStep(ReasonSemanticProjectionUnavailable)
	drifted.NextStep = &other
	require.Error(t, drifted.Validate(), "a receipt must carry this code's own next step")

	err := DeclaredLandmarkNotFound(RoleWindow, "Autopus Desktop")
	assert.Equal(t, reason, ReasonCodeOf(err))
	assert.Contains(t, err.Error(), "AXWindow")
	assert.Contains(t, err.Error(), "Autopus Desktop")
}

// SPEC-QAMESH-013 REQ-6: an oversized tree is refused by the name of the bound
// it crossed, and never as a protocol mismatch or a pass.
func TestObservedTreeBoundExceeded_NamesTheBoundAndRoundTripsThroughReceiptValidate(t *testing.T) {
	t.Parallel()

	reason := ReasonObservedTreeBoundExceeded
	assert.NotEqual(t, ReasonProviderProtocolMismatch, reason)
	assert.NotEqual(t, ReasonSemanticProjectionUnavailable, reason)
	assert.Contains(t, ReasonCodes(), reason)
	assert.Equal(t,
		"Observe a surface within the declared node, depth, and byte bounds instead of truncating the tree.",
		NextStep(reason))

	receipt := successfulReceipt(RuntimeProviderOrca)
	receipt.ReasonCode = &reason
	next := NextStep(reason)
	receipt.NextStep = &next
	require.NoError(t, receipt.Validate())

	for _, bound := range []ObservedTreeBound{
		ObservedTreeBoundNodes, ObservedTreeBoundDepth, ObservedTreeBoundBytes,
	} {
		err := ObservedTreeBoundExceeded(bound, 256, 4096)
		assert.Equal(t, reason, ReasonCodeOf(err))
		assert.Contains(t, err.Error(), string(bound))
		assert.Contains(t, err.Error(), "4096")
		assert.NotErrorIs(t, err, ErrMalformedEnvelope)
	}
}

// A blocked outcome carries a verdict that is never a pass, for both new codes.
func TestNewReasonCodes_NormalizeToBlockedOutcomes(t *testing.T) {
	t.Parallel()

	for condition, reason := range map[FailureCondition]ReasonCode{
		FailureDeclaredLandmarkAbsent: ReasonDeclaredLandmarkNotFound,
		FailureObservedTreeBound:      ReasonObservedTreeBoundExceeded,
	} {
		outcome, err := NormalizeFailure(FailureSignal{
			Condition:         condition,
			Provider:          providerIdentity(RuntimeProviderOrca),
			Scope:             ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
			CapabilitySummary: supportedCapabilities(),
		})
		require.NoError(t, err)
		require.NotNil(t, outcome.ReasonCode)
		assert.Equal(t, reason, *outcome.ReasonCode)
		assert.Equal(t, VerdictBlocked, outcome.Verdict)
		assert.Nil(t, outcome.SemanticProjection)
		require.NoError(t, outcome.RuntimeReceipt.Validate())
	}
}
