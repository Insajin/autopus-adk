package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})
	req := ProviderRequest{
		Provider: "claude",
		Config:   ProviderConfig{Name: "claude", Binary: "claude"},
		Prompt:   "review this",
		Timeout:  1 * time.Second,
	}
	_, _ = b.Execute(context.Background(), req)

	for _, c := range mock.commands {
		require.NotContains(t, c, "export AUTOPUS_SESSION_ID=",
			"Execute must not export AUTOPUS_SESSION_ID when HookMode is off")
	}
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
