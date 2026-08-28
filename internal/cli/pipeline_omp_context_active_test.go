package cli

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestPipelineOMPBackend_ManagedActivePreflightFailureFallsBackExactlyOnce(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	usePipelineOMPActiveTestScope(&config)
	runner := &pipelineOMPManagedActiveRunnerSpy{prepareErr: errors.New("grant unavailable")}
	config.ManagedActive = runner
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	response, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhaseImplement, "IMPLEMENT-PHASE-PROMPT", nil,
	))

	require.NoError(t, err)
	assert.Equal(t, "rpc-canonical-full", response.Backend)
	assert.Equal(t, 1, runner.prepareCalls)
	assert.Zero(t, runner.executeCalls)
	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Equal(t, 1, countPipelineOMPRPCCommand(commands, "prompt"))
}

func TestPipelineOMPBackend_ManagedActiveConsumesLeaseAndReturnsOutput(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	usePipelineOMPActiveTestScope(&config)
	runner := &pipelineOMPManagedActiveRunnerSpy{output: "managed active output"}
	config.ManagedActive = runner
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	response, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhaseImplement, "IMPLEMENT-PHASE-PROMPT", []string{"prior phase"},
	))

	require.NoError(t, err)
	assert.Equal(t, "rpc-managed-active-history", response.Backend)
	assert.Equal(t, "managed active output", response.Output)
	assert.Equal(t, 1, runner.prepareCalls)
	assert.Equal(t, 1, runner.executeCalls)
	assert.Equal(t, 1, runner.consumeCalls)
	_, statErr := os.Stat(logPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "active success must not start canonical RPC")
}

func TestPipelineOMPBackend_ManagedActivePostStartFailureNeverFallsBack(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	usePipelineOMPActiveTestScope(&config)
	runner := &pipelineOMPManagedActiveRunnerSpy{executeErr: errors.New("provider failed after start")}
	config.ManagedActive = runner
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	response, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhaseImplement, "IMPLEMENT-PHASE-PROMPT", nil,
	))

	require.ErrorContains(t, err, "provider failed after start")
	assert.Equal(t, "rpc-managed-active-history", response.Backend)
	assert.Equal(t, "execution_error", response.FailureClass)
	assert.Equal(t, 1, runner.executeCalls)
	_, statErr := os.Stat(logPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "post-start failure must not run canonical RPC")
}

func TestPipelineOMPBackend_CanonicalFallbackIsStickyForTheRun(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	usePipelineOMPActiveTestScope(&config)
	runner := &pipelineOMPManagedActiveRunnerSpy{prepareErr: errors.New("grant unavailable")}
	config.ManagedActive = runner
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	first, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhasePlan, "PLAN-PHASE-PROMPT", nil,
	))
	require.NoError(t, err)
	require.Equal(t, "rpc-canonical-full", first.Backend)

	runner.prepareErr = nil
	runner.output = "must not run"
	second, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhaseImplement, "IMPLEMENT-PHASE-PROMPT", []string{"prior"},
	))
	require.NoError(t, err)
	assert.Equal(t, "rpc-canonical-full", second.Backend)
	assert.Equal(t, 1, runner.prepareCalls)
	assert.Zero(t, runner.executeCalls)
	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Equal(t, 2, countPipelineOMPRPCCommand(commands, "prompt"))
}

func TestPipelineOMPBackend_ActiveRunNeverFallsBackAfterPreflightDrift(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	usePipelineOMPActiveTestScope(&config)
	runner := &pipelineOMPManagedActiveRunnerSpy{output: "active plan"}
	config.ManagedActive = runner
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)

	first, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhasePlan, "PLAN-PHASE-PROMPT", nil,
	))
	require.NoError(t, err)
	require.Equal(t, "rpc-managed-active-history", first.Backend)

	runner.prepareErr = errors.New("grant drift")
	second, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(
		t, config, pipeline.PhaseImplement, "IMPLEMENT-PHASE-PROMPT", []string{"prior"},
	))
	require.ErrorContains(t, err, "preflight drifted")
	assert.Equal(t, "execution_error", second.FailureClass)
	assert.Equal(t, 2, runner.prepareCalls)
	assert.Equal(t, 1, runner.executeCalls)
	assert.Equal(t, 1, runner.closeCalls)
	_, statErr := os.Stat(logPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "active drift must not start canonical RPC")
}

type pipelineOMPManagedActiveRunnerSpy struct {
	prepareErr, executeErr     error
	output                     string
	prepareCalls, executeCalls int
	consumeCalls, closeCalls   int
}

func (runner *pipelineOMPManagedActiveRunnerSpy) Prepare(
	_ context.Context, candidate pipelineOMPManagedActiveCandidate,
) (pipelineOMPManagedActivePrepared, error) {
	runner.prepareCalls++
	if runner.prepareErr != nil {
		return pipelineOMPManagedActivePrepared{}, runner.prepareErr
	}
	binding := pipelineOMPActiveLeaseBindingFixture()
	binding.SpecID = candidate.Snapshot.SpecID
	identity := pipelineOMPContextIdentity(candidate.Snapshot)
	binding.TaskID, binding.SessionID = identity[0], identity[1]
	binding.Phase = string(candidate.Snapshot.PhaseID)
	binding.SnapshotHash = candidate.Snapshot.SnapshotHash
	binding.GitCommitHash = candidate.Snapshot.GitCommitHash
	binding.OriginalTaskHash = workflowContextRuntimeHash(candidate.OriginalTask)
	binding.DecisionDeltaHash = workflowContextRuntimeHash(candidate.Snapshot.ActivePrompt)
	binding.ModelScopeDigest = candidate.ModelScopeDigest
	binding.Provider, binding.Model = candidate.Provider, candidate.Model
	binding.AutoSourceCommit, binding.AutoSourceTree = candidate.AutoSourceCommit, candidate.AutoSourceTree
	lease, err := newPipelineOMPActiveLease(binding, time.Now(), time.Minute)
	return pipelineOMPManagedActivePrepared{Lease: lease, Binding: binding}, err
}

func (runner *pipelineOMPManagedActiveRunnerSpy) Execute(
	_ context.Context,
	_ pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
) (string, error) {
	runner.executeCalls++
	if err := prepared.Lease.Consume(prepared.Binding, time.Now()); err != nil {
		return "", err
	}
	runner.consumeCalls++
	if runner.executeErr != nil {
		return "", runner.executeErr
	}
	return runner.output, nil
}

func (runner *pipelineOMPManagedActiveRunnerSpy) Close() error {
	runner.closeCalls++
	return nil
}

func usePipelineOMPActiveTestScope(config *pipelineOMPBackendConfig) {
	config.PhaseModels = map[pipeline.PhaseID]string{
		pipeline.PhasePlan: "provider-b/model-plan", pipeline.PhaseTestScaffold: "provider-b/model-test",
		pipeline.PhaseImplement: "provider-b/model-implement", pipeline.PhaseValidate: "provider-b/model-validate",
		pipeline.PhaseReview: "provider-b/model-review",
	}
}
