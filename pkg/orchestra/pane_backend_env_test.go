package orchestra

import (
	"context"
	"os"
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
	defer os.RemoveAll("/tmp/autopus/" + sid)

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
