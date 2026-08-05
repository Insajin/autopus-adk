package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestPipelineOMPMaxTimeSeconds_UsesPositiveWholeSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		maxTime time.Duration
		want    string
	}{
		{name: "ten minutes", maxTime: 10 * time.Minute, want: "600"},
		{name: "subsecond", maxTime: time.Nanosecond, want: "1"},
		{name: "fractional second", maxTime: 1500 * time.Millisecond, want: "2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, pipelineOMPMaxTimeSeconds(test.maxTime))
		})
	}
}

func TestWritePipelineOMPActiveModels_BindsConfiguredContextWindow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	active := pipelineOMPActiveProcessConfig{
		backend: pipelineOMPBackendConfig{
			PhaseModels:        map[pipeline.PhaseID]string{pipeline.PhaseImplement: "openai/model-a"},
			ModelContextWindow: 1_000_000,
		},
		endpoint: "http://127.0.0.1:43123",
	}

	require.NoError(t, writePipelineOMPActiveModels(root, active))
	body, err := os.ReadFile(filepath.Join(root, "models.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "contextWindow: 1000000")
	assert.NotContains(t, string(body), "contextWindow: 262144")
}

func TestConfigurePipelineOMPActiveSandbox_InheritedParentSkipsInnerWrapperOnDarwin(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("/usr/bin/true")
	originalPath := cmd.Path
	err := configurePipelineOMPActiveSandbox(
		cmd, "http://127.0.0.1:43123", pipelineOMPActiveSandboxInheritedParent,
	)
	if runtime.GOOS != "darwin" {
		require.ErrorContains(t, err, "Darwin")
		return
	}
	require.NoError(t, err)
	assert.Equal(t, originalPath, cmd.Path)
	assert.Equal(t, []string{"/usr/bin/true"}, cmd.Args)
}

func TestPipelineOMPActiveManagedPrompt_AcceptsNullResponseLifecycleAndSafeWidget(t *testing.T) {
	t.Parallel()
	terminal := true
	frames := []pipelineOMPRPCFrame{
		{ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`)},
		{ID: "pipeline-active-prompt-1", Type: "prompt_result", AgentInvoked: boolPointer(true)},
		{Type: "agent_start"},
		{Type: "turn_start"},
		{Type: "turn_end"},
		{ID: "widget-1", Type: "extension_ui_request", Method: "setWidget"},
		{Type: "agent_end", IsTerminal: &terminal},
	}
	protocol, sent := pipelineOMPProtocolFixture(frames)

	err := protocol.callManagedPrompt(context.Background(), "safe prompt")

	require.NoError(t, err)
	assert.NotContains(t, sent.String(), "extension_ui_response")
}

func TestPipelineOMPActiveManagedPrompt_RejectsInteractiveOrUncorrelatedActivity(t *testing.T) {
	t.Parallel()
	success := pipelineOMPRPCFrame{
		ID: "pipeline-active-prompt-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`null`),
	}
	tests := []struct {
		name, want string
		frames     []pipelineOMPRPCFrame
	}{
		{name: "interactive confirm", want: "maintenance crossed", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{ID: "confirm-1", Type: "extension_ui_request", Method: "confirm"},
		}},
		{name: "fire and forget notify remains closed", want: "maintenance crossed", frames: []pipelineOMPRPCFrame{
			success, {Type: "agent_start"}, {Type: "turn_start"}, {Type: "turn_end"},
			{ID: "notify-1", Type: "extension_ui_request", Method: "notify"},
		}},
		{name: "uncorrelated result", want: "did not invoke", frames: []pipelineOMPRPCFrame{
			success, {ID: "other", Type: "prompt_result", AgentInvoked: boolPointer(true)},
		}},
		{name: "local only result", want: "did not invoke", frames: []pipelineOMPRPCFrame{
			success, {ID: "pipeline-active-prompt-1", Type: "prompt_result", AgentInvoked: boolPointer(false)},
		}},
		{name: "lifecycle before response", want: "out of order", frames: []pipelineOMPRPCFrame{
			{Type: "agent_start"}, success,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			protocol, _ := pipelineOMPProtocolFixture(test.frames)
			err := protocol.callManagedPrompt(context.Background(), "safe prompt")
			require.ErrorContains(t, err, test.want)
		})
	}
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
