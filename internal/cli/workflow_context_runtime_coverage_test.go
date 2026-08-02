package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextRuntimePolicy_ConsumesEffectiveConfigAndCommand(t *testing.T) {
	t.Parallel()
	if policy, ok, err := workflowContextPolicyFromConfig(nil); err != nil || ok || policy.Profile != "" {
		t.Fatalf("nil config policy = %#v, %v, %v", policy, ok, err)
	}
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("runtime-policy")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		}},
	}
	require.NoError(t, config.Save(dir, cfg))
	policy, ok, err := workflowContextPolicyFromConfig(cfg)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "active", policy.Profile)
	assert.Equal(t, config.OMPContextHistoryActive, policy.HistoryMode)

	cmd := newWorkflowContextRuntimeCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--project-dir", dir, "--format", "json"})
	require.NoError(t, cmd.Execute(), output.String())
	var got workflowContextRuntimePolicyOutput
	require.NoError(t, json.Unmarshal(output.Bytes(), &got))
	assert.True(t, got.Enabled)
	assert.Equal(t, policy, got.Policy)

	cmd = newWorkflowContextRuntimeCmd()
	cmd.SetArgs([]string{"--project-dir", dir, "--format", "yaml"})
	require.ErrorContains(t, cmd.Execute(), "unsupported format")
}

func TestWorkflowContextRuntimeRequestValidation_RejectsUnsafePolicyCombinations(t *testing.T) {
	t.Parallel()
	base := newWorkflowContextRuntimeFixture(t)
	tests := []struct {
		name   string
		mutate func(*WorkflowContextRuntimeRequest)
	}{
		{"history not active", func(r *WorkflowContextRuntimeRequest) { r.Policy.HistoryMode = config.OMPContextHistoryShadow }},
		{"memory active", func(r *WorkflowContextRuntimeRequest) { r.Policy.MemoryMode = "active" }},
		{"probe optional", func(r *WorkflowContextRuntimeRequest) { r.Policy.CapabilityPolicy = "optional" }},
		{"global mutation", func(r *WorkflowContextRuntimeRequest) { r.Policy.MutationScope = "global" }},
		{"root disagreement", func(r *WorkflowContextRuntimeRequest) { r.RootClass = config.OMPContextRuntimeNoSession }},
		{"user root", func(r *WorkflowContextRuntimeRequest) { r.Policy.RuntimeRootPolicy, r.RootClass = "user", "user" }},
		{"zero target", func(r *WorkflowContextRuntimeRequest) { r.Policy.HistoryTargetTokens = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := base
			tt.mutate(&request)
			require.Error(t, validateWorkflowContextRuntimeRequest(request))
		})
	}
	require.NoError(t, validateWorkflowContextRuntimeRequest(base))
}

func TestWorkflowContextRuntimeCapabilityAndFailureReasonBranches(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	request.Policy.RuntimeRootPolicy = config.OMPContextRuntimeNoSession
	request.RootClass = config.OMPContextRuntimeNoSession
	request.Capabilities.NoSession = false
	assert.Equal(t, "no-session", missingWorkflowContextCapability(request))
	request.Capabilities.NoSession = true
	assert.Empty(t, missingWorkflowContextCapability(request))
	request.Capabilities.MemoryInterception = false
	assert.Empty(t, missingWorkflowContextCapability(request), "memory off must not gate history active")
	request.Policy.MemoryMode = config.OMPContextMemoryShadow
	assert.Equal(t, "memory-interception", missingWorkflowContextCapability(request))

	assert.Equal(t, "ephemeral-state-unavailable", workflowContextFailureReason(promptlayer.ErrOMPContextBindingUnavailable, nil))
	assert.Equal(t, "event-sequence-invalid", workflowContextFailureReason(errors.New("unexpected OMP context runtime event"), nil))
	assert.Equal(t, "history-credit-unverified", workflowContextFailureReason(errors.New("bad history credit"), nil))
	assert.Equal(t, "runtime-cleanup-failed", workflowContextFailureReason(nil, errors.New("cleanup")))
	assert.Equal(t, "rehydration-verification-failed", workflowContextFailureReason(errors.New("other"), nil))
}

