package desktopobserve

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticProjection_RejectsRawStateStringsAndUnallowlistedNames(t *testing.T) {
	t.Parallel()

	t.Run("raw state value", func(t *testing.T) {
		t.Parallel()
		projection, err := NormalizeProjection(semanticFixture(), identityRedactor)
		require.NoError(t, err)
		body, err := json.Marshal(projection)
		require.NoError(t, err)
		body = replaceOnce(t, body, `"semantic_state":{"enabled":true}`, `"semantic_state":{"enabled":true,"value":"secret@example.com"}`)

		var decoded SemanticProjection
		err = decodeStrict(body, MaxEnvelopeBytes, &decoded)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUnknownField)
	})

	t.Run("unallowlisted accessible name", func(t *testing.T) {
		t.Parallel()
		projection := semanticFixture()
		projection.Root.Children = append(projection.Root.Children, SemanticNode{
			Role: RoleStaticText, Name: "secret@example.com",
		})
		normalized, err := NormalizeProjection(projection, identityRedactor)
		require.NoError(t, err)
		ledger := NewStateLedger()
		require.NoError(t, ledger.Register(bindingForProjection(normalized)))

		outcome, err := EvaluateOracle(OracleInput{
			Projection: normalized,
			Ledger:     ledger,
			Policy: OraclePolicy{
				MinimumLandmarks: signedAppOraclePolicy().MinimumLandmarks,
				AllowedNames:     []string{"Autopus"},
			},
			Receipt: successfulStateReceipt(RuntimeProviderLocal),
		})
		require.NoError(t, err)
		assert.Equal(t, VerdictBlocked, outcome.Verdict)
		require.NotNil(t, outcome.ReasonCode)
		assert.Equal(t, ReasonRedactionFailed, *outcome.ReasonCode)
		assert.Nil(t, outcome.SemanticProjection)
	})
}

func TestEvaluateOracle_PassedOutcomeDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	ledger := NewStateLedger()
	require.NoError(t, ledger.Register(bindingForProjection(normalized)))
	input := OracleInput{
		Projection: normalized,
		Ledger:     ledger,
		Policy:     signedAppOraclePolicy(),
		Receipt:    successfulStateReceipt(RuntimeProviderLocal),
	}
	outcome, err := EvaluateOracle(input)
	require.NoError(t, err)
	require.Equal(t, VerdictPassed, outcome.Verdict)
	before, err := json.Marshal(outcome)
	require.NoError(t, err)
	beforeCanonical := append([]byte(nil), outcome.SemanticProjection.CanonicalJSON...)

	input.Projection.Root.Children[0].Name = "mutated"
	*input.Projection.Root.SemanticState.Enabled = false
	input.Projection.Root.AdvertisedActions[0] = Action("AXUnsafe")
	input.Projection.Root.Frame.X = 999
	input.Projection.CanonicalJSON[0] = 'x'
	input.Receipt.CapabilitySummary[0].Name = Operation("screenshot")

	after, err := json.Marshal(outcome)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, beforeCanonical, outcome.SemanticProjection.CanonicalJSON)
	assert.Equal(t, "Autopus", outcome.SemanticProjection.Root.Children[0].Name)
	assert.True(t, *outcome.SemanticProjection.Root.SemanticState.Enabled)
	assert.Equal(t, OperationCapabilities, outcome.RuntimeReceipt.CapabilitySummary[0].Name)
}

func TestFailureNormalizer_InvalidSignalsFailClosedWithoutReceipt(t *testing.T) {
	t.Parallel()

	valid := FailureSignal{
		Condition:         FailureProviderStart,
		Provider:          providerIdentity(RuntimeProviderLocal),
		Scope:             ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
		CapabilitySummary: unsupportedCapabilities(),
	}
	tests := []struct {
		name   string
		mutate func(*FailureSignal)
	}{
		{name: "unknown condition", mutate: func(signal *FailureSignal) { signal.Condition = "unknown" }},
		{name: "invalid provider", mutate: func(signal *FailureSignal) { signal.Provider.Name = "orca-secret-path" }},
		{name: "invalid scope", mutate: func(signal *FailureSignal) { signal.Scope.PublicRef = "other-window" }},
		{name: "invalid capabilities", mutate: func(signal *FailureSignal) { signal.CapabilitySummary = nil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			signal := valid
			signal.CapabilitySummary = append([]CapabilityStatus(nil), valid.CapabilitySummary...)
			test.mutate(&signal)
			outcome, err := NormalizeFailure(signal)
			require.Error(t, err)
			assert.Empty(t, outcome.RuntimeReceipt.SchemaVersion)
			assert.Nil(t, outcome.SemanticProjection)
		})
	}
}

func TestFailureNormalizer_CapabilityUnsupportedReceiptIsTruthful(t *testing.T) {
	t.Parallel()

	capabilities := supportedCapabilities()
	capabilities[1].Status = CapabilityUnsupported
	outcome, err := NormalizeFailure(FailureSignal{
		Condition:          FailureOperationMissing,
		Provider:           providerIdentity(RuntimeProviderLocal),
		Scope:              ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
		CapabilitySummary:  capabilities,
		RequestedOperation: OperationGetState,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, ReasonCapabilityUnsupported, *outcome.ReasonCode)
	assert.Equal(t, CapabilityUnsupported, outcome.RuntimeReceipt.CapabilitySummary[1].Status)

	capabilities[1].Status = CapabilitySupported
	invalid, err := NormalizeFailure(FailureSignal{
		Condition:          FailureOperationMissing,
		Provider:           providerIdentity(RuntimeProviderLocal),
		Scope:              ReceiptScope{Kind: ScopeWindow, PublicRef: "main-window"},
		CapabilitySummary:  capabilities,
		RequestedOperation: OperationGetState,
	})
	require.Error(t, err)
	assert.Empty(t, invalid.RuntimeReceipt.SchemaVersion)
}

func TestEvaluateOracle_InvalidReceiptReturnsTypedErrorWithoutConsumingState(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	ledger := NewStateLedger()
	binding := bindingForProjection(normalized)
	require.NoError(t, ledger.Register(binding))
	receipt := successfulStateReceipt(RuntimeProviderLocal)
	receipt.Provider.Name = "invalid-provider"

	outcome, err := EvaluateOracle(OracleInput{
		Projection: normalized,
		Ledger:     ledger,
		Policy:     signedAppOraclePolicy(),
		Receipt:    receipt,
	})
	require.Error(t, err)
	assert.Empty(t, outcome.RuntimeReceipt.SchemaVersion)
	assert.Nil(t, outcome.SemanticProjection)
	require.NoError(t, ledger.Consume(binding), "invalid receipt must fail before oracle consumption")
}

func replaceOnce(t *testing.T, body []byte, old, replacement string) []byte {
	t.Helper()
	bodyString := string(body)
	require.Contains(t, bodyString, old)
	return []byte(strings.Replace(bodyString, old, replacement, 1))
}
