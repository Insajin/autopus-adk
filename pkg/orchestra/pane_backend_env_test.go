package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type backendHookExportTerminal struct {
	*seqScreenMock
	mu            sync.Mutex
	events        []backendHookExportEvent
	commandCalls  int
	failCommandAt int
}

type backendHookExportEvent struct {
	kind  string
	value string
}

func (m *backendHookExportTerminal) SendCommand(_ context.Context, _ terminal.PaneID, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandCalls++
	m.events = append(m.events, backendHookExportEvent{kind: "command", value: cmd})
	if m.commandCalls == m.failCommandAt {
		return errors.New("injected hook export failure")
	}
	return nil
}

func (m *backendHookExportTerminal) SendLongText(ctx context.Context, paneID terminal.PaneID, text string) error {
	m.mu.Lock()
	m.events = append(m.events, backendHookExportEvent{kind: "long_text", value: text})
	m.mu.Unlock()
	return m.seqScreenMock.SendLongText(ctx, paneID, text)
}

func (m *backendHookExportTerminal) eventsSnapshot() []backendHookExportEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]backendHookExportEvent(nil), m.events...)
}

// TestExecute_ExportsSessionEnvWhenHookMode verifies SPEC-ORCH-022: the
// structured spec-review / orchestra-run paths drive InteractivePaneBackend.Execute
// directly (not RunInteractivePaneOrchestra), so the AUTOPUS_SESSION_ID export
// that the orchestra path performs in launchInteractiveSessions must be mirrored
// inside Execute. Without it the provider's Stop/AfterAgent hook sees no session
// ID and never writes the done-file, so FileIPCDetector cannot collect results.
func TestExecute_ExportsSessionEnvWhenHookMode(t *testing.T) {
	t.Parallel()
	const sid = "orch-test-sendenv"
	defer func() { _ = os.RemoveAll("/tmp/autopus/" + sid) }()

	mock := &seqScreenMock{name: "cmux", screens: []string{"❯ "}}
	b := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:  mock,
		HookMode:  true,
		SessionID: sid,
	})
	req := ProviderRequest{
		Provider: "claude",
		Config:   ProviderConfig{Name: "claude", Binary: "claude"},
		Prompt:   "review this",
		Timeout:  1 * time.Second, // bound so the FileIPC wait does not block the test
	}
	_, _ = b.Execute(context.Background(), req)

	found := false
	for _, c := range mock.commands {
		if strings.Contains(c, "export AUTOPUS_SESSION_ID="+sid) {
			found = true
			break
		}
	}
	require.True(t, found,
		"Execute must export AUTOPUS_SESSION_ID into the pane when HookMode is on; sent commands: %v", mock.commands)
}

// TestExecute_NoSessionEnvWhenHookModeOff verifies the export is gated: with
// HookMode off, no AUTOPUS_SESSION_ID export is sent (subprocess/screen-poll
// path is unaffected).
func TestExecute_NoSessionEnvWhenHookModeOff(t *testing.T) {
	t.Parallel()
	mock := &seqScreenMock{name: "cmux", screens: []string{"❯ "}}
	provider := ProviderConfig{Name: "claude", Binary: "claude", InteractiveInput: "args"}
	b := NewInteractivePaneBackend(OrchestraConfig{
		Terminal: mock, WorkingDir: t.TempDir(), InitialDelay: time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	})
	req := ProviderRequest{
		Provider: "claude",
		Config:   provider,
		Prompt:   "review this",
		Timeout:  3 * time.Second,
	}
	resp, err := b.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, mock.longTextsSnapshot(), "non-hook execution must still launch the provider")

	for _, c := range mock.commands {
		require.NotContains(t, c, "export AUTOPUS_SESSION_ID=",
			"Execute must not export AUTOPUS_SESSION_ID when HookMode is off")
	}
}