func TestWorkflowContextOverlay_RejectsControllerModeMemoryAndApplyErrors(t *testing.T) {
	t.Parallel()
	request := WorkflowContextOverlayRequest{HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff}
	_, err := ApplyWorkflowContextOverlay(context.Background(), nil, request)
	require.Error(t, err)
	_, err = ApplyWorkflowContextOverlay(context.Background(), overlayErrorStub{}, request)
	require.ErrorContains(t, err, "apply")

	wrongMode := activeOverlayReadback()
	wrongMode.EffectiveHistoryMode = config.OMPContextHistoryShadow
	_, err = ApplyWorkflowContextOverlay(context.Background(), newFakeWorkflowContextOverlay(t, wrongMode), request)
	require.ErrorContains(t, err, "effective mode")
	wrongMemory := activeOverlayReadback()
	wrongMemory.EffectiveMemoryMode = config.OMPContextMemoryShadow
	_, err = ApplyWorkflowContextOverlay(context.Background(), newFakeWorkflowContextOverlay(t, wrongMemory), request)
	require.ErrorContains(t, err, "memory mode")
}

func TestWorkflowContextRuntimeHelpers_CleanupAndReceiptFailureBranches(t *testing.T) {
	t.Parallel()
	receipt := WorkflowContextRuntimeReceipt{}
	err := cleanupWorkflowContextRuntime(context.Background(), &errorWorkflowContextDriver{cleanupErr: errors.New("denied")}, &receipt)
	require.Error(t, err)
	assert.Equal(t, "cleanup-failed", receipt.Cleanup.Reason)
	receipt = WorkflowContextRuntimeReceipt{}
	err = cleanupWorkflowContextRuntime(context.Background(), &errorWorkflowContextDriver{countErr: errors.New("readback")}, &receipt)
	require.Error(t, err)
	assert.Equal(t, "cleanup-readback-failed", receipt.Cleanup.Reason)
	receipt = WorkflowContextRuntimeReceipt{}
	err = cleanupWorkflowContextRuntime(context.Background(), &errorWorkflowContextDriver{count: 1}, &receipt)
	require.Error(t, err)
	assert.Equal(t, "artifacts-remain", receipt.Cleanup.Reason)

	writer := &WorkflowContextReceiptWriter{WorkspaceRoot: t.TempDir()}
	request := WorkflowContextRuntimeRequest{ReceiptWriter: writer}
	receipt = WorkflowContextRuntimeReceipt{TaskID: "../escape", SessionID: "session"}
	_, err = finishWorkflowContextRuntime(request, receipt, errors.New("primary"))
	require.Error(t, err)
}

func TestWorkflowContextSupervisor_PreflightFailuresRollbackBeforeDriver(t *testing.T) {
	t.Parallel()
	t.Run("overlay readback mismatch", func(t *testing.T) {
		request := newWorkflowContextRuntimeFixture(t)
		driver := &fakeWorkflowContextDriver{}
		request.Driver = driver
		mismatch := activeOverlayReadback()
		mismatch.ReadbackHash = runtimeHash("mismatch")
		request.Overlay = newFakeWorkflowContextOverlay(t, mismatch, shadowOverlayReadback())
		receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, nil)
		require.Error(t, err)
		assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
		assert.Zero(t, driver.runCalls)
	})

	t.Run("artifact count unavailable", func(t *testing.T) {
		request := newWorkflowContextRuntimeFixture(t)
		request.Driver = &errorWorkflowContextDriver{countErr: errors.New("unavailable")}
		request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, nil)
		require.Error(t, err)
		assert.Equal(t, "artifact-count-unavailable", receipt.Fallback.Reason)
		assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
	})

	t.Run("artifact count negative", func(t *testing.T) {
		request := newWorkflowContextRuntimeFixture(t)
		request.Driver = &errorWorkflowContextDriver{count: -1}
		request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, nil)
		require.ErrorContains(t, err, "negative")
		assert.Equal(t, "artifact-count-unavailable", receipt.Fallback.Reason)
		assert.Equal(t, config.OMPContextHistoryShadow, receipt.Mode.EffectiveHistoryMode)
	})
}

func TestWorkflowContextRuntimeReceiptAndDispatchRejectSerializationAbuse(t *testing.T) {
	t.Parallel()
	_, err := json.Marshal(WorkflowContextDispatch{})
	require.ErrorIs(t, err, promptlayer.ErrOMPContextBodySerialization)
	writer := WorkflowContextReceiptWriter{WorkspaceRoot: t.TempDir()}
	receipt := WorkflowContextRuntimeReceipt{
		SchemaVersion: WorkflowContextRuntimeReceiptSchemaVersion, Event: "terminal",
		TaskID: "TASK-1", SessionID: "session-1", RootClass: config.OMPContextRuntimeIsolatedTaskOwned,
		DocumentOmissions: []string{"AGENTS.md"}, MemoryInjections: []string{},
	}
	require.ErrorContains(t, writer.Write(receipt), "document omissions")
	receipt.DocumentOmissions = nil
	receipt.SessionID = "../escape"
	require.Error(t, writer.Write(receipt))
}

