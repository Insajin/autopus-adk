package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func (m *yieldSaveFailureTerminal) SendLongText(_ context.Context, _ terminal.PaneID, _ string) error {
	return errors.New("force provider launch failure after pane provisioning")
}

func (m *yieldSaveFailureTerminal) FocusPane(_ context.Context, _ terminal.PaneID) error {
	m.setTempDir(m.invalidTempDir)
	return nil
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
	assert.Equal(t, []string{"pane-1"}, term.closeCalls,
		"the pane stays locally owned and must close when persistence fails")
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "pane-1",
		"a successfully closed pane must be untracked")
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
