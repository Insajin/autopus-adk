package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteRound_RecoveredDirectPromptOnlySkipsCurrentRoundFileIPC(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("recovered-direct-round-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"claude": true})
	writeRoundIPCFixture(t, session, "claude", 2, "direct round response")

	term := newCmuxMock()
	term.readScreenOutput = "❯\n"
	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	panes := []paneInfo{{
		provider: provider, paneID: "pane-recovered", directPromptRound: 2,
	}}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
		Prompt: "recovered prompt", TimeoutSeconds: 5, Terminal: term,
		Interactive: true, HookMode: true, InitialDelay: time.Millisecond,
		WorkingDir: t.TempDir(),
	}

	round2 := executeRound(context.Background(), cfg, panes, session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round 1"}})

	require.Len(t, round2, 1)
	assert.Equal(t, "direct round response", round2[0].Output)
	assert.NoFileExists(t, filepath.Join(session.Dir(), RoundSignalName("claude", 2, "input.json")))
	require.NotEmpty(t, term.sendLongTextCalls, "recovered round must submit directly to its stable prompt")
	directSendCount := len(term.sendLongTextCalls)
	cursor, err := os.ReadFile(filepath.Join(session.Dir(), hookRoundCursorName("claude")))
	require.NoError(t, err)
	assert.Equal(t, "2", string(cursor))

	writeRoundIPCFixture(t, session, "claude", 3, "next round IPC response")
	round3 := executeRound(context.Background(), cfg, panes, session, 3, round2)

	require.Len(t, round3, 1)
	assert.Equal(t, "next round IPC response", round3[0].Output)
	assert.FileExists(t, filepath.Join(session.Dir(), RoundSignalName("claude", 3, "input.json")),
		"the round-scoped direct marker must not disable next-round file IPC")
	assert.Len(t, term.sendLongTextCalls, directSendCount, "next round must return to file IPC")
	for _, call := range term.sendCommandCalls {
		assert.NotContains(t, call.Cmd, "AUTOPUS_ROUND", "active panes must never receive a shell export")
	}
}

func writeRoundIPCFixture(t *testing.T, session *HookSession, provider string, round int, output string) {
	t.Helper()
	require.NoError(t, session.writeArtifact(RoundSignalName(provider, round, "ready"), nil, 0o600))
	require.NoError(t, session.writeArtifact(RoundSignalName(provider, round, "done"), nil, 0o600))
	require.NoError(t, session.writeJSONArtifact(RoundSignalName(provider, round, "result.json"), HookResult{
		Output: output,
	}))
}