func TestWorkflowContextCanonicalFallbackAndCanaryRejectMissingAuthority(t *testing.T) {
	t.Parallel()
	request := newWorkflowContextRuntimeFixture(t)
	receipt := newWorkflowContextRuntimeReceipt(request)
	receipt.PhaseSequence = []string{"checkpointed", "compacted"}
	got, err := NewWorkflowContextRuntimeSupervisor(nil).runCanonicalFallback(
		context.Background(), request, receipt, (&recordingWorkflowContextDispatcher{}).Dispatch,
	)
	require.ErrorContains(t, err, "independent canonical fallback")
	assert.Equal(t, WorkflowContextOutcomeBlocked, got.Outcome)
	assert.Equal(t, config.OMPContextFallbackBlock, got.Fallback.Mode)

	_, err = RunWorkflowContextInstalledCanary(context.Background(), nil, request, nil)
	require.ErrorContains(t, err, "supervisor")
	request.Capabilities.Version = "OMP version unknown"
	_, err = RunWorkflowContextInstalledCanary(context.Background(), NewWorkflowContextRuntimeSupervisor(nil), request, nil)
	require.ErrorContains(t, err, "identity")
}

func TestWorkflowContextCanonicalFallback_FailureBranchesStayBlocked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		source     func(WorkflowContextRuntimeRequest) WorkflowContextCanonicalSource
		dispatch   WorkflowContextDispatchFunc
		wantReason string
	}{
		{
			name: "rebuild error",
			source: func(WorkflowContextRuntimeRequest) WorkflowContextCanonicalSource {
				return workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
					return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{}, errors.New("rebuild failed")
				})
			},
			dispatch: (&recordingWorkflowContextDispatcher{}).Dispatch, wantReason: "canonical-full-rebuild-failed",
		},
		{
			name: "binding invalid",
			source: func(request WorkflowContextRuntimeRequest) WorkflowContextCanonicalSource {
				return workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
					return request.Binding.Delivery, promptlayer.OMPContextEphemeral{}, nil
				})
			},
			dispatch: (&recordingWorkflowContextDispatcher{}).Dispatch, wantReason: "canonical-full-binding-invalid",
		},
		{
			name: "dispatch missing",
			source: func(request WorkflowContextRuntimeRequest) WorkflowContextCanonicalSource {
				return workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
					return request.Binding.Delivery, request.Binding.Ephemeral, nil
				})
			},
			dispatch: nil, wantReason: "canonical-full-dispatch-unavailable",
		},
		{
			name: "dispatch failed",
			source: func(request WorkflowContextRuntimeRequest) WorkflowContextCanonicalSource {
				return workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
					return request.Binding.Delivery, request.Binding.Ephemeral, nil
				})
			},
			dispatch:   func(context.Context, WorkflowContextDispatch) error { return errors.New("provider rejected") },
			wantReason: "canonical-full-dispatch-failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newWorkflowContextRuntimeFixture(t)
			request.Overlay = newFakeWorkflowContextOverlay(t, shadowOverlayReadback())
			request.CanonicalSource = tt.source(request)
			receipt := newWorkflowContextRuntimeReceipt(request)
			got, err := NewWorkflowContextRuntimeSupervisor(nil).runCanonicalFallback(context.Background(), request, receipt, tt.dispatch)
			require.Error(t, err)
			assert.Equal(t, WorkflowContextOutcomeBlocked, got.Outcome)
			assert.Equal(t, config.OMPContextFallbackBlock, got.Fallback.Mode)
			assert.Equal(t, tt.wantReason, got.Fallback.Reason)
		})
	}
}

type overlayErrorStub struct{}

func (overlayErrorStub) Apply(context.Context, WorkflowContextOverlayRequest) (WorkflowContextOverlayReadback, error) {
	return WorkflowContextOverlayReadback{}, errors.New("overlay unavailable")
}

type errorWorkflowContextDriver struct {
	count      int
	countErr   error
	cleanupErr error
}

func (d *errorWorkflowContextDriver) ArtifactCount(context.Context) (int, error) {
	return d.count, d.countErr
}
func (*errorWorkflowContextDriver) Run(context.Context, func(WorkflowContextRuntimeEvent) error) error {
	return nil
}
func (d *errorWorkflowContextDriver) Cleanup(context.Context) error { return d.cleanupErr }
