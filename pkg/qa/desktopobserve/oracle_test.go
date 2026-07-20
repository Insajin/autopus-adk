package desktopobserve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanvasMinimumLandmarks_MissingSemanticCoverageCannotPass(t *testing.T) {
	t.Parallel()

	projection := semanticFixture()
	projection.Root = SemanticNode{
		NodeRef: "canvas",
		Role:    RoleCanvas,
		Frame:   &Frame{X: 0, Y: 0, Width: 900, Height: 700},
	}
	normalized, err := NormalizeProjection(projection, identityRedactor)
	require.NoError(t, err)
	ledger := NewStateLedger()
	require.NoError(t, ledger.Register(bindingForProjection(normalized)))

	outcome, err := EvaluateOracle(OracleInput{
		Projection: normalized,
		Ledger:     ledger,
		Policy:     signedAppOraclePolicy(),
		Receipt:    successfulStateReceipt(RuntimeProviderLocal),
	})
	require.NoError(t, err)

	assert.NotEqual(t, VerdictPassed, outcome.Verdict)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, ReasonSemanticProjectionUnavailable, *outcome.ReasonCode)
	assert.Nil(t, outcome.SemanticProjection)
	assert.Zero(t, passedCheckCount(outcome.DeterministicChecks))
}

func TestDeterministicOracle_FreshMinimumLandmarksPassOnce(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeProjection(semanticFixture(), identityRedactor)
	require.NoError(t, err)
	ledger := NewStateLedger()
	binding := bindingForProjection(normalized)
	require.NoError(t, ledger.Register(binding))
	input := OracleInput{
		Projection: normalized,
		Ledger:     ledger,
		Policy:     signedAppOraclePolicy(),
		Receipt:    successfulStateReceipt(RuntimeProviderLocal),
	}

	first, err := EvaluateOracle(input)
	require.NoError(t, err)
	assert.Equal(t, VerdictPassed, first.Verdict)
	require.NotNil(t, first.SemanticProjection)
	assert.NotZero(t, passedCheckCount(first.DeterministicChecks))

	second, err := EvaluateOracle(input)
	require.NoError(t, err)
	assert.NotEqual(t, VerdictPassed, second.Verdict)
	require.NotNil(t, second.ReasonCode)
	assert.Equal(t, ReasonStaleState, *second.ReasonCode)
	assert.Nil(t, second.SemanticProjection)
}

func signedAppOraclePolicy() OraclePolicy {
	return OraclePolicy{
		MinimumLandmarks: []LandmarkRequirement{
			{Role: RoleApplication, Name: "Autopus", RequiredState: StateEnabled},
			{Role: RoleWindow, Name: "Autopus", RequiredState: StateFocused},
		},
		AllowedNames: []string{"", "Autopus"},
	}
}

func bindingForProjection(projection SemanticProjection) StateBinding {
	return StateBinding{
		StateRef:    projection.StateRef,
		ProviderRef: projection.ProviderRef,
		AppRef:      projection.AppRef,
		WindowRef:   projection.WindowRef,
		Digest:      projection.Digest,
	}
}

func passedCheckCount(checks []DeterministicCheck) int {
	count := 0
	for _, check := range checks {
		if check.Status == CheckPassed {
			count++
		}
	}
	return count
}
