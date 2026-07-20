package orchestra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackedFlakyCleanupTerminal struct {
	flakyCloseTerminal
}

func (m *trackedFlakyCleanupTerminal) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	return terminal.PaneID("surface:42"), nil
}

func TestCleanupPanes_RetriesCloseAndUntracksSplitSurface(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &trackedFlakyCleanupTerminal{flakyCloseTerminal: flakyCloseTerminal{
		mockTerminal: mockTerminal{name: "cmux"},
		failUntil:    1,
	}}

	paneID, err := splitTrackedPane(context.Background(), term, terminal.Horizontal)
	require.NoError(t, err)
	assert.Contains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), string(paneID))

	cleanupPanes(term, []paneInfo{{paneID: paneID}})

	assert.Equal(t, 2, term.closeAttempt, "cleanup must retry a transient Close failure")
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), string(paneID),
		"a successfully closed pane must be removed from the tracker")
}

type orderedSplitTerminal struct {
	eventsPath        string
	cleanupGate       string
	splitCalls        int
	sendCommandCalls  int
	sendLongTextCalls int
}

func (m *orderedSplitTerminal) Name() string                                  { return "cmux" }
func (m *orderedSplitTerminal) CreateWorkspace(context.Context, string) error { return nil }

func (m *orderedSplitTerminal) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	m.splitCalls++
	if m.splitCalls == 1 {
		_ = appendTransportEvent(m.eventsPath, "split:first")
		return terminal.PaneID("surface:101"), nil
	}
	_ = appendTransportEvent(m.eventsPath, "split:second-error")
	return "", errors.New("second split failed")
}

func (m *orderedSplitTerminal) SendCommand(context.Context, terminal.PaneID, string) error {
	m.sendCommandCalls++
	return nil
}

func (m *orderedSplitTerminal) SendLongText(context.Context, terminal.PaneID, string) error {
	m.sendLongTextCalls++
	return nil
}

func (m *orderedSplitTerminal) Notify(context.Context, string) error { return nil }
func (m *orderedSplitTerminal) ReadScreen(context.Context, terminal.PaneID, terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (m *orderedSplitTerminal) PipePaneStart(context.Context, terminal.PaneID, string) error {
	return nil
}
func (m *orderedSplitTerminal) PipePaneStop(context.Context, terminal.PaneID) error { return nil }

func (m *orderedSplitTerminal) Close(_ context.Context, ref string) error {
	if err := appendTransportEvent(m.eventsPath, "close:"+ref); err != nil {
		return err
	}
	return os.WriteFile(m.cleanupGate, []byte("closed"), 0o600)
}

func appendTransportEvent(path, event string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.WriteString(event + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func newOrderedFallbackProvider(binary, name, eventsPath, gatePath string) ProviderConfig {
	return ProviderConfig{
		Name: name, Binary: binary, Args: []string{eventsPath, gatePath, name}, OutputFormat: "text",
	}
}

func TestRunPaneOrchestra_PartialSplitCleansBeforeAllowedFallback(t *testing.T) {
	isolateSurfaceTracker(t)
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.log")
	gatePath := filepath.Join(dir, "cleanup.done")
	binary, err := os.Executable()
	require.NoError(t, err)
	term := &orderedSplitTerminal{eventsPath: eventsPath, cleanupGate: gatePath}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			newOrderedFallbackProvider(binary, "first", eventsPath, gatePath),
			newOrderedFallbackProvider(binary, "second", eventsPath, gatePath),
		},
		Strategy: StrategyConsensus, Prompt: "fallback only after cleanup",
		TimeoutSeconds: 3, Terminal: term,
	}
	originalNewCommand := newCommand
	t.Cleanup(func() { newCommand = originalNewCommand })
	newCommand = func(_ context.Context, _ string, args ...string) command {
		waitCh := make(chan error, 1)
		waitCh <- nil
		if len(args) != 3 {
			return &fakeCommand{
				waitCh: waitCh,
				startFn: func(*fakeCommand) error {
					return fmt.Errorf("unexpected provider args: %v", args)
				},
			}
		}
		return &fakeCommand{
			waitCh:   waitCh,
			exitCode: 0,
			startFn: func(cmd *fakeCommand) error {
				if _, err := os.Stat(args[1]); errors.Is(err, os.ErrNotExist) {
					if err := appendTransportEvent(args[0], "subprocess-before-cleanup"); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
				if err := appendTransportEvent(args[0], "subprocess:"+args[2]); err != nil {
					return err
				}
				_, err := io.WriteString(cmd.stdout, "fallback output\n")
				return err
			},
		}
	}

	result, err := RunPaneOrchestra(context.Background(), cfg)

	require.NoError(t, err)
	require.NotNil(t, result)
	data, readErr := os.ReadFile(eventsPath)
	require.NoError(t, readErr)
	events := strings.Fields(string(data))
	require.GreaterOrEqual(t, len(events), 5)
	assert.Equal(t, []string{"split:first", "split:second-error", "close:surface:101"}, events[:3])
	assert.NotContains(t, events, "subprocess-before-cleanup")
	assert.FileExists(t, gatePath)
	assert.Zero(t, term.sendCommandCalls+term.sendLongTextCalls, "no provider command may launch in a partial pane set")
	assert.Contains(t, result.Summary, "SplitPane for second: second split failed",
		"the successful subprocess fallback must preserve the original pane failure")
	assert.Contains(t, result.DegradedReasons, "pane_partial_split_cleanup")
	require.NotNil(t, result.RunReceipt)
	assert.Contains(t, result.RunReceipt.DegradedReasons, "pane_partial_split_cleanup",
		"the machine-readable receipt must expose the partial-pane degradation")
}

func TestRunPaneOrchestra_DebateWithPaneTerminal_DoesNotRequireInteractiveFlag(t *testing.T) {
	term := newCmuxMock()
	term.splitPaneErr = errors.New("debate pane unavailable")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "use the attached pane terminal",
		TimeoutSeconds: 3,
		Terminal:       term,
		FallbackMode:   FallbackModeAbort,
		NoJudge:        true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result, err := RunPaneOrchestra(ctx, cfg)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Len(t, term.splitPaneCalls, 1,
		"pane transport selection must not depend on the interactive-session flag")
	assert.Contains(t, err.Error(), "debate pane unavailable")
}

func TestRunJudgeRound_EmptySplitErrorAllowsSubprocessFallback(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, marker := newPaneBoundaryMarkerProvider(t, "judge-fixture", "")
	term := &paneCommitTerminal{splitErr: errors.New("judge split unavailable")}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, JudgeProvider: provider.Name,
		Terminal: term, TimeoutSeconds: 3,
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil,
		[]ProviderResponse{{Provider: "claude", Output: "candidate"}}, 1)

	require.NotNil(t, resp)
	assert.Equal(t, 1, term.splitCalls)
	assert.Equal(t, "subprocess", resp.ExecutedBackend)
	assert.FileExists(t, marker, "an empty SplitPane failure remains eligible for subprocess fallback")
	assert.Empty(t, term.closed)
}
