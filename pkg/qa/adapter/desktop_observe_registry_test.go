package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopObserveRegistry_ExactReadOnlyMetadata(t *testing.T) {
	t.Parallel()

	metadata, ok := ByID("desktop-accessibility-observe")
	require.True(t, ok)
	assert.Equal(t, "desktop-accessibility-observe", metadata.ID)
	assert.Equal(t, []string{"desktop"}, metadata.Surfaces)
	assert.Equal(t, []string{"macos"}, metadata.SupportedPlatforms)
	assert.Equal(t, []string{"desktop-native"}, metadata.DefaultLanes)
	assert.Empty(t, metadata.RequiredBinaries)
	assert.Equal(t, []string{
		"capabilities",
		"get_state",
		"list_apps",
		"list_windows",
		"permissions",
	}, metadata.ReadOnlyOperations)
	assert.Equal(t, []string{
		"semantic_projection",
		"deterministic_checks",
		"runtime_receipt",
	}, metadata.ArtifactCapabilities)
	assert.NotContains(t, metadata.ArtifactCapabilities, "stdout")
	assert.NotContains(t, metadata.ArtifactCapabilities, "stderr")
}

func TestDesktopObserveRegistry_ExactFailureTaxonomyWithoutOrcaDependency(t *testing.T) {
	t.Parallel()

	metadata, ok := ByID("desktop-accessibility-observe")
	require.True(t, ok)
	assert.Equal(t, []string{
		"provider_unavailable",
		"capability_unsupported",
		"accessibility_permission_missing",
		"target_app_not_found",
		"target_window_not_found",
		"stale_state",
		"semantic_projection_unavailable",
		"redaction_failed",
		"evidence_quarantined",
		"provider_protocol_mismatch",
	}, metadata.SetupGapReasonCodes)
	assert.NotEmpty(t, metadata.SetupGapReason)
	for _, binary := range metadata.RequiredBinaries {
		assert.NotEqual(t, "orca", binary)
	}
}
