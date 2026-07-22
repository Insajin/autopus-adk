package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cancelAfterReadsTerminal struct {
	*seqScreenMock
	cancel    context.CancelFunc
	stopAfter int
}

func (m *cancelAfterReadsTerminal) ReadScreen(ctx context.Context, paneID terminal.PaneID, opts terminal.ReadScreenOpts) (string, error) {
	screen, err := m.seqScreenMock.ReadScreen(ctx, paneID, opts)
	if m.readCalls >= m.stopAfter {
		m.cancel()
	}
	return screen, err
}

func TestWaitForPaneReady_EarlyHookReadyWithoutInputPrompt_ReturnsFalse(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-early-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("codex", 1, "ready")),
		[]byte("1"), 0o600,
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	term := &cancelAfterReadsTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{"starting codex...\n"}},
		cancel:        cancel,
		stopAfter:     1,
	}
	ready := waitForPaneReady(
		ctx, term, terminal.PaneID("surface:1"),
		SessionReadyPatterns(), time.Second, session, "codex", 1,
	)

	assert.False(t, ready, "hook startup alone must not prove the TUI can receive input")
	assert.Equal(t, 1, term.readCalls, "the gate must inspect the provider screen")
}

func TestWaitForPaneReady_EarlyHookReadyWaitsForInputPrompt(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-prompt-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("codex", 1, "ready")),
		[]byte("1"), 0o600,
	))

	term := &seqScreenMock{name: "cmux", screens: []string{
		"starting codex...\n",
		"PostCompact hooks\nTurn hooks on or off\nNo hooks installed for this event.\nPress esc to go back\n",
		"› Find and fix a bug in @filename\n",
		"› Find and fix a bug in @filename\n",
	}}
	ready := waitForPaneReady(
		context.Background(), term, terminal.PaneID("surface:2"),
		SessionReadyPatterns(), time.Second, session, "codex", 1,
	)

	assert.True(t, ready)
	assert.Equal(t, 4, term.readCalls, "the hook signal must not bypass stable screen readiness")
}

func TestIsSessionReady_CodexHooksModal_ReturnsFalse(t *testing.T) {
	t.Parallel()

	screen := "› Summarize recent commits\nPostCompact hooks\nTurn hooks on or off\nNo hooks installed for this event.\nPress esc to go back\n"
	assert.False(t, isSessionReady(screen, SessionReadyPatterns()))
}

func TestIsProviderSessionReady_CustomProviderUsesGenericCLIPrompt(t *testing.T) {
	t.Parallel()

	patterns := SessionReadyPatterns()
	assert.True(t, isProviderSessionReady("❯\n", patterns, "custom-provider"))
	assert.False(t, isProviderSessionReady("❯\n", patterns, "codex"),
		"a known provider must not accept another provider's prompt")
}

func TestWaitForPaneReady_TransientPromptFrame_ReturnsFalse(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-transient-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("codex", 1, "ready")),
		[]byte("1"), 0o600,
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	term := &cancelAfterReadsTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{
			"starting codex...\n",
			"› Summarize recent commits\n",
			"loading extensions...\n",
		}},
		cancel: cancel, stopAfter: 3,
	}

	ready := waitForPaneReady(ctx, term, terminal.PaneID("surface:3"),
		SessionReadyPatterns(), time.Second, session, "codex", 1)

	assert.False(t, ready, "one transient prompt frame must not open the input gate")
	assert.Equal(t, 3, term.readCalls)
}

func TestWaitForPaneReady_OtherProviderPrompt_ReturnsFalse(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-other-provider-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("codex", 1, "ready")),
		[]byte("1"), 0o600,
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	term := &cancelAfterReadsTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{"❯\n", "❯\n"}},
		cancel:        cancel, stopAfter: 2,
	}

	ready := waitForPaneReady(ctx, term, terminal.PaneID("surface:4"),
		SessionReadyPatterns(), time.Second, session, "codex", 1)

	assert.False(t, ready, "a Claude prompt must not make a Codex pane ready")
}

