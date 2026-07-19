package run

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestLocalOrcaOracleParity_FullTypedEvidenceDiffIsExplicitlyBounded(t *testing.T) {
	t.Parallel()

	local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
	orca := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
	localProjection := runnerSemanticFixture("provider-local", "state-shared")
	localProjection.Root.Frame = &desktopobserve.Frame{X: 1, Y: 2, Width: 1200, Height: 800}
	localProjection.Root.Children[0].Frame = &desktopobserve.Frame{X: 10, Y: 20, Width: 1180, Height: 760}
	local.projection = &localProjection
	runner := newDesktopObservationRunner(local, orca)
	localOutcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	orcaOutcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderOrca))
	require.NoError(t, err)

	assert.Equal(t, localOutcome.RuntimeReceipt.Provider.Version, orcaOutcome.RuntimeReceipt.Provider.Version)
	assert.Equal(t, localOutcome.RuntimeReceipt.Provider.ProtocolVersion, orcaOutcome.RuntimeReceipt.Provider.ProtocolVersion)
	require.NotNil(t, localOutcome.SemanticProjection)
	require.NotNil(t, orcaOutcome.SemanticProjection)
	assert.NotEqual(t, localOutcome.SemanticProjection.ProviderRef, orcaOutcome.SemanticProjection.ProviderRef)
	assert.Equal(t, localOutcome.SemanticProjection.AppRef, orcaOutcome.SemanticProjection.AppRef)
	assert.Equal(t, localOutcome.SemanticProjection.WindowRef, orcaOutcome.SemanticProjection.WindowRef)
	assert.Equal(t, localOutcome.RuntimeReceipt.Scope, orcaOutcome.RuntimeReceipt.Scope)
	assert.Equal(t, localOutcome.Verdict, orcaOutcome.Verdict)
	assert.Equal(t, localOutcome.SemanticProjection.Digest, orcaOutcome.SemanticProjection.Digest)
	assert.Equal(t, localOutcome.DeterministicChecks, orcaOutcome.DeterministicChecks)

	localEvidence := desktopObservationEvidence(localOutcome)
	require.NotNil(t, localEvidence.SemanticProjection)
	assert.Nil(t, localEvidence.SemanticProjection.Root.Frame)
	assert.Nil(t, localEvidence.SemanticProjection.Root.Children[0].Frame)
	assert.NotNil(t, localOutcome.SemanticProjection.Root.Frame)
	assert.NotNil(t, localOutcome.SemanticProjection.Root.Children[0].Frame)
	assert.Equal(t, normalizedObservationBytes(t, localOutcome), normalizedObservationBytes(t, orcaOutcome))
	assertDesktopClientUnsafeCallsZero(t, local)
	assertDesktopClientUnsafeCallsZero(t, orca)
}

func normalizedObservationBytes(t *testing.T, outcome desktopobserve.OracleOutcome) []byte {
	t.Helper()
	value := desktopObservationEvidence(outcome)
	body, err := json.Marshal(value)
	require.NoError(t, err)
	var clone desktopobserve.ObservationEvidence
	require.NoError(t, json.Unmarshal(body, &clone))
	require.NotNil(t, clone.SemanticProjection)
	clone.SemanticProjection.ProviderRef = "<provider-public-ref>"
	clone.SemanticProjection.StateRef = "<state-public-ref>"
	placeholderNodeRefs(&clone.SemanticProjection.Root, 0)
	clone.RuntimeReceipt.Provider.Name = "<provider-public-name>"
	body, err = json.Marshal(clone)
	require.NoError(t, err)
	return body
}

func placeholderNodeRefs(node *desktopobserve.SemanticNode, index int) int {
	node.NodeRef = fmt.Sprintf("<node-public-ref-%d>", index)
	index++
	for child := range node.Children {
		index = placeholderNodeRefs(&node.Children[child], index)
	}
	return index
}

func assertDesktopClientUnsafeCallsZero(t *testing.T, client *fakeDesktopClient) {
	t.Helper()
	assert.Zero(t, client.rawIndexCalls)
	assert.Zero(t, client.actionCalls)
	assert.Zero(t, client.screenshotCalls)
}
