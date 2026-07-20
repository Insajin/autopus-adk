package run

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestReadOnlyOperationAllowlist_DispatchRejectsForbiddenOperationsBeforeProviderCalls(t *testing.T) {
	t.Parallel()

	providers := []desktopobserve.RuntimeProvider{
		desktopobserve.RuntimeProviderLocal,
		desktopobserve.RuntimeProviderOrca,
	}
	forbidden := []desktopobserve.Operation{
		"click",
		"type",
		"paste",
		"drag",
		"scroll",
		"screenshot",
		"raw_tree",
		"shell",
		"file",
		"lifecycle_mutation",
	}

	for _, provider := range providers {
		provider := provider
		for _, operation := range forbidden {
			operation := operation
			t.Run(string(provider)+"/"+string(operation), func(t *testing.T) {
				t.Parallel()
				local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
				orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
				request := desktopRunRequest(provider)
				request.Operations = []desktopobserve.Operation{operation}

				outcome, err := newDesktopObservationRunner(local, orca).Run(context.Background(), request)
				require.NoError(t, err)
				require.NotNil(t, outcome.ReasonCode)
				assert.Equal(t, desktopobserve.ReasonCapabilityUnsupported, *outcome.ReasonCode)
				assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
				assert.Nil(t, outcome.SemanticProjection)
				assert.Zero(t, passedDesktopCheckCount(outcome.DeterministicChecks))
				assert.Equal(t, desktopobserve.ReadOnlyOperations(), capabilityNames(outcome.RuntimeReceipt))
				assertDesktopClientCallCountsZero(t, local)
				assertDesktopClientCallCountsZero(t, orca)
			})
		}
	}
}

func assertDesktopClientCallCountsZero(t *testing.T, client *fakeDesktopClient) {
	t.Helper()
	assert.Empty(t, client.calls)
	assert.Zero(t, client.rawIndexCalls)
	assert.Zero(t, client.actionCalls)
	assert.Zero(t, client.screenshotCalls)
}

func passedDesktopCheckCount(checks []desktopobserve.DeterministicCheck) int {
	count := 0
	for _, check := range checks {
		if check.Status == desktopobserve.CheckPassed {
			count++
		}
	}
	return count
}
