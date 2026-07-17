package config

// SPEC-ADK-REVIEW-INTEGRITY-001 config-side defaults tests: the derive-by-default
// quorum and the per-document floor input to spec.ResolveAuxTotalBudget. The total
// aux-doc budget itself is owned by pkg/spec (DefaultAuxTotalBudgetLines).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultFullConfig_DocContextMaxLinesFloorInput verifies the per-document cap
// default stays 200. It is the floor input to spec.ResolveAuxTotalBudget (which
// raises it up to spec.DefaultAuxTotalBudgetLines), not the total budget itself.
func TestDefaultFullConfig_DocContextMaxLinesFloorInput(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	assert.Equal(t, 200, cfg.Spec.ReviewGate.DocContextMaxLines,
		"per-document cap stays 200 as the authoring warning / compression fallback threshold and the ResolveAuxTotalBudget floor input")
}

// TestDefaultFullConfig_MinProvidersUnsetDerivesMajority verifies the review gate
// ships with min_providers unset (0) so the quorum derives from the live provider
// count (3 providers -> majority 2) rather than a frozen literal (REQ-RINT-QUORUM-05).
func TestDefaultFullConfig_MinProvidersUnsetDerivesMajority(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	assert.Equal(t, 0, cfg.Spec.ReviewGate.MinProviders,
		"min_providers must default to 0 so the majority quorum derives from the configured provider count")

	// Document the derived expectation for the default 3-provider set without
	// importing pkg/spec (which would risk a test-time import cycle): 3/2+1 = 2.
	configured := len(cfg.Spec.ReviewGate.Providers)
	require.Equal(t, 3, configured, "default review gate ships three providers")
	assert.Equal(t, 2, configured/2+1, "derived majority quorum for the default provider set is 2")
}
