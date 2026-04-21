package orchestra

import (
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

func TestProviderCapability_HookModeUsesFileIPC(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{HookMode: true, Terminal: newCmuxMock()}
	cap := providerCapability(cfg, ProviderConfig{Name: "claude"})

	assert.Equal(t, "file_ipc", cap.PromptTransportMode)
	assert.True(t, cap.SupportsPromptReceipt)
	assert.Contains(t, cap.CollectionModes, "hook")
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

	require.NoError(t, pruneReliabilityArtifacts(baseDir, 20, 7*24*time.Hour))

	_, oldErr := os.Stat(oldDir)
	_, newErr := os.Stat(newDir)
	assert.Error(t, oldErr)
	assert.NoError(t, newErr)
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
