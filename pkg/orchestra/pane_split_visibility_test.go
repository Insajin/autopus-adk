package orchestra

import (
	"context"
	"os"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type surfaceCapableMock struct {
	*mockTerminal
	createSurfaceCalls int
}

func (m *surfaceCapableMock) CreateSurface(context.Context) (terminal.PaneID, error) {
	m.createSurfaceCalls++
	return terminal.PaneID("surface-hidden"), nil
}

func TestSplitProviderPanes_SurfaceCapableTerminalStillUsesVisibleSplits(t *testing.T) {
	t.Parallel()

	mock := &surfaceCapableMock{mockTerminal: newCmuxMock()}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("codex"),
			echoProvider("gemini"),
		},
		Terminal: mock,
	}

	panes, _, err := splitProviderPanes(context.Background(), cfg)
	require.NoError(t, err)
	for _, pane := range panes {
		_ = os.Remove(pane.outputFile)
	}

	assert.Zero(t, mock.createSurfaceCalls, "provider panes must be visible splits, not hidden tab surfaces")
	require.Len(t, mock.splitPaneCalls, 3)
	for _, dir := range mock.splitPaneCalls {
		assert.Equal(t, terminal.Horizontal, dir)
	}
}