func TestWaitForPaneReady_HookProviderRequiresMatchingRoundSignal(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-wrong-round-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("codex", 1, "ready")),
		[]byte("1"), 0o600,
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	term := &cancelAfterReadsTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{
			"› Summarize recent commits\n", "› Summarize recent commits\n",
		}},
		cancel: cancel, stopAfter: 2,
	}

	ready := waitForPaneReady(ctx, term, terminal.PaneID("surface:5"),
		SessionReadyPatterns(), time.Second, session, "codex", 2)

	assert.False(t, ready, "a ready signal from another round must not open the gate")
}

func TestWaitForPaneReady_HooklessProviderUsesStableScreen(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-hookless-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	term := &seqScreenMock{name: "cmux", screens: []string{"Ask anything\n", "Ask anything\n"}}
	ready := waitForPaneReady(context.Background(), term, terminal.PaneID("surface:6"),
		SessionReadyPatterns(), time.Second, session, "opencode", 1)

	assert.True(t, ready, "a hookless provider must retain provider-specific screen fallback")
	assert.Equal(t, 2, term.readCalls)
}

func TestWaitForPaneReady_ClaudeANSIAndNBSPPrompt_IsStableReady(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-claude-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	require.NoError(t, os.WriteFile(
		filepath.Join(session.Dir(), RoundSignalName("claude", 1, "ready")),
		[]byte("1"), 0o600,
	))

	const screen = "\x1b[32m❯\x1b[0m\u00a0Try \"write a test for <filepath>\"\n"
	term := &seqScreenMock{name: "cmux", screens: []string{screen, screen}}
	ready := waitForPaneReady(context.Background(), term, terminal.PaneID("surface:7"),
		SessionReadyPatterns(), time.Second, session, "claude", 1)

	assert.True(t, ready)
	assert.Equal(t, 2, term.readCalls, "Claude prompt must be stable across two frames")
}

func TestWaitForSessionReady_RequiresProviderSpecificStableFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		screens []string
	}{
		{
			name:    "other provider prompt",
			screens: []string{"❯\n", "❯\n"},
		},
		{
			name:    "transient provider prompt",
			screens: []string{"› Summarize recent commits\n", "loading extensions...\n"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			term := &cancelAfterReadsTerminal{
				seqScreenMock: &seqScreenMock{name: "cmux", screens: tt.screens},
				cancel:        cancel,
				stopAfter:     len(tt.screens),
			}
			panes := []paneInfo{{
				provider: ProviderConfig{Name: "codex", StartupTimeout: time.Second},
				paneID:   terminal.PaneID("surface:stable-gate"),
			}}

			failed := waitForSessionReady(ctx, term, panes)

			require.Len(t, failed, 1)
			assert.Equal(t, "codex", failed[0].Name)
			assert.True(t, panes[0].skipWait)
		})
	}
}

func TestWaitForSessionReadyWithHook_MissingReadySignalSkipsPrompt(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("pane-ready-wrapper-hook-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const screen = "❯ Try \"write a test for <filepath>\"\n"
	term := &cancelAfterReadsTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{screen, screen}},
		cancel:        cancel,
		stopAfter:     2,
	}
	panes := []paneInfo{{
		provider: ProviderConfig{Name: "claude", StartupTimeout: time.Second},
		paneID:   terminal.PaneID("surface:hook-gate"),
	}}

	failed := waitForSessionReadyWithHook(ctx, term, panes, session, 0)
	promptFailed := sendPrompts(ctx, OrchestraConfig{Terminal: term, Prompt: "must not send"}, panes)

	require.Len(t, failed, 1)
	assert.Equal(t, "claude", failed[0].Name)
	assert.True(t, panes[0].skipWait)
	assert.Empty(t, promptFailed)
	assert.Empty(t, term.longTextsSnapshot(), "a readiness failure must block prompt delivery")
}
