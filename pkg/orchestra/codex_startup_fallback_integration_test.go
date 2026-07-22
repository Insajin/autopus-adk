package orchestra

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var codexFallbackResponsePath = regexp.MustCompile(`response file: ([^ ]+?\.md)`)

type codexStartupFallbackTerminal struct {
	*seqScreenMock
}

func (m *codexStartupFallbackTerminal) SendCommand(
	ctx context.Context,
	paneID terminal.PaneID,
	command string,
) error {
	if err := m.seqScreenMock.SendCommand(ctx, paneID, command); err != nil {
		return err
	}
	match := codexFallbackResponsePath.FindStringSubmatch(command)
	if len(match) != 2 {
		return nil
	}
	return os.WriteFile(match[1], []byte(markedResponse("codex response-only completion")), 0o600)
}

func TestInteractivePaneBackend_CodexUntrustedStartupSendsPromptAndCollectsResponse(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	term := &codexStartupFallbackTerminal{seqScreenMock: &seqScreenMock{
		name: "cmux", screens: []string{codexStablePrompt},
	}}
	provider := ProviderConfig{
		Name: "codex", Binary: "codex", StartupTimeout: 3 * time.Second,
	}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal: term, HookMode: true,
		SessionID: "codex-startup-integration-" + NewSessionID(),
		Providers: []ProviderConfig{provider}, WorkingDir: t.TempDir(),
		InitialDelay: time.Millisecond,
	})

	response, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: provider.Name, Config: provider, Prompt: "prove prompt delivery",
		Role: "reviewer", Timeout: 8 * time.Second,
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "codex response-only completion", response.Output)
	assert.False(t, response.TimedOut)
	promptSent := false
	for _, command := range term.commands {
		if codexFallbackResponsePath.MatchString(command) {
			promptSent = true
		}
	}
	assert.True(t, promptSent, "the stable Codex prompt must receive the initial request")
}
