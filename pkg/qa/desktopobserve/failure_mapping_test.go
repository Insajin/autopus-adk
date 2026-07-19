package desktopobserve

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailureNormalizer_MapsTenSignalsToSafeNonPassOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		condition FailureCondition
		reason    ReasonCode
	}{
		{FailureProviderStart, ReasonProviderUnavailable},
		{FailureOperationMissing, ReasonCapabilityUnsupported},
		{FailureAccessibilityDenied, ReasonAccessibilityPermissionMissing},
		{FailureAppAliasUnmatched, ReasonTargetAppNotFound},
		{FailureWindowAliasUnmatched, ReasonTargetWindowNotFound},
		{FailureStateRefRejected, ReasonStaleState},
		{FailureLandmarksInsufficient, ReasonSemanticProjectionUnavailable},
		{FailureRedaction, ReasonRedactionFailed},
		{FailureRawOnlyQuarantine, ReasonEvidenceQuarantined},
		{FailureProtocolVersion, ReasonProviderProtocolMismatch},
	}
	emitted := make([]ReasonCode, 0, len(tests))
	for _, test := range tests {
		test := test
		t.Run(string(test.condition), func(t *testing.T) {
			capabilities := supportedCapabilities()
			requestedOperation := Operation("")
			if test.condition == FailureOperationMissing {
				capabilities[1].Status = CapabilityUnsupported
				requestedOperation = OperationGetState
			}
			outcome, err := NormalizeFailure(FailureSignal{
				Condition: test.condition,
				Provider: ProviderIdentity{
					Name: "autopus-desktop-local", Version: "1.0.0", ProtocolVersion: ProtocolVersion,
				},
				Scope:              ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
				CapabilitySummary:  capabilities,
				RequestedOperation: requestedOperation,
			})
			require.NoError(t, err)
			require.NotNil(t, outcome.ReasonCode)
			assert.Equal(t, test.reason, *outcome.ReasonCode)
			emitted = append(emitted, *outcome.ReasonCode)
			assert.NotEqual(t, VerdictPassed, outcome.Verdict)
			assert.Nil(t, outcome.SemanticProjection)
			assert.Zero(t, passedCheckCount(outcome.DeterministicChecks))
			require.NotNil(t, outcome.RuntimeReceipt.NextStep)
			assert.NotEmpty(t, *outcome.RuntimeReceipt.NextStep)
			require.NoError(t, outcome.RuntimeReceipt.Validate())
			body, err := json.Marshal(outcome)
			require.NoError(t, err)
			assert.False(t, unsafeFailureDetail.Match(body), string(body))
		})
	}
	assert.Equal(t, ReasonCodes(), emitted)
}

var unsafeFailureDetail = regexp.MustCompile(`(?i)(provider error|raw_tree|screenshot bytes|0x[0-9a-f]+|/Users/|/private/var/|pid[=:]|socket[=:]|secret[=:])`)
