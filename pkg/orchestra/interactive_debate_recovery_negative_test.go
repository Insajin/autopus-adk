package orchestra

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteRound_FileIPCDeliveredOmitsRecoveryEvidence(t *testing.T) {
	session, err := NewHookSession("normal-file-ipc-" + NewSessionID())
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	session.SetHookProviders(map[string]bool{"claude": true})
	writeRoundIPCFixture(t, session, "claude", 2, "normal IPC response")

	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	term := newCmuxMock()
	term.readScreenOutput = "❯\n"
	store := &reliabilityStore{
		runID: "normal-file-ipc-" + NewSessionID(),
		dir:   t.TempDir(),
	}
	cfg := OrchestraConfig{
		Providers:        []ProviderConfig{provider},
		Strategy:         StrategyDebate,
		Prompt:           "normal Round 2 IPC",
		TimeoutSeconds:   5,
		Terminal:         term,
		Interactive:      true,
		HookMode:         true,
		InitialDelay:     time.Millisecond,
		WorkingDir:       t.TempDir(),
		RunID:            store.runID,
		ReliabilityStore: store,
	}

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "pane-1"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, "normal IPC response", responses[0].Output)
	recoveryFiles, err := filepath.Glob(filepath.Join(store.artifactDir(), "recovery-*.json"))
	require.NoError(t, err)
	assert.Empty(t, recoveryFiles)
	assert.Empty(t, store.recovery)

	bundlePath := store.writeFailureBundle("normal IPC", "none", false)
	data, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	var bundle FailureBundle
	require.NoError(t, json.Unmarshal(data, &bundle))
	assert.Empty(t, bundle.RecoveryTransitions)
	assert.NotContains(t, string(data), `"recovery_transitions"`)
	assert.Zero(t, store.summary(bundlePath).OpenEvents)
}
