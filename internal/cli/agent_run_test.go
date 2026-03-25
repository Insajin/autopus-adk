package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentRunCmd_NoArgs verifies the command returns a usage error when no task ID is provided.
func TestAgentRunCmd_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := newAgentRunSubCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err, "run with no args must return an error")
}

// TestAgentRunCmd_MissingTask verifies an error is returned when context.yaml does not exist.
func TestAgentRunCmd_MissingTask(t *testing.T) {
	// Note: cannot use t.Parallel() when changing working directory.

	// Point the working directory to a temp dir with no .autopus directory.
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newAgentRunSubCmd()
	cmd.SetArgs([]string{"T99"})
	err = cmd.Execute()
	require.Error(t, err, "run with missing context.yaml must return an error")
	assert.True(t,
		strings.Contains(err.Error(), "T99") || strings.Contains(err.Error(), "task context not found"),
		"error must mention the task ID or 'task context not found', got: %v", err,
	)
}

// TestAgentRunCmd_ValidTask verifies that a valid context.yaml leads to successful execution
// and a result.yaml is written.
func TestAgentRunCmd_ValidTask(t *testing.T) {
	// Note: cannot use t.Parallel() when changing working directory.

	dir := t.TempDir()
	taskID := "T01"

	// Create the expected context.yaml.
	runsDir := filepath.Join(dir, ".autopus", "runs", taskID)
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	contextYAML := []byte("task_id: T01\ndescription: test task\n")
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, "context.yaml"), contextYAML, 0o644))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newAgentRunSubCmd()
	cmd.SetArgs([]string{taskID})
	err = cmd.Execute()
	require.NoError(t, err, "run with valid context.yaml must succeed")

	// result.yaml must be written.
	resultPath := filepath.Join(runsDir, "result.yaml")
	_, statErr := os.Stat(resultPath)
	assert.NoError(t, statErr, "result.yaml must be created after successful run")
}

// TestAgentRunCmd_PathTraversal verifies that path traversal task IDs are rejected.
func TestAgentRunCmd_PathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		taskID string
	}{
		{"dot-dot-slash", "../../etc"},
		{"slash-prefix", "/tmp/evil"},
		{"contains-slash", "foo/bar"},
		{"dot-dot", ".."},
		{"empty-like", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := runAgentTask(tt.taskID)
			require.Error(t, err, "path traversal task ID must be rejected")
			assert.Contains(t, err.Error(), "invalid task ID")
		})
	}
}

// TestAgentRunCmd_InvalidYAML verifies that malformed context.yaml returns an error.
func TestAgentRunCmd_InvalidYAML(t *testing.T) {
	// Note: cannot use t.Parallel() when changing working directory.

	dir := t.TempDir()
	taskID := "T02"

	// Create a context.yaml with invalid YAML content.
	runsDir := filepath.Join(dir, ".autopus", "runs", taskID)
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	// Invalid YAML: tabs used as indentation are not valid in YAML.
	invalidYAML := []byte("task_id: T02\n\tinvalid: [unclosed\n")
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, "context.yaml"), invalidYAML, 0o644))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newAgentRunSubCmd()
	cmd.SetArgs([]string{taskID})
	err = cmd.Execute()
	require.Error(t, err, "run with invalid YAML must return an error")
	assert.True(t,
		strings.Contains(err.Error(), "parse context") || strings.Contains(err.Error(), "T02"),
		"error must mention parse failure or task ID, got: %v", err,
	)
}
