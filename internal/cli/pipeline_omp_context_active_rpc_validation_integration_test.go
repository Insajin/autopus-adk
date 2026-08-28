package cli

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestPipelineOMPActiveProcessConfig_BindsProviderEndpointWithoutCredentialMaterial(t *testing.T) {
	config, _ := pipelineOMPBackendTestConfig(t)
	config.PhaseModels = map[pipeline.PhaseID]string{pipeline.PhasePlan: "provider-a/model-a"}
	config.Environment = append(pipelineOMPCanonicalEnvironment(config.Environment),
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43123",
		pipelineOMPActiveCredentialKey+"=private-provider-credential",
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
		GrantDigest: workflowContextRuntimeHash("grant"), PolicyDigest: workflowContextRuntimeHash("policy"),
		WorkspaceID: "autopus-adk", SpecID: config.SpecID, GitCommitHash: config.GitCommitHash,
		ModelScopeDigest: candidate.ModelScopeDigest,
	}}
	first, err := preparePipelineOMPActiveProcessConfig(config, candidate, prepared)
	require.NoError(t, err)
	config.Environment = append(pipelineOMPCanonicalEnvironment(config.Environment),
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43124",
		pipelineOMPActiveCredentialKey+"=private-provider-credential",
	)
	second, err := preparePipelineOMPActiveProcessConfig(config, candidate, prepared)
	require.NoError(t, err)

	assert.NotEqual(t, first.binding.BindingHash, second.binding.BindingHash)
	assert.NotEqual(t, first.binding.OptionsHash, second.binding.OptionsHash)
	serialized, err := json.Marshal([]WorkflowContextBridgeBinding{first.binding, second.binding})
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), "private-provider-credential")
	assert.NotContains(t, string(serialized), "127.0.0.1")
}

func TestPipelineOMPActiveRPC_RejectsTextOnlyPhaseModelBeforeProviderCall(t *testing.T) {
	session, _, logPath := pipelineOMPActiveRPCSessionFixture(t, false)
	session.selector = "provider-a/model-text-only"

	_, _, err := session.Execute(context.Background(), "safe plan phase")

	require.ErrorContains(t, err, "native image compaction capability is unavailable")
	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "compact"))
}

func TestPipelineOMPActiveRPC_RejectsTextOnlyModelBeforeProviderCall(t *testing.T) {
	session, _, logPath, err := pipelineOMPActiveRPCSessionFixtureWithModel(
		t, "model-text-only", pipelineOMPActiveSandboxManaged,
	)

	require.ErrorContains(t, err, "native image compaction capability is unavailable")
	assert.Nil(t, session)
	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "compact"))
}

func TestPipelineOMPActiveRPC_InheritedParentSandboxUsesDirectVerifiedImage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("inherited parent sandbox is Darwin-only")
	}
	session, _, _ := pipelineOMPActiveRPCSessionFixtureWithSandbox(
		t, false, pipelineOMPActiveSandboxInheritedParent,
	)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	output, receipt, err := session.Execute(context.Background(), "safe inherited prompt")

	require.NoError(t, err)
	assert.Equal(t, "safe assistant output 1", output)
	assert.True(t, receipt.SameProcess)
}
