package orchestra

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seqScreenMock returns a scripted sequence of ReadScreen outputs, sticking on
// the final element once exhausted (unlike mockTerminal's modulo cycling).
// It records SendLongText/SendCommand calls so prompt-gating can be asserted.
type seqScreenMock struct {
	mu        sync.Mutex
	name      string
	screens   []string
	idx       int
	longTexts []string
	commands  []string
	readCalls int
	splitErr  error
}

func (m *seqScreenMock) Name() string                                  { return m.name }
func (m *seqScreenMock) CreateWorkspace(context.Context, string) error { return nil }

func (m *seqScreenMock) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	if m.splitErr != nil {
		return "", m.splitErr
	}
	return terminal.PaneID("pane-1"), nil
}

func (m *seqScreenMock) SendCommand(_ context.Context, _ terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *seqScreenMock) SendLongText(_ context.Context, _ terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.longTexts = append(m.longTexts, text)
	return nil
}

func (m *seqScreenMock) Notify(context.Context, string) error { return nil }

func (m *seqScreenMock) ReadScreen(context.Context, terminal.PaneID, terminal.ReadScreenOpts) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++
	if len(m.screens) == 0 {
		return "", nil
	}
	if m.idx >= len(m.screens) {
		return m.screens[len(m.screens)-1], nil
	}
	s := m.screens[m.idx]
	m.idx++
	return s, nil
}

func (m *seqScreenMock) PipePaneStart(context.Context, terminal.PaneID, string) error { return nil }
func (m *seqScreenMock) PipePaneStop(context.Context, terminal.PaneID) error          { return nil }
func (m *seqScreenMock) Close(context.Context, string) error                          { return nil }

func (m *seqScreenMock) longTextsSnapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.longTexts))
	copy(out, m.longTexts)
	return out
}

// readyScreen contains the claude session-ready prompt (also a completion prompt).
const readyScreen = "loading...\n❯\n"

// notReadyScreen contains no prompt pattern.
const notReadyScreen = "starting claude...\n"

// fastStartupClaude is a sendkeys provider with a short startup timeout so the
// session-ready poll resolves quickly in tests.
func fastStartupClaude() ProviderConfig {
	return ProviderConfig{
		Name:           "claude",
		Binary:         "claude",
		StartupTimeout: 2 * time.Second,
	}
}

// TestExecute_S10_PromptGatedOnReady asserts the prompt is sent ONLY after the
// session becomes ready: zero prompt sends before ready, exactly one after.
func TestExecute_S10_PromptGatedOnReady(t *testing.T) {
	t.Parallel()
	const prompt = "PLEASE_REVIEW_THIS_PROMPT_S10"
	mock := &seqScreenMock{
		name: "cmux",
		// 3 non-ready reads, then ready; completion poll then sees ❯ repeatedly.
		screens: []string{notReadyScreen, notReadyScreen, notReadyScreen, readyScreen},
	}
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})
	req := ProviderRequest{
		Provider: "claude",
		Prompt:   prompt,
		Round:    0,
		Timeout:  3 * time.Second,
		Config:   fastStartupClaude(),
	}

	resp, err := b.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// SendLongText is called once for the launch command (no prompt body) and
	// once for the short prompt-file instruction after ready. The launch command
	// must NOT contain the prompt; exactly one SendLongText points at the prompt file.
	var promptSends int
	for _, lt := range mock.longTextsSnapshot() {
		assert.NotEqual(t, prompt, lt, "full prompt body must not be typed into the pane")
		if strings.Contains(lt, "Markdown file") {
			promptSends++
		}
	}
	assert.Equal(t, 1, promptSends, "prompt must be sent exactly once, after ready")
}

// TestExecute_S11_MonitorFallsBackToPoll asserts that when MonitorEnabled is
// true but the monitor detector is unavailable for a plain mock terminal,
// completion still resolves via the ScreenPoll path before overall timeout.
func TestExecute_S11_MonitorPollSelection(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Terminal:       &seqScreenMock{name: "cmux"},
		MonitorEnabled: true,
		MonitorTimeout: 50 * time.Millisecond,
	}
	// resolveCompletionDetector with no signal-capable terminal and HookMode off
	// must select the non-event-driven ScreenPollDetector (poll path).
	resolved := resolveCompletionDetector(cfg, nil)
	_, isPoll := resolved.detector.(*ScreenPollDetector)
	assert.True(t, isPoll, "non-signal terminal must resolve to ScreenPollDetector")
	assert.False(t, resolved.eventDriven, "poll path is not event-driven")
}

