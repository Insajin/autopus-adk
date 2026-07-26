package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeArtifact_RedactsSecrets(t *testing.T) {
	t.Parallel()

	artifact := sanitizeArtifact("Authorization: Bearer secret-token\nOPENAI_API_KEY=sk-supersecret")

	assert.Equal(t, len("Authorization: Bearer secret-token\nOPENAI_API_KEY=sk-supersecret"), artifact.ByteLength)
	assert.NotEmpty(t, artifact.Hash)
	assert.NotContains(t, artifact.Preview, "secret-token")
	assert.NotContains(t, artifact.Preview, "sk-supersecret")
}

func TestProviderCapability_RoundOneReportsActualTransportAndProviderCollection(t *testing.T) {
	t.Parallel()

	hasHook := true
	noHook := false
	tests := []struct {
		name           string
		provider       ProviderConfig
		wantTransport  string
		wantCollection string
	}{
		{
			name:           "claude direct pane input",
			provider:       ProviderConfig{Name: "claude", InteractiveInput: "stdin", HasHook: &hasHook},
			wantTransport:  "send_long_text",
			wantCollection: "hook",
		},
		{
			name:           "codex sendkeys input",
			provider:       ProviderConfig{Name: "codex", InteractiveInput: "", HasHook: &hasHook},
			wantTransport:  "sendkeys",
			wantCollection: "hook",
		},
		{
			name:           "gemini launch argument input",
			provider:       ProviderConfig{Name: "gemini", Binary: "agy", InteractiveInput: "stdin", HasHook: &hasHook},
			wantTransport:  "cli_args",
			wantCollection: "hook",
		},
		{
			name:           "hookless provider polling",
			provider:       ProviderConfig{Name: "opencode", InteractiveInput: "args", HasHook: &noHook},
			wantTransport:  "cli_args",
			wantCollection: "poll",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := OrchestraConfig{HookMode: true, Terminal: newCmuxMock()}
			capability := providerCapability(cfg, tt.provider)

			assert.Equal(t, tt.wantTransport, capability.PromptTransportMode)
			assert.Equal(t, []string{tt.wantCollection}, capability.CollectionModes)
			assert.True(t, capability.SupportsPromptReceipt)
		})
	}
}

func TestPreflightReceipt_ConfiguredHookRemainsRuntimeUnverified(t *testing.T) {
	t.Parallel()

	hasHook := true
	cfg := OrchestraConfig{HookMode: true, Terminal: newCmuxMock()}
	receipt := preflightReceipt("run-configured-hook", cfg, ProviderConfig{
		Name:             "claude",
		InteractiveInput: "stdin",
		HasHook:          &hasHook,
	})

	assert.Equal(t, "pass", receipt.Status, "launch preflight may pass without claiming completion")
	assert.Equal(t, "completion hook is configured but not runtime-verified", receipt.Reason)
}

func TestPreflightReceipt_UsesRequestedWorkingDir(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{
		HookMode:   true,
		WorkingDir: "/tmp/autopus-spec",
		Terminal:   newCmuxMock(),
	}

	receipt := preflightReceipt("run-1", cfg, ProviderConfig{Name: "claude"})

	assert.Equal(t, "pass", receipt.Status)
	assert.Equal(t, "/tmp/autopus-spec", receipt.RequestedCWD)
	assert.Equal(t, "/tmp/autopus-spec", receipt.EffectiveCWD)
	assert.Equal(t, "run-1", receipt.Correlation.RunID)
}

func TestPruneReliabilityArtifacts_RemovesOldRuns(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	oldDir := filepath.Join(baseDir, "old")
	newDir := filepath.Join(baseDir, "new")
	require.NoError(t, os.MkdirAll(oldDir, 0o700))
	require.NoError(t, os.MkdirAll(newDir, 0o700))

	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldDir, oldTime, oldTime))

	require.NoError(t, pruneReliabilityArtifacts(baseDir, 20, 7*24*time.Hour, time.Hour))

	_, oldErr := os.Stat(oldDir)
	_, newErr := os.Stat(newDir)
	assert.Error(t, oldErr)
	assert.NoError(t, newErr)
}

func TestPruneReliabilityArtifacts_SkipsActiveRuns(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	activeDir := filepath.Join(baseDir, "active")
	require.NoError(t, os.MkdirAll(activeDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(activeDir, reliabilityActiveMarkerName), []byte("active"), 0o600))

	for i := 0; i < 25; i++ {
		runDir := filepath.Join(baseDir, fmt.Sprintf("run-%02d", i))
		require.NoError(t, os.MkdirAll(runDir, 0o700))
	}

	require.NoError(t, pruneReliabilityArtifacts(baseDir, 5, 30*24*time.Hour, time.Hour))

	_, err := os.Stat(activeDir)
	assert.NoError(t, err)
}

func TestReliabilityStore_WritesFailureBundle(t *testing.T) {
	t.Parallel()

	store, err := newReliabilityStore("run-test")
	require.NoError(t, err)

	store.recordEvent(timeoutEvent("run-test", "claude", 1, "retry with subprocess fallback"))
	path := store.writeFailureBundle("degraded run", "retry with subprocess fallback", true)

	require.NotEmpty(t, path)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)

	summary := store.summary(path)
	assert.Equal(t, "run-test", summary.RunID)
	assert.Equal(t, path, summary.FailureBundle)
	assert.Equal(t, 1, summary.OpenEvents)
}

func TestReliabilityStore_RecreatesRunDirOnWrite(t *testing.T) {
	t.Parallel()

	store, err := newReliabilityStore("run-recreate")
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(store.artifactDir()))

	path := store.recordCollection(collectionReceipt("run-recreate", "claude", "hook", "hook", "timeout", "boom", "", 1, true))

	require.NotEmpty(t, path)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}
