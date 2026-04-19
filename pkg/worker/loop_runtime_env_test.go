package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSymphonyWorkspace_CreatesPromptMarkdown(t *testing.T) {
	workDir := t.TempDir()
	prompt := "Please read .symphony/prompt.md before proceeding."

	err := prepareSymphonyWorkspace(workDir, prompt)
	require.NoError(t, err)

	promptPath := filepath.Join(workDir, ".symphony", "prompt.md")
	data, err := os.ReadFile(promptPath)
	require.NoError(t, err)
	assert.Equal(t, prompt, string(data))

	info, err := os.Stat(promptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), info.Mode().Perm())
}

func TestPrepareTaskRuntimeEnv_ConfiguresWritableCaches(t *testing.T) {
	taskCfg := adapter.TaskConfig{WorkDir: t.TempDir()}

	err := prepareTaskRuntimeEnv(&taskCfg)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"))
	assert.DirExists(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "gocache"))
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["TMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["TEST_TMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "tmp"), taskCfg.EnvVars["GOTMPDIR"])
	assert.Equal(t, filepath.Join(taskCfg.WorkDir, ".symphony", "artifacts", "gocache"), taskCfg.EnvVars["GOCACHE"])
}