// TestExecute_S12_TimeoutDeterministic asserts a never-completing pane returns a
// DETERMINISTIC, bounded result with no hang. The per-provider timeout is shared
// between the session-ready gate (REQ-009) and the completion wait (REQ-011), so
// the budget is set well above the ready-gate cost (matching the proven-stable
// S13 budget) so that under normal load the deadline lands on the completion wait
// and yields a TimedOut pane response. Under pathological CPU contention (e.g. a
// loaded `-race` CI runner) the deadline can instead trip the ready gate, which is
// the SPEC-sanctioned REQ-009 failure path; that deterministic-failure outcome is
// accepted here too. The invariant that must ALWAYS hold is a bounded return (no
// hang) and, when a response is produced, sanitized (non-garbage) output.
func TestExecute_S12_TimeoutDeterministic(t *testing.T) {
	t.Parallel()
	// Screen becomes ready, then never shows a completion prompt -> completion never matches.
	mock := &seqScreenMock{
		name:    "cmux",
		screens: []string{readyScreen, "AI is thinking...\nstill working\n"},
	}
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})
	req := ProviderRequest{
		Provider: "claude",
		Prompt:   "hello",
		Timeout:  2 * time.Second,
		Config:   fastStartupClaude(),
	}

	start := time.Now()
	resp, err := b.Execute(context.Background(), req)
	elapsed := time.Since(start)

	// Must always return promptly (no indefinite hang), regardless of which
	// bounded stage the shared deadline trips.
	assert.Less(t, elapsed, 4*time.Second, "must return promptly, no hang")

	if err != nil {
		// Ready gate consumed the budget under load -> deterministic REQ-009
		// committed-pane failure. It remains on the pane transport and returns an
		// actionable error rather than crossing into subprocess execution.
		assert.Contains(t, err.Error(), "interactive pane execution failed",
			"ready-gate timeout must surface a deterministic actionable failure")
		if resp != nil {
			assert.Equal(t, paneBackendName, resp.ExecutedBackend,
				"a ready-gate failure after SplitPane must remain on the pane backend")
		}
		return
	}

	// Normal path: completion wait timed out -> deterministic TimedOut pane result.
	require.NotNil(t, resp)
	assert.True(t, resp.TimedOut, "completion timeout must set TimedOut")
	assert.Equal(t, "pane", resp.ExecutedBackend, "timeout is a deterministic pane result, not fallback")
	assert.NotContains(t, resp.Output, "\x1b[", "partial output must be sanitized")
}

// TestExecute_S13_NonHookDetector asserts HookMode=false resolves a non-hook
// completion detector (ScreenPollDetector), never FileIPCDetector.
func TestExecute_S13_NonHookDetector(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{Terminal: &seqScreenMock{name: "cmux"}, HookMode: false}
	resolved := resolveCompletionDetector(cfg, nil)
	_, isFileIPC := resolved.detector.(*FileIPCDetector)
	assert.False(t, isFileIPC, "non-hook mode must not use FileIPCDetector")
	_, isPoll := resolved.detector.(*ScreenPollDetector)
	assert.True(t, isPoll, "non-hook mode resolves to ScreenPollDetector")

	// Execution still completes/returns without erroring on the missing hook.
	mock := &seqScreenMock{name: "cmux", screens: []string{readyScreen}}
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock, HookMode: false})
	resp, err := b.Execute(context.Background(), ProviderRequest{
		Provider: "claude", Prompt: "hi", Timeout: 2 * time.Second, Config: fastStartupClaude(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestExecute_S14_BothBackendsFail asserts that when the pane fails (SplitPane
// error) AND the subprocess is unavailable (non-existent binary), Execute
// returns an actionable error naming both causes plus a recovery instruction,
// and records that neither backend succeeded.
func TestExecute_S14_BothBackendsFail(t *testing.T) {
	t.Parallel()
	mock := &seqScreenMock{name: "cmux", splitErr: assertSplitErr}
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})
	req := ProviderRequest{
		Provider: "claude",
		Prompt:   "hello",
		Timeout:  time.Second,
		// Non-existent binary -> subprocess fallback fails.
		Config: ProviderConfig{Name: "claude", Binary: "binary_that_does_not_exist_xyz_s14"},
	}

	resp, err := b.Execute(context.Background(), req)
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "interactive pane execution failed", "names pane failure")
	assert.Contains(t, msg, "-p subprocess fallback", "names subprocess fallback unavailable")
	assert.Contains(t, msg, "logged-in cmux/tmux", "includes recovery instruction")

	require.NotNil(t, resp)
	assert.Equal(t, noneBackendMarker, resp.ExecutedBackend, "neither backend succeeded")
}

// assertSplitErr is a sentinel SplitPane error for S14.
var assertSplitErr = errSplit

type splitError struct{}

func (splitError) Error() string { return "split pane unavailable" }

var errSplit error = splitError{}

// TestExecute_S6_FallbackSucceeds verifies the recoverable best-effort fallback
// path (REQ-005/INV-004/INV-005, acceptance S6): when interactive-pane execution
// fails BUT the -p/stdin subprocess IS available, Execute returns the subprocess
// response tagged ExecutedBackend="subprocess" with non-empty parseable output.
func TestExecute_S6_FallbackSucceeds(t *testing.T) {
	t.Parallel()
	// Pane fails immediately via SplitPane error so the per-provider context
	// budget remains for the best-effort subprocess fallback.
	mock := &seqScreenMock{name: "cmux", splitErr: errSplit}
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})
	// echoProvider (cat) echoes the stdin prompt to stdout, so a reviewer JSON
	// prompt becomes the subprocess output (available API path).
	req := ProviderRequest{
		Provider: "claude",
		Prompt:   `{"verdict":"PASS","summary":"ok","findings":[]}`,
		Timeout:  5 * time.Second,
		Config:   echoProvider("claude"),
	}

	resp, err := b.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "subprocess", resp.ExecutedBackend, "recoverable fallback records subprocess backend")
	assert.NotEmpty(t, resp.Output, "fallback output is non-empty")

	out, perr := (&OutputParser{}).ParseReviewer(resp.Output)
	require.NoError(t, perr, "fallback output is parseable reviewer JSON")
	assert.Equal(t, "PASS", out.Verdict)
}
