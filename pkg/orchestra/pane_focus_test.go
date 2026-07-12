package orchestra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paneFocusMock struct {
	*mockTerminal
	focusErr   error
	focusCalls []terminal.PaneID
	events     []string
}

func (m *paneFocusMock) SplitPane(ctx context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	paneID, err := m.mockTerminal.SplitPane(ctx, dir)
	if err == nil {
		m.events = append(m.events, fmt.Sprintf("split:%s", paneID))
	}
	return paneID, err
}

func (m *paneFocusMock) FocusPane(_ context.Context, paneID terminal.PaneID) error {
	m.focusCalls = append(m.focusCalls, paneID)
	m.events = append(m.events, fmt.Sprintf("focus:%s", paneID))
	return m.focusErr
}

func removePaneFocusOutputs(panes []paneInfo) {
	for _, pane := range panes {
		_ = os.Remove(pane.outputFile)
	}
}

func TestSplitProviderPanes_WithPaneFocuser_FocusesFirstProviderAfterFanOut(t *testing.T) {
	t.Parallel()

	// Given
	mock := &paneFocusMock{mockTerminal: newCmuxMock()}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("codex"),
			echoProvider("gemini"),
		},
		Terminal: mock,
	}

	// When
	panes, failed, err := splitProviderPanes(context.Background(), cfg)
	defer removePaneFocusOutputs(panes)

	// Then
	require.NoError(t, err)
	assert.Empty(t, failed)
	require.Len(t, panes, 3)
	assert.Equal(t, []terminal.PaneID{"pane-1"}, mock.focusCalls)
	assert.Equal(t, []string{
		"split:pane-1",
		"split:pane-2",
		"split:pane-3",
		"focus:pane-1",
	}, mock.events, "focus must happen once, after fan-out, on the first provider pane")
}

func TestSplitProviderPanes_WithoutPaneFocuser_SucceedsWithoutFocus(t *testing.T) {
	t.Parallel()

	// Given
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("codex"),
		},
		Terminal: mock,
	}

	// When
	panes, failed, err := splitProviderPanes(context.Background(), cfg)
	defer removePaneFocusOutputs(panes)

	// Then
	require.NoError(t, err)
	assert.Empty(t, failed)
	assert.Len(t, panes, 2)
}

func TestSplitProviderPanes_WhenFocusFails_ReturnsCreatedPanes(t *testing.T) {
	t.Parallel()

	// Given
	mock := &paneFocusMock{
		mockTerminal: newCmuxMock(),
		focusErr:     errors.New("focus unavailable"),
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("codex"),
		},
		Terminal: mock,
	}

	// When
	panes, failed, err := splitProviderPanes(context.Background(), cfg)
	defer removePaneFocusOutputs(panes)

	// Then
	require.NoError(t, err, "focus is a best-effort visibility aid")
	assert.Empty(t, failed)
	assert.Len(t, panes, 2)
	assert.Equal(t, []terminal.PaneID{"pane-1"}, mock.focusCalls)
}
