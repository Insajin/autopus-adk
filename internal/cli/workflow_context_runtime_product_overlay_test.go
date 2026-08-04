package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductOverlay_ActiveReuseAndShadowRollbackStayTaskOwned(t *testing.T) {
	input, _, _, _ := newWorkflowContextProductFixture(t)
	userRoot := t.TempDir()
	t.Setenv("HOME", userRoot)
	writeWorkflowContextProductOverlayCanaries(t, userRoot)
	writeWorkflowContextProductOverlayCanaries(t, input.ProjectDir)
	userBefore := snapshotWorkflowContextProductOverlayTree(t, filepath.Join(userRoot, ".omp"))
	projectBefore := snapshotWorkflowContextProductOverlayTree(t, filepath.Join(input.ProjectDir, ".omp"))

	runtimeRoot := t.TempDir()
	controller, configPath, err := newWorkflowContextProductOverlay(runtimeRoot, config.OMPContextMemoryOff)
	require.NoError(t, err)
	require.NotNil(t, controller)
	assertWorkflowContextProductOverlayPath(t, runtimeRoot, configPath)
	activeBody, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NotEmpty(t, activeBody)
	activeHash := hashWorkflowContextProductOverlayBody(activeBody)
	activeInfo, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.True(t, activeInfo.Mode().IsRegular())
	assert.Equal(t, fs.FileMode(0o600), activeInfo.Mode().Perm())

	sentinel := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	require.NoError(t, os.Chtimes(configPath, sentinel, sentinel))
	activeInfo, err = os.Stat(configPath)
	require.NoError(t, err)
	active, err := ApplyWorkflowContextOverlay(context.Background(), controller, WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryActive,
		MemoryMode:  config.OMPContextMemoryOff,
		Reason:      "promotion-gates-passed",
	})
	require.NoError(t, err)
	afterActiveInfo, err := os.Stat(configPath)
	require.NoError(t, err)
	afterActiveBody, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.True(t, os.SameFile(activeInfo, afterActiveInfo), "active Apply must reuse the prepared inode")
	assert.True(t, afterActiveInfo.ModTime().Equal(sentinel), "active Apply must not rewrite prepared settings")
	assert.Equal(t, activeBody, afterActiveBody)
	assert.Equal(t, activeHash, active.OverlayHash)
	assert.Equal(t, active.OverlayHash, active.ReadbackHash)
	assert.Equal(t, config.OMPContextHistoryShadow, active.PreviousHistoryMode)
	assertWorkflowContextProductOverlayOnlyFile(t, runtimeRoot, configPath)

	shadow, err := ApplyWorkflowContextOverlay(context.Background(), controller, WorkflowContextOverlayRequest{
		HistoryMode: config.OMPContextHistoryShadow,
		MemoryMode:  config.OMPContextMemoryOff,
		Reason:      "quality-regression",
	})
	require.NoError(t, err)
	shadowBody, err := os.ReadFile(configPath)
	require.NoError(t, err)
	shadowHash := hashWorkflowContextProductOverlayBody(shadowBody)
	assert.NotEqual(t, activeBody, shadowBody, "rollback must replace the active task overlay")
	assert.NotEqual(t, activeHash, shadowHash)
	assert.Equal(t, shadowHash, shadow.OverlayHash)
	assert.Equal(t, shadow.OverlayHash, shadow.ReadbackHash)
	assert.Equal(t, config.OMPContextHistoryActive, shadow.PreviousHistoryMode)
	assert.Equal(t, "quality-regression", shadow.Reason)
	assertWorkflowContextProductOverlayOnlyFile(t, runtimeRoot, configPath)

	assert.Equal(t, userBefore, snapshotWorkflowContextProductOverlayTree(t, filepath.Join(userRoot, ".omp")))
	assert.Equal(t, projectBefore, snapshotWorkflowContextProductOverlayTree(t, filepath.Join(input.ProjectDir, ".omp")))
}

func TestWorkflowContextManagedManualCompactionOverlay_DisablesAutomaticCompactionOnly(t *testing.T) {
	t.Parallel()
	productRoot, managedRoot := t.TempDir(), t.TempDir()
	_, productPath, err := newWorkflowContextProductOverlay(productRoot, config.OMPContextMemoryOff)
	require.NoError(t, err)
	managedPath, err := newWorkflowContextManagedManualCompactionOverlay(managedRoot, config.OMPContextMemoryOff)
	require.NoError(t, err)

	productBody, err := os.ReadFile(productPath)
	require.NoError(t, err)
	managedBody, err := os.ReadFile(managedPath)
	require.NoError(t, err)
	assert.Contains(t, string(productBody), "compaction:\n  enabled: true\n")
	assert.Contains(t, string(managedBody), "compaction:\n  enabled: false\n")
	assert.NotEqual(t, productBody, managedBody)
	assertWorkflowContextProductOverlayPath(t, managedRoot, managedPath)
	assertWorkflowContextProductOverlayOnlyFile(t, managedRoot, managedPath)
}

