package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineRun_NonexistentSpec_FailsBeforeCompletion(t *testing.T) {
	root := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "test-pipeline-run"}
	cmd.SetContext(context.Background())
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cfg := &pipelineRunConfig{
		Platform: "codex",
		Strategy: "sequential",
	}

	err = runPipeline(cmd, "SPEC-DOES-NOT-EXIST", cfg)

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "SPEC not found: SPEC-DOES-NOT-EXIST")
	}
	assert.NotContains(t, stdout.String(), "Pipeline complete")
}

func TestPipelineRun_DryRunPassesResolvedSpecDirToFrozenPromptBuilder(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-PIPELINE-PROMPT-001"
	specDir := filepath.Join(root, ".autopus", "specs", specID)
	require.NoError(t, os.MkdirAll(specDir, 0o700))
	for name, body := range map[string]string{
		"spec.md":       "# SPEC-WRONG-001: mismatched identity\n",
		"plan.md":       "plan",
		"acceptance.md": "acceptance",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(specDir, name), []byte(body), 0o600))
	}

	var output bytes.Buffer
	cmd := &cobra.Command{Use: "test-pipeline-run"}
	cmd.SetContext(context.Background())
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err := runPipeline(cmd, specID, &pipelineRunConfig{
		Platform: "codex", Strategy: "sequential", DryRun: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity")
	assert.NotContains(t, output.String(), "Pipeline complete")
}
