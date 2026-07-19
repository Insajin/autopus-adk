package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestRuntimeReceipt_DesktopEvidenceRoundTripPreservesStrictSafeValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	observation := successfulObservationEvidence(t)
	path, err := WriteFinalManifest(
		desktopObservationManifest(t, dir, observation, "passed"),
		filepath.Join(dir, "published"),
	)
	require.NoError(t, err)
	manifest, err := LoadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, manifest.OracleResults.DesktopObservation)
	receipt := manifest.OracleResults.DesktopObservation.RuntimeReceipt
	require.NoError(t, receipt.Validate())
	assert.Equal(t, "autopus-desktop-local", receipt.Provider.Name)
	assert.Equal(t, desktopobserve.ProtocolVersion, receipt.Provider.ProtocolVersion)
	assert.Equal(t, desktopobserve.ScopeWindow, receipt.Scope.Kind)
	assert.Equal(t, "main-window", receipt.Scope.PublicRef)
	assert.Equal(t, desktopobserve.ReadOnlyOperations(), receiptCapabilityNames(receipt))
	assert.Nil(t, receipt.ReasonCode)
	assert.Nil(t, receipt.NextStep)

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assertForbiddenDesktopInventoryZero(t, body)
}

func receiptCapabilityNames(receipt desktopobserve.RuntimeReceipt) []desktopobserve.Operation {
	values := make([]desktopobserve.Operation, 0, len(receipt.CapabilitySummary))
	for _, capability := range receipt.CapabilitySummary {
		values = append(values, capability.Name)
	}
	return values
}