func TestWorkflowContextProductOverlay_FailsClosedOnUnsafeInputsAndStateDrift(t *testing.T) {
	t.Run("invalid memory mode", func(t *testing.T) {
		_, _, err := newWorkflowContextProductOverlay(t.TempDir(), "active")
		require.ErrorContains(t, err, "memory mode is invalid")
	})

	t.Run("unsafe runtime roots", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		_, _, err := newWorkflowContextProductOverlay(missing, config.OMPContextMemoryOff)
		require.ErrorContains(t, err, "root is unsafe")

		fileRoot := filepath.Join(t.TempDir(), "runtime-file")
		require.NoError(t, os.WriteFile(fileRoot, []byte("unsafe"), 0o600))
		_, _, err = newWorkflowContextProductOverlay(fileRoot, config.OMPContextMemoryOff)
		require.ErrorContains(t, err, "root is unsafe")
	})

	t.Run("request and active identity drift", func(t *testing.T) {
		controller, path, err := newWorkflowContextProductOverlay(t.TempDir(), config.OMPContextMemoryShadow)
		require.NoError(t, err)
		overlay := controller.(*workflowContextProductOverlay)
		//nolint:staticcheck // The nil context intentionally verifies the fail-closed API contract.
		_, err = overlay.Apply(nil, WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryShadow,
		})
		require.ErrorContains(t, err, "context is required")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = overlay.Apply(ctx, WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryShadow,
		})
		require.ErrorIs(t, err, context.Canceled)

		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		})
		require.ErrorContains(t, err, "memory mode changed")

		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: "invalid", MemoryMode: config.OMPContextMemoryShadow,
		})
		require.ErrorContains(t, err, "history mode is invalid")

		require.NoError(t, os.WriteFile(path, []byte("drift"), 0o600))
		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryShadow,
		})
		require.ErrorContains(t, err, "readback mismatch")
	})

	t.Run("prepared inode replacement", func(t *testing.T) {
		controller, path, err := newWorkflowContextProductOverlay(t.TempDir(), config.OMPContextMemoryOff)
		require.NoError(t, err)
		overlay := controller.(*workflowContextProductOverlay)
		replacement := filepath.Join(filepath.Dir(path), "replacement.yml")
		require.NoError(t, os.WriteFile(replacement, overlay.activeBody, 0o600))
		require.NoError(t, os.Rename(replacement, path))
		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		})
		require.ErrorContains(t, err, "identity changed")
	})

	t.Run("rollback cannot reactivate", func(t *testing.T) {
		controller, _, err := newWorkflowContextProductOverlay(t.TempDir(), config.OMPContextMemoryOff)
		require.NoError(t, err)
		overlay := controller.(*workflowContextProductOverlay)
		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryOff, MemoryMode: config.OMPContextMemoryOff,
		})
		require.NoError(t, err)
		_, err = overlay.Apply(context.Background(), WorkflowContextOverlayRequest{
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		})
		require.ErrorContains(t, err, "cannot reactivate")
	})
}

func writeWorkflowContextProductOverlayCanaries(t *testing.T, root string) {
	t.Helper()
	canaries := map[string]string{
		"settings.json": `{"canary":"settings"}`,
		"config.json":   `{"canary":"config-json"}`,
		"config.yml":    "canary: config-yaml\n",
	}
	for name, body := range canaries {
		path := filepath.Join(root, ".omp", name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
}

func snapshotWorkflowContextProductOverlayTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(body)
		return nil
	}))
	return snapshot
}

func assertWorkflowContextProductOverlayPath(t *testing.T, root, path string) {
	t.Helper()
	relative, err := filepath.Rel(root, path)
	require.NoError(t, err)
	assert.NotEqual(t, ".", relative)
	assert.NotEqual(t, "..", relative)
	assert.NotContains(t, filepath.ToSlash(relative), "../")
	assert.Equal(t, "context-product.yml", filepath.Base(path))
}

func assertWorkflowContextProductOverlayOnlyFile(t *testing.T, root, want string) {
	t.Helper()
	var files []string
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, filepath.Clean(path))
		}
		return nil
	}))
	assert.Equal(t, []string{filepath.Clean(want)}, files)
}

func hashWorkflowContextProductOverlayBody(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
