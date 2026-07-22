package orchestra

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hooklessExportRejectTerminal struct {
	*backendHookExportTerminal
}

func (t *hooklessExportRejectTerminal) SendCommand(ctx context.Context, paneID terminal.PaneID, command string) error {
	if strings.Contains(command, "AUTOPUS_") {
		return errors.New("unexpected hook environment export")
	}
	return t.backendHookExportTerminal.SendCommand(ctx, paneID, command)
}

func TestInteractivePaneBackendExecute_HooklessProviderSkipsHookEnvExport(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	off := false
	provider := ProviderConfig{
		Name: "opencode", Binary: "opencode", InteractiveInput: "args",
		HasHook: &off, HasStartupHook: &off,
	}
	term := &hooklessExportRejectTerminal{backendHookExportTerminal: &backendHookExportTerminal{
		seqScreenMock: &seqScreenMock{name: "cmux", screens: []string{readyScreen}},
	}}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal: term, HookMode: true, SessionID: "hookless-backend-" + NewSessionID(),
		Providers: []ProviderConfig{provider}, InitialDelay: time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	})

	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: provider.Name, Config: provider, Prompt: "review this", Timeout: 500 * time.Millisecond,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, term.longTextsSnapshot(), "hookless provider must still launch")
	for _, event := range term.eventsSnapshot() {
		assert.False(t,
			event.kind == "command" && strings.Contains(event.value, "AUTOPUS_"),
			"hookless provider must not receive hook environment exports: %+v", event,
		)
	}
}
