package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	recoveryCanaryRootEnv        = "AUTOPUS_RECOVERY_CANARY_ROOT"
	recoveryCanaryPromptSentinel = "RUNTIME-RECOVERY-CANARY-PROMPT-DO-NOT-PERSIST"
)

func TestRecoveryRuntimeCanary_DeterministicSuccessPersistsPrivateEvidence(t *testing.T) {
	root, configured := os.LookupEnv(recoveryCanaryRootEnv)
	if !configured {
		t.Skipf("set %s to opt in to the retained runtime canary", recoveryCanaryRootEnv)
	}
	requireRuntimeCanaryRoot(t, root)

	runID := "runtime-recovery-canary-" + NewSessionID()
	require.Equal(t, runID, sanitizeProviderName(runID))
	dir := filepath.Join(root, "runs", sanitizeProviderName(runID))
	requireRuntimeCanaryDir(t, dir)
	t.Logf("RUNTIME_CANARY_DIR=%s", dir)

	term := newRound2ReleaseRecoveryTerminal()
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	store := &reliabilityStore{runID: runID, dir: dir}
	cfg.RunID = runID
	cfg.Prompt = recoveryCanaryPromptSentinel
	cfg.ReliabilityStore = store
	armSuccessfulRound2Recovery(t, term.recoveryLaunchTerminal, session, provider)

	responses := executeRound(
		context.Background(),
		cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session,
		2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)
	require.Len(t, responses, 1)
	require.Equal(t, "replacement response", responses[0].Output)
	require.Empty(t, responses[0].Error)

	bundlePath := store.writeFailureBundle(
		"runtime recovery canary complete",
		"inspect retained runtime canary artifacts",
		false,
	)
	require.Equal(t, filepath.Join(dir, "failure-bundle.json"), bundlePath)

	recoveryPaths, err := filepath.Glob(filepath.Join(dir, "recovery-*.json"))
	require.NoError(t, err)
	sort.Strings(recoveryPaths)
	require.Len(t, recoveryPaths, 3)
	persistedTransitions := make([]PaneRecoveryTransition, 0, len(recoveryPaths))
	for i, path := range recoveryPaths {
		requireRuntimeCanaryFile(t, path)
		assert.True(t, strings.HasPrefix(filepath.Base(path), fmt.Sprintf("recovery-%06d-", i+1)))
		var transition PaneRecoveryTransition
		readRuntimeCanaryJSON(t, path, &transition)
		persistedTransitions = append(persistedTransitions, transition)
	}

	var bundle FailureBundle
	readRuntimeCanaryJSON(t, bundlePath, &bundle)
	require.Equal(t, runID, bundle.RunID)
	require.False(t, bundle.Degraded)
	require.Len(t, bundle.RecoveryTransitions, 3)
	require.Equal(t, persistedTransitions, bundle.RecoveryTransitions)
	for i, expected := range []struct {
		stage  string
		status string
	}{
		{stage: "owner_closed", status: "pass"},
		{stage: "replacement_ready", status: "pass"},
		{stage: "prompt_submitted", status: "pass"},
	} {
		transition := bundle.RecoveryTransitions[i]
		assert.Equal(t, uint64(i+1), transition.Sequence)
		assert.Equal(t, expected.stage, transition.Stage)
		assert.Equal(t, expected.status, transition.Status)
		assert.Empty(t, transition.FailureCode)
		assert.Equal(t, runID, transition.Correlation.RunID)
	}

	require.Len(t, bundle.PromptReceipts, 2)
	assert.Equal(t, []string{"failed", "pass"}, []string{
		bundle.PromptReceipts[0].Status,
		bundle.PromptReceipts[1].Status,
	})
	assert.Equal(t, promptFailureFileIPCAbort, bundle.PromptReceipts[0].FailureCode)
	assert.Empty(t, bundle.PromptReceipts[1].FailureCode)
	for _, receipt := range bundle.PromptReceipts {
		assert.Empty(t, receipt.Mismatch)
		assert.Empty(t, receipt.Prompt.Preview)
		assert.Positive(t, receipt.Prompt.ByteLength)
		assert.NotEmpty(t, receipt.Prompt.Hash)
	}

	requireRuntimeCanaryPrivacy(t, dir, session.SessionID())
	requireRuntimeCanaryModes(t, dir)
}

func requireRuntimeCanaryRoot(t *testing.T, root string) {
	t.Helper()
	require.NotEmpty(t, root, "%s must not be empty", recoveryCanaryRootEnv)
	require.True(t, filepath.IsAbs(root), "%s must be absolute", recoveryCanaryRootEnv)
	require.Equal(t, filepath.Clean(root), root, "%s must be a clean path", recoveryCanaryRootEnv)
	require.Equal(t, "orchestra", filepath.Base(root))
	require.Equal(t, "runtime", filepath.Base(filepath.Dir(root)))
	require.Equal(t, ".autopus", filepath.Base(filepath.Dir(filepath.Dir(root))))
}

func requireRuntimeCanaryDir(t *testing.T, dir string) {
	t.Helper()
	_, err := os.Lstat(dir)
	if err == nil {
		t.Fatalf("runtime canary dir already exists: %s", dir)
	}
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.MkdirAll(filepath.Dir(dir), 0o700))
	require.NoError(t, os.Mkdir(dir, 0o700))
}

func requireRuntimeCanaryPrivacy(t *testing.T, dir, sessionID string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	var raw strings.Builder
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		raw.Write(data)
	}
	for _, forbidden := range []string{
		recoveryCanaryPromptSentinel,
		sessionID,
		"old-pane",
		"pane-1",
		"write tmp file",
		"is a directory",
		`"mismatch":`,
		`"error":`,
	} {
		assert.NotContains(t, raw.String(), forbidden)
	}
}

func requireRuntimeCanaryModes(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	jsonPaths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, jsonPaths)
	for _, path := range jsonPaths {
		requireRuntimeCanaryFile(t, path)
	}
}

func requireRuntimeCanaryFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "%s must be a regular file", path)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), path)
}

func readRuntimeCanaryJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}
