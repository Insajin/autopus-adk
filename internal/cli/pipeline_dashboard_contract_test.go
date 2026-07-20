package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestPipelineDashboard_MalformedCheckpoint_ReturnsError(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-DASHBOARD-001"
	path := specCheckpointPath(specID)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("phase: [broken yaml"), 0o600))
	cmd := newPipelineDashboardCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{specID})

	err := cmd.Execute()

	require.Error(t, err)
	assert.NotContains(t, output.String(), "showing default state")
}

func TestPipelineDashboard_MissingCheckpoint_UsesPendingFallback(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	cmd := newPipelineDashboardCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"SPEC-DASHBOARD-002"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, output.String(), "showing default state")
	assert.Contains(t, output.String(), "pending")
}

func TestPersistPipelineBlockedReceipt_ExistingCanonicalCheckpointIsPreserved(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-BLOCKED-CLI-001"
	require.NoError(t, os.MkdirAll(pipelineStateDir, 0o700))
	cp := pipeline.Checkpoint{
		Version: pipeline.CheckpointVersion, RouteVersion: pipeline.PipelineRouteVersion,
		SpecID: specID, SnapshotHash: "sha256:old", GitCommitHash: "git-a",
		TaskStatus: map[string]pipeline.CheckpointStatus{string(pipeline.PhasePlan): pipeline.CheckpointStatusDone},
	}
	require.NoError(t, cp.SaveFile(specCheckpointPath(specID)))
	before, err := os.ReadFile(specCheckpointPath(specID))
	require.NoError(t, err)

	err = persistPipelineBlockedReceipt(
		specID, "sha256:new", "git-a", pipeline.StrategySequential, errors.New("backend unavailable"),
	)

	require.NoError(t, err)
	after, readErr := os.ReadFile(specCheckpointPath(specID))
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
	blocked, loadErr := pipeline.LoadFile(filepath.Join(pipelineStateDir, specID+".blocked.yaml"))
	require.NoError(t, loadErr)
	require.NotNil(t, blocked.Receipt)
	assert.Equal(t, pipeline.TerminalBlocked, blocked.Receipt.Terminal)
}
