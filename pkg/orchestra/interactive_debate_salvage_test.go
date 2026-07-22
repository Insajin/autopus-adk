package orchestra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type yieldSaveFailureTerminal struct {
	mockTerminal
	invalidTempDir string
	setTempDir     func(string)
}

func (m *yieldSaveFailureTerminal) SplitPane(_ context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splitPaneCalls = append(m.splitPaneCalls, dir)
	if m.splitPaneErr != nil {
		return "", m.splitPaneErr
	}
	m.nextPaneID++
	id := terminal.PaneID(fmt.Sprintf("surface:%d", m.nextPaneID))
	m.createdPanes = append(m.createdPanes, id)
	return id, nil
}

func (m *yieldSaveFailureTerminal) SendLongText(_ context.Context, _ terminal.PaneID, _ string) error {
	return errors.New("force provider launch failure after pane provisioning")
}

func (m *yieldSaveFailureTerminal) FocusPane(_ context.Context, _ terminal.PaneID) error {
	m.setTempDir(m.invalidTempDir)
	return nil
}

func (m *yieldSaveFailureTerminal) WorkspaceRef() (string, error) {
	return "workspace:1", nil
}

func (m *yieldSaveFailureTerminal) WithWorkspaceRef(string) (terminal.Terminal, error) {
	return m, nil
}

func TestRunPaneDebate_YieldSaveFailureCleansOwnedPanes(t *testing.T) {
	isolateSurfaceTracker(t)

	workingDir := t.TempDir()
	invalidTempDir := filepath.Join(workingDir, "not-a-directory")
	require.NoError(t, os.WriteFile(invalidTempDir, []byte("block session persistence"), 0o600))
	term := &yieldSaveFailureTerminal{
		mockTerminal:   mockTerminal{name: "cmux"},
		invalidTempDir: invalidTempDir,
		setTempDir:     func(path string) { t.Setenv("TMPDIR", path) },
	}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "yield only after durable session persistence",
		TimeoutSeconds: 1,
		Terminal:       term,
		Interactive:    true,
		YieldRounds:    true,
		NoJudge:        true,
		InitialDelay:   time.Millisecond,
		WorkingDir:     workingDir,
	}

	result, err := runPaneDebate(context.Background(), cfg, 1, time.Second, time.Now())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "persist yield session")
	assert.Equal(t, []string{"surface:1"}, term.closeCalls,
		"the pane stays locally owned and must close when persistence fails")
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:1",
		"a successfully closed pane must be untracked")
}

func TestRunPaneDebate_YieldHandoffFailureKeepsDurableSessionAndPanes(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := &yieldSaveFailureTerminal{
		mockTerminal: mockTerminal{name: "cmux"},
		setTempDir:   func(string) {},
	}
	originalUntracker := yieldSurfaceUntracker
	yieldSurfaceUntracker = func(terminal.Terminal, string) error {
		return errors.New("injected tracker handoff failure")
	}
	t.Cleanup(func() { yieldSurfaceUntracker = originalUntracker })
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "keep the durable recovery handle",
		TimeoutSeconds: 1,
		Terminal:       term,
		Interactive:    true,
		YieldRounds:    true,
		NoJudge:        true,
		InitialDelay:   time.Millisecond,
		WorkingDir:     t.TempDir(),
	}

	result, err := runPaneDebate(context.Background(), cfg, 1, time.Second, time.Now())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Empty(t, term.closeCalls,
		"a durable yield session owns the panes even when tracker handoff reports an error")
	assert.Contains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:1",
		"the tracker recovery handle must remain when handoff persistence fails")
	match := regexp.MustCompile(`yield session (orch-[0-9a-f]+)`).FindStringSubmatch(err.Error())
	require.Len(t, match, 2)
	sessionID := match[1]
	t.Cleanup(func() { _ = RemoveSession(sessionID) })
	assert.ErrorContains(t, err, "auto orchestra cleanup --session-id "+sessionID)
	loaded, loadErr := LoadSession(sessionID)
	require.NoError(t, loadErr)
	assert.Equal(t, map[string]string{"claude": "surface:1"}, loaded.Panes)
}

func TestExecuteRound_OrdersResponsesAtDebateBoundary(t *testing.T) {
	term := &mockTerminal{name: "cmux"}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "delta"},
			{Name: "alpha"},
		},
		Strategy:       StrategyDebate,
		Terminal:       term,
		TimeoutSeconds: 1,
		InitialDelay:   time.Nanosecond,
	}
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "unknown-z"}, paneID: "pane-z", skipWait: true},
		{provider: ProviderConfig{Name: "alpha"}, paneID: "pane-a", skipWait: true},
		{provider: ProviderConfig{Name: "unknown-a"}, paneID: "pane-u", skipWait: true},
		{provider: ProviderConfig{Name: "delta"}, paneID: "pane-d", skipWait: true},
	}

	responses := executeRound(context.Background(), cfg, panes, nil, 1, nil)

	assert.Equal(t, []string{"delta", "alpha", "unknown-a", "unknown-z"}, responseProviderNames(responses))
}

func TestWaitAndCollectResults_PreservesCollectionOrder(t *testing.T) {
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "second"}, skipWait: true},
		{provider: ProviderConfig{Name: "first"}, skipWait: true},
	}

	responses := waitAndCollectResults(
		context.Background(), OrchestraConfig{}, panes, nil, time.Now(), nil, nil, 0,
	)

	assert.Equal(t, []string{"second", "first"}, responseProviderNames(responses),
		"shared collection order must remain untouched for non-debate strategies")
}

func responseProviderNames(responses []ProviderResponse) []string {
	names := make([]string, 0, len(responses))
	for _, response := range responses {
		names = append(names, response.Provider)
	}
	return names
}
