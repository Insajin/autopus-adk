package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func pipelineOMPActiveRPCSessionFixture(
	t *testing.T,
	unsafe bool,
) (*pipelineOMPActiveEvaluatorSession, pipelineOMPBackendConfig, string) {
	return pipelineOMPActiveRPCSessionFixtureWithSandbox(t, unsafe, pipelineOMPActiveSandboxManaged)
}

func pipelineOMPActiveRPCSessionFixtureWithSandbox(
	t *testing.T,
	unsafe bool,
	sandboxMode pipelineOMPActiveSandboxMode,
) (*pipelineOMPActiveEvaluatorSession, pipelineOMPBackendConfig, string) {
	t.Helper()
	model := "model-a"
	if unsafe {
		model = "model-unsafe"
	}
	session, config, logPath, err := pipelineOMPActiveRPCSessionFixtureWithModel(t, model, sandboxMode)
	require.NoError(t, err)
	return session, config, logPath
}

func pipelineOMPActiveRPCSessionFixtureWithModel(
	t *testing.T,
	model string,
	sandboxMode pipelineOMPActiveSandboxMode,
) (*pipelineOMPActiveEvaluatorSession, pipelineOMPBackendConfig, string, error) {
	t.Helper()
	requireDarwinManagedOMPSandboxForTest(t)
	config, _ := pipelineOMPBackendTestConfig(t)
	config.Executable = os.Args[0]
	logPath := filepath.Join(config.ProjectDir, "active-native-rpc.jsonl")
	config.PhaseModels = map[pipeline.PhaseID]string{pipeline.PhasePlan: "provider-a/" + model}
	config.Environment = append(config.Environment,
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43123",
		pipelineOMPActiveCredentialKey+"=fixture-token-value",
	)
	config, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: config.ProjectDir, SpecID: config.SpecID, SpecDir: config.SpecDir,
		SnapshotHash: config.SnapshotHash, GitCommitHash: config.GitCommitHash,
		PhaseID: pipeline.PhasePlan, Attempt: 1, Prompt: "canonical", ActivePrompt: "active",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(
		snapshot, config.PhaseModels[pipeline.PhasePlan], config.PhaseModels,
	)
	require.NoError(t, err)
	prepared := pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
		GrantDigest:  workflowContextRuntimeHash("grant"),
		PolicyDigest: workflowContextRuntimeHash("active-rpc-policy"), WorkspaceID: "autopus-adk",
		SpecID: config.SpecID, GitCommitHash: config.GitCommitHash,
		ModelScopeDigest: candidate.ModelScopeDigest,
	}}
	session, err := startPipelineOMPActiveEvaluatorSession(
		context.Background(), config, candidate, prepared, true, sandboxMode,
	)
	return session, config, logPath, err
}