func TestInteractivePaneBackendExecute_HookExportFailure_ReturnsBeforeLaunchAndCollection(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	tests := []struct {
		name          string
		failCommandAt int
		wantError     string
	}{
		{name: "session send", failCommandAt: 1, wantError: "export hook session failed"},
		{name: "session enter", failCommandAt: 2, wantError: "commit hook session export failed"},
		{name: "round send", failCommandAt: 3, wantError: "export hook round failed"},
		{name: "round enter", failCommandAt: 4, wantError: "commit hook round export failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := "pane-backend-export-" + NewSessionID()
			term := &backendHookExportTerminal{
				seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{readyScreen}},
				failCommandAt: tt.failCommandAt,
			}
			provider := ProviderConfig{Name: "claude", Binary: "claude", InteractiveInput: "args"}
			backend := NewInteractivePaneBackend(OrchestraConfig{
				Terminal: term, HookMode: true, SessionID: sessionID,
				Providers: []ProviderConfig{provider}, InitialDelay: time.Millisecond,
				CompletionDetector: &stubCompletionDetector{completed: true},
			})

			resp, err := backend.Execute(context.Background(), ProviderRequest{
				Provider: "claude", Config: provider, Prompt: "review this", Round: 2,
			})

			require.Error(t, err)
			require.NotNil(t, resp)
			assert.ErrorContains(t, err, tt.wantError)
			assert.Contains(t, resp.Error, tt.wantError)
			assert.Equal(t, paneBackendName, resp.ExecutedBackend)
			assert.Empty(t, term.longTextsSnapshot(), "provider launch must not run after hook export failure")
			assert.Zero(t, term.readCalls, "collector and readiness polling must not run after hook export failure")
		})
	}
}

func TestInteractivePaneBackendExecute_HookExports_PrecedeProviderLaunch(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	sessionID := "pane-backend-order-" + NewSessionID()
	term := &backendHookExportTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{readyScreen}},
	}
	provider := ProviderConfig{Name: "claude", Binary: "claude", InteractiveInput: "args"}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal: term, HookMode: true, SessionID: sessionID,
		Providers: []ProviderConfig{provider}, InitialDelay: time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	})

	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "claude", Config: provider, Prompt: "review this", Round: 2,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	events := term.eventsSnapshot()
	require.GreaterOrEqual(t, len(events), 5)
	assert.Equal(t, "command", events[0].kind)
	assert.Contains(t, events[0].value, "export AUTOPUS_SESSION_ID="+sessionID)
	assert.Equal(t, backendHookExportEvent{kind: "command", value: "\n"}, events[1])
	assert.Equal(t, backendHookExportEvent{kind: "command", value: "export AUTOPUS_ROUND=2"}, events[2])
	assert.Equal(t, backendHookExportEvent{kind: "command", value: "\n"}, events[3])
	assert.Equal(t, "long_text", events[4].kind, "provider launch must follow both committed hook exports")
}

func TestExecute_DoesNotCleanupSharedHookSession(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	const sid = "orch-test-shared-hook-owner"
	owner, err := NewHookSession(sid)
	require.NoError(t, err)
	defer owner.Cleanup()
	sentinel := filepath.Join(owner.Dir(), "sibling-owner-active")
	require.NoError(t, os.WriteFile(sentinel, []byte("active"), 0o600))

	provider := ProviderConfig{Name: "claude", Binary: "claude", InteractiveInput: "args"}
	mock := &seqScreenMock{name: "cmux", screens: []string{readyScreen}}
	b := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:           mock,
		HookMode:           true,
		SessionID:          sid,
		WorkingDir:         t.TempDir(),
		Providers:          []ProviderConfig{provider},
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	})
	req := ProviderRequest{
		Provider: "claude",
		Config:   provider,
		Prompt:   "review this",
		Role:     "reviewer",
		Timeout:  5 * time.Second,
	}

	resp, err := b.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.DirExists(t, owner.Dir(),
		"provider-level Execute must not remove the shared hook session while sibling providers may still be running")
	require.FileExists(t, sentinel, "provider-level Execute must preserve sibling-owned artifacts")
	require.NoError(t, owner.WriteInputRound("claude", 2, "sibling still active"),
		"the outer owner must remain usable after provider-level Execute returns")
}

func TestExecute_CodexPromptUsesSendkeysAfterReady(t *testing.T) {
	t.Parallel()

	mock := &seqScreenMock{name: "cmux", screens: []string{"› Summarize recent commits\n"}}
	provider := ProviderConfig{Name: "codex", Binary: "codex", PaneArgs: []string{"-m", "gpt-5.4"}}
	b := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:     mock,
		WorkingDir:   t.TempDir(),
		InitialDelay: time.Millisecond,
	})
	req := ProviderRequest{
		Provider: "codex",
		Config:   provider,
		Prompt:   "review this",
		Role:     "reviewer",
		Timeout:  3 * time.Second,
	}

	resp, err := b.Execute(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	var promptCommandFound bool
	for _, c := range mock.commands {
		if strings.Contains(c, "Markdown file") && strings.Contains(c, "AUTOPUS_RESPONSE_BEGIN") {
			promptCommandFound = true
		}
	}
	require.True(t, promptCommandFound, "codex prompt-file instruction must be sent via SendCommand/sendkeys so the TUI can submit it reliably")
	require.Len(t, mock.longTexts, 1, "codex should use SendLongText only for the launch command, not for the prompt-file instruction")
}
