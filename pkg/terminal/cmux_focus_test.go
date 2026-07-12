package terminal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ PaneFocuser = (*CmuxAdapter)(nil)

func TestCmuxAdapter_FocusPane_WithSurfaceRef_MovesSurfaceWithFocus(t *testing.T) {
	// Given
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	adapter := &CmuxAdapter{}

	// When
	err := adapter.FocusPane(context.Background(), PaneID("surface:7"))

	// Then
	require.NoError(t, err)
	assert.Equal(t, "cmux", captured.lastName())
	assert.Equal(t, []string{"move-surface", "--surface", "surface:7", "--focus", "true"}, captured.lastArgs())
}
