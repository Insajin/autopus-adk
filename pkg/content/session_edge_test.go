package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestSaveState_NilState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	err := content.SaveState(path, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestSaveState_InvalidPath(t *testing.T) {
	t.Parallel()

	state := &content.SessionState{
		WorkflowPhase: "test",
	}

	err := content.SaveState("/nonexistent/dir/file.md", state)
	assert.Error(t, err)
}

func TestLoadState_PlainYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	// Write plain YAML without ```yaml blocks
	plainYAML := "workflow_phase: planning\ncompleted_tasks:\n  - task-1\n"
	require.NoError(t, os.WriteFile(path, []byte(plainYAML), 0644))

	state, err := content.LoadState(path)
	require.NoError(t, err)
	assert.Equal(t, "planning", state.WorkflowPhase)
	assert.Equal(t, []string{"task-1"}, state.CompletedTasks)
}

func TestLoadState_IncompleteYAMLBlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.md")

	// YAML block with opening ``` but no closing ``` -- extractYAMLBlock returns ""
	// so the whole content is parsed as YAML, which fails due to non-YAML markdown.
	// This tests the extractYAMLBlock fallback path.
	badMD := "```yaml\nworkflow_phase: test\n"
	require.NoError(t, os.WriteFile(path, []byte(badMD), 0644))

	// extractYAMLBlock returns "" (no closing ```), so whole content is parsed.
	// The whole content starts with ```yaml which is invalid YAML, so it errors.
	_, err := content.LoadState(path)
	assert.Error(t, err)
}

func TestSaveAndLoadState_EmptyContextSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	state := &content.SessionState{
		WorkflowPhase: "review",
	}

	require.NoError(t, content.SaveState(path, state))

	loaded, err := content.LoadState(path)
	require.NoError(t, err)
	assert.Equal(t, "review", loaded.WorkflowPhase)
	assert.Empty(t, loaded.ContextSummary)
}

func TestSaveAndLoadState_ExactBoundaryContextSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	// Exactly 8000 chars (maxContextChars) -- should not be truncated
	exactSummary := make([]byte, 8000)
	for i := range exactSummary {
		exactSummary[i] = 'x'
	}

	state := &content.SessionState{
		WorkflowPhase:  "boundary",
		ContextSummary: string(exactSummary),
	}

	require.NoError(t, content.SaveState(path, state))

	loaded, err := content.LoadState(path)
	require.NoError(t, err)
	assert.Len(t, loaded.ContextSummary, 8000)
}
