package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SPEC-ORCH-013 R2: ReadScreen Baseline ---

// TestWaitForCompletion_Baseline_PrevRoundPromptIgnored verifies that
// waitForCompletion does not false-positive on previous round's prompt.
// S3: If screen has not changed from baseline, completion is NOT detected.
func TestWaitForCompletion_Baseline_PrevRoundPromptIgnored(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	// Screen shows prompt from previous round — identical to baseline
	mock.readScreenOutput = "❯\n"
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// R2: pass baseline matching current screen — completion should NOT be detected
	baseline := "❯\n"
	result := waitForCompletion(ctx, mock, pi, patterns, baseline)
	assert.False(t, result,
		"must not false-positive when screen matches previous round baseline")
}

// TestWaitForCompletion_Baseline_ScreenChangeDetectsCompletion verifies
// completion is detected after screen changes from baseline.
// S4: Screen content changes from baseline, then shows prompt -> completion.
func TestWaitForCompletion_Baseline_ScreenChangeDetectsCompletion(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	// Screen shows new content + prompt — different from baseline
	mock.readScreenOutput = "new output from AI\n❯\n"
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// R2: baseline differs from current screen — completion should be detected
	baseline := "old prompt from last round\n❯\n"
	result := waitForCompletion(ctx, mock, pi, patterns, baseline)
	assert.True(t, result,
		"must detect completion when screen changes from baseline and shows prompt")
}

// TestWaitForCompletion_Baseline_EmptyBaseline verifies empty baseline
// does not suppress completion detection.
func TestWaitForCompletion_Baseline_EmptyBaseline(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Empty baseline should not block completion
	result := waitForCompletion(ctx, mock, pi, patterns, "")
	assert.True(t, result, "empty baseline must not suppress completion detection")
}

// TestWaitForCompletion_Baseline_SpecialChars verifies baseline with special
// characters is compared correctly.
func TestWaitForCompletion_Baseline_SpecialChars(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	// Screen changed from baseline with special chars
	mock.readScreenOutput = "new response\n❯\n"
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	baseline := "previous\x1b[31m output\x1b[0m with ANSI\n❯\n"
	result := waitForCompletion(ctx, mock, pi, patterns, baseline)
	assert.True(t, result,
		"baseline with special chars must be compared correctly")
}

// TestSendPrompts_SkipsArgsProvider verifies providers with InteractiveInput="args"
// are skipped during prompt sending.
func TestSendPrompts_SkipsArgsProvider(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	argsProvider := ProviderConfig{
		Name: "opencode", Binary: "opencode",
		InteractiveInput: "args",
	}
	stdinProvider := ProviderConfig{
		Name: "claude", Binary: "claude",
	}
	panes := []paneInfo{
		{provider: argsProvider, paneID: "pane-1"},
		{provider: stdinProvider, paneID: "pane-2"},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{argsProvider, stdinProvider},
		Terminal:  mock,
		Prompt:    "test prompt",
	}

	failed := sendPrompts(context.Background(), cfg, panes)
	assert.Empty(t, failed)
	// Only stdin provider should have received SendLongText
	assert.Len(t, mock.sendLongTextCalls, 1)
	assert.Equal(t, "pane-2", string(mock.sendLongTextCalls[0].PaneID))
}

// TestSendPrompts_SkipsSkipWaitProvider verifies skipWait providers are skipped.
func TestSendPrompts_SkipsSkipWaitProvider(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "p1", Binary: "echo"}, paneID: "pane-1", skipWait: true},
		{provider: ProviderConfig{Name: "p2", Binary: "echo"}, paneID: "pane-2"},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{{Name: "p1"}, {Name: "p2"}},
		Terminal:  mock,
		Prompt:    "test",
	}

	failed := sendPrompts(context.Background(), cfg, panes)
	assert.Empty(t, failed)
	// Only non-skipWait provider should get prompt
	assert.Len(t, mock.sendLongTextCalls, 1)
}

// TestLaunchInteractiveSessions_SendLongTextError verifies SendLongText failure
// is recorded as a failed provider.
func TestLaunchInteractiveSessions_SendLongTextError(t *testing.T) {
	t.Parallel()
	mock := &sendLongTextErrorMock{mockTerminal: mockTerminal{name: "cmux"}}
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "p1", Binary: "echo"}, paneID: "pane-1"},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{{Name: "p1", Binary: "echo"}},
		Terminal:  mock,
		Prompt:    "test",
	}

	failed := launchInteractiveSessions(context.Background(), cfg, panes)
	assert.Len(t, failed, 1)
	assert.Equal(t, "p1", failed[0].Name)
	assert.True(t, panes[0].skipWait, "failed launch should set skipWait")
}

// TestLaunchInteractiveSessions_ArgsMode verifies providers with InteractiveInput="args"
// receive the prompt as a CLI argument at launch.
func TestLaunchInteractiveSessions_ArgsMode(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	argsProvider := ProviderConfig{
		Name: "opencode", Binary: "opencode",
		PaneArgs:         []string{"run", "-m", "gpt-5.4"},
		InteractiveInput: "args",
	}
	panes := []paneInfo{
		{provider: argsProvider, paneID: "pane-1"},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{argsProvider},
		Terminal:  mock,
		Prompt:    "fix the bug",
	}

	failed := launchInteractiveSessions(context.Background(), cfg, panes)
	assert.Empty(t, failed)
	require.NotEmpty(t, mock.sendLongTextCalls)
	// The launch command should include the prompt
	assert.Contains(t, mock.sendLongTextCalls[0].Text, "fix the bug")
}
