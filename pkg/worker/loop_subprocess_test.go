package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- prepareSymphonyWorkspace ----------

func TestPrepareSymphonyWorkspace_SkipsWhenNoSymphonyRef(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	err := prepareSymphonyWorkspace(workDir, "just a regular prompt with no symphony reference")
	require.NoError(t, err)
	// No .symphony dir should be created.
	_, statErr := os.Stat(filepath.Join(workDir, ".symphony"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestPrepareSymphonyWorkspace_CreatesStructureAndWritesPrompt(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	prompt := "please read .symphony/prompt.md and execute the plan"
	err := prepareSymphonyWorkspace(workDir, prompt)
	require.NoError(t, err)

	promptPath := filepath.Join(workDir, ".symphony", "prompt.md")
	data, readErr := os.ReadFile(promptPath)
	require.NoError(t, readErr)
	assert.Equal(t, prompt, string(data))

	// Artifacts dir must also exist.
	artifactsDir := filepath.Join(workDir, ".symphony", "artifacts")
	info, statErr := os.Stat(artifactsDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestPrepareSymphonyWorkspace_IdempotentWhenPromptExists(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	prompt := "use .symphony/prompt.md"

	require.NoError(t, prepareSymphonyWorkspace(workDir, prompt))
	// Second call must not overwrite or error out.
	require.NoError(t, prepareSymphonyWorkspace(workDir, "updated prompt .symphony/prompt.md"))

	// Original content is preserved because the file already existed.
	data, _ := os.ReadFile(filepath.Join(workDir, ".symphony", "prompt.md"))
	assert.Equal(t, prompt, string(data))
}

// ---------- prepareTaskRuntimeEnv ----------

func TestPrepareTaskRuntimeEnv_CreatesRequiredDirsAndSetsEnvVars(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cfg := &adapter.TaskConfig{TaskID: "env-test", WorkDir: workDir}
	require.NoError(t, prepareTaskRuntimeEnv(cfg))

	// All three dirs must exist.
	artifactsDir := filepath.Join(workDir, ".symphony", "artifacts")
	tmpDir := filepath.Join(artifactsDir, "tmp")
	goCacheDir := filepath.Join(artifactsDir, "gocache")

	for _, dir := range []string{artifactsDir, tmpDir, goCacheDir} {
		info, err := os.Stat(dir)
		require.NoError(t, err, "expected dir %s to exist", dir)
		assert.True(t, info.IsDir())
	}

	assert.Equal(t, tmpDir, cfg.EnvVars["TMPDIR"])
	assert.Equal(t, tmpDir, cfg.EnvVars["TEST_TMPDIR"])
	assert.Equal(t, tmpDir, cfg.EnvVars["GOTMPDIR"])
	assert.Equal(t, goCacheDir, cfg.EnvVars["GOCACHE"])
}

func TestPrepareTaskRuntimeEnv_PreservesExistingEnvVars(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cfg := &adapter.TaskConfig{
		TaskID:  "env-preserve",
		WorkDir: workDir,
		EnvVars: map[string]string{"CUSTOM": "preserved"},
	}
	require.NoError(t, prepareTaskRuntimeEnv(cfg))

	assert.Equal(t, "preserved", cfg.EnvVars["CUSTOM"])
	// Runtime vars are still injected.
	assert.NotEmpty(t, cfg.EnvVars["TMPDIR"])
}

// ---------- providerResultError ----------

func TestProviderResultError_WithError(t *testing.T) {
	t.Parallel()

	result := adapter.TaskResult{Error: " auth failed "}
	err := providerResultError("claude", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude result error")
	assert.Contains(t, err.Error(), "auth failed")
}

func TestProviderResultError_FallsBackToOutput(t *testing.T) {
	t.Parallel()

	result := adapter.TaskResult{Output: " task aborted "}
	err := providerResultError("codex", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task aborted")
}

func TestProviderResultError_DefaultMessage(t *testing.T) {
	t.Parallel()

	result := adapter.TaskResult{}
	err := providerResultError("", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider result marked as error")
}

// ---------- parseStream (parseStreamWithBudget without budget) ----------

func TestParseStream_SingleResultLine(t *testing.T) {
	t.Parallel()

	wl := NewWorkerLoop(LoopConfig{
		Provider: &mockAdapter{
			name: "mock",
			parseFn: func(line []byte) (adapter.StreamEvent, error) {
				return adapter.StreamEvent{Type: "result", Data: line}, nil
			},
			extractFn: func(evt adapter.StreamEvent) adapter.TaskResult {
				return adapter.TaskResult{Output: "stream output"}
			},
		},
	})

	r := strings.NewReader(`{"type":"result","output":"stream output"}` + "\n")
	result, err := wl.parseStream(r, "task-1")
	require.NoError(t, err)
	assert.Equal(t, "stream output", result.Output)
}

func TestParseStream_SkipsMalformedLines(t *testing.T) {
	t.Parallel()

	wl := NewWorkerLoop(LoopConfig{
		Provider: &mockAdapter{
			name: "mock",
			parseFn: func(line []byte) (adapter.StreamEvent, error) {
				return adapter.StreamEvent{Type: "result", Data: line}, nil
			},
			extractFn: func(evt adapter.StreamEvent) adapter.TaskResult {
				return adapter.TaskResult{Output: "ok"}
			},
		},
	})

	r := strings.NewReader("not json at all\n" + `{"type":"result","output":"ok"}` + "\n")
	result, err := wl.parseStream(r, "task-skip")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Output)
}

func TestParseStream_NoResultErrors(t *testing.T) {
	t.Parallel()

	wl := NewWorkerLoop(LoopConfig{
		Provider: &mockAdapter{
			name: "mock",
			parseFn: func(line []byte) (adapter.StreamEvent, error) {
				return adapter.StreamEvent{Type: "system", Data: line}, nil
			},
		},
	})

	r := strings.NewReader(`{"type":"system"}` + "\n")
	_, err := wl.parseStream(r, "task-noResult")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result event")
}
