package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProductSession_FailureAuthorityStopsBeforeDriverRun(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *WorkflowContextProductSessionInput, *WorkflowContextProductRuntimeInputs)
		want   string
	}{
		{name: "runtime unavailable", mutate: func(_ *testing.T, _ *WorkflowContextProductSessionInput, runtime *WorkflowContextProductRuntimeInputs) {
			runtime.Supervisor = nil
		}, want: "runtime inputs"},
		{name: "invalid harness config", mutate: func(t *testing.T, input *WorkflowContextProductSessionInput, _ *WorkflowContextProductRuntimeInputs) {
			require.NoError(t, os.WriteFile(filepath.Join(input.ProjectDir, "autopus.yaml"), []byte("{\n"), 0o600))
		}, want: "load harness config"},
		{name: "inactive policy", mutate: func(t *testing.T, input *WorkflowContextProductSessionInput, _ *WorkflowContextProductRuntimeInputs) {
			require.NoError(t, config.Save(input.ProjectDir, config.DefaultFullConfig("inactive")))
		}, want: "selected active"},
		{name: "binding invalid", mutate: func(_ *testing.T, input *WorkflowContextProductSessionInput, _ *WorkflowContextProductRuntimeInputs) {
			input.TaskID = ""
		}, want: "build context binding"},
		{name: "factory error", mutate: func(_ *testing.T, _ *WorkflowContextProductSessionInput, runtime *WorkflowContextProductRuntimeInputs) {
			runtime.NewManagedDriver = func(WorkflowContextManagedRPCOptions) (WorkflowContextManagedProcessDriver, error) {
				return nil, errors.New("factory denied")
			}
		}, want: "construct managed driver"},
		{name: "factory nil driver", mutate: func(_ *testing.T, _ *WorkflowContextProductSessionInput, runtime *WorkflowContextProductRuntimeInputs) {
			runtime.NewManagedDriver = func(WorkflowContextManagedRPCOptions) (WorkflowContextManagedProcessDriver, error) {
				return nil, nil
			}
		}, want: "returned nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, runtime, driver, factory := newWorkflowContextProductFixture(t)
			test.mutate(t, &input, &runtime)
			_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)
			require.ErrorContains(t, err, test.want)
			assert.Zero(t, driver.runCalls)
			if test.name != "factory error" && test.name != "factory nil driver" {
				assert.Zero(t, factory.calls)
			}
		})
	}
}

func TestWorkflowContextProductCanonicalInput_RejectsIncompleteFilesystemAuthority(t *testing.T) {
	input, _, _, _ := newWorkflowContextProductFixture(t)
	for _, test := range []struct {
		name string
		in   WorkflowContextProductSessionInput
		want string
	}{
		{name: "empty project", in: WorkflowContextProductSessionInput{}, want: "project directory is required"},
		{name: "missing config", in: func() WorkflowContextProductSessionInput {
			value := input
			value.ProjectDir = t.TempDir()
			return value
		}(), want: "autopus.yaml is required"},
		{name: "empty spec directory", in: func() WorkflowContextProductSessionInput {
			value := input
			value.SpecDir = ""
			return value
		}(), want: "canonical SPEC directory is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := canonicalWorkflowContextProductInput(test.in)
			require.ErrorContains(t, err, test.want)
		})
	}

	alias := input
	alias.OriginalTask = "/auto-go SPEC-OMP-004 --auto"
	require.NoError(t, validateWorkflowContextProductPromptAuthority(alias))
	source := workflowContextProductCanonicalSource{options: promptlayer.ContextDeliveryOptions{Root: input.ProjectDir}}
	_, _, err := source.Rebuild(context.Background(), promptlayer.ContextDeliveryOptions{Root: t.TempDir()})
	require.ErrorContains(t, err, "options changed")
}

func TestWorkflowContextSupervisor_TerminalFailuresPreserveExactReasons(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *WorkflowContextRuntimeRequest)
		want   string
	}{
		{name: "invalid request", mutate: func(_ *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Policy.HistoryMode = config.OMPContextHistoryShadow
		}, want: "runtime-request-invalid"},
		{name: "missing runtime dependency", mutate: func(_ *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = nil
			request.Overlay = nil
		}, want: "runtime-dependency-unavailable"},
		{name: "invalid history credit", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = &coverageGapDriver{events: []WorkflowContextRuntimeEvent{
				{Kind: WorkflowContextEventPreCompaction},
				{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{}},
			}}
			request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		}, want: "history-credit-unverified"},
		{name: "missing dispatch", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = coverageGapSuccessfulDriver()
			request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		}, want: "rehydration-verification-failed"},
		{name: "rollback mismatch", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = coverageGapSuccessfulDriver()
			request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), activeOverlayReadback())
		}, want: "rollback-readback-mismatch"},
		{name: "cleanup failure", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = &coverageGapDriver{events: coverageGapSuccessfulDriver().events, artifacts: 1, cleanupErr: errors.New("cleanup denied")}
			request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
		}, want: "runtime-cleanup-failed"},
		{name: "receipt write failure", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Driver = coverageGapSuccessfulDriver()
			request.Overlay = newFakeWorkflowContextOverlay(t, activeOverlayReadback(), shadowOverlayReadback())
			path := filepath.Join(t.TempDir(), "not-a-directory")
			require.NoError(t, os.WriteFile(path, []byte("occupied"), 0o600))
			request.ReceiptWriter = &WorkflowContextReceiptWriter{WorkspaceRoot: path}
		}, want: "receipt-write-failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newWorkflowContextRuntimeFixture(t)
			test.mutate(t, &request)
			var dispatch WorkflowContextDispatchFunc = func(context.Context, WorkflowContextDispatch) error { return nil }
			if test.name == "missing dispatch" {
				dispatch = nil
			}
			receipt, err := NewWorkflowContextRuntimeSupervisor(nil).Run(context.Background(), request, dispatch)
			require.Error(t, err)
			assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
			assert.Equal(t, test.want, receipt.Fallback.Reason)
		})
	}
}

func TestWorkflowContextCanonicalFallback_RollbackCleanupAndVerificationFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *WorkflowContextRuntimeRequest)
		want   string
	}{
		{name: "rollback", mutate: func(_ *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Overlay = overlayErrorStub{}
		}, want: "rollback-readback-mismatch"},
		{name: "cleanup", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Overlay = newFakeWorkflowContextOverlay(t, shadowOverlayReadback())
			request.Driver = &coverageGapDriver{artifacts: 1, cleanupErr: errors.New("cleanup denied")}
		}, want: "runtime-cleanup-failed"},
		{name: "verification", mutate: func(t *testing.T, request *WorkflowContextRuntimeRequest) {
			request.Overlay = newFakeWorkflowContextOverlay(t, shadowOverlayReadback())
			request.Driver = nil
			request.CanonicalSource = workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
				delivery := request.Binding.Delivery
				delivery.SnapshotHash = runtimeHash("corrupt")
				return delivery, request.Binding.Ephemeral, nil
			})
		}, want: "canonical-full-verification-failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newWorkflowContextRuntimeFixture(t)
			request.CanonicalSource = workflowContextCanonicalSourceFunc(func(context.Context, promptlayer.ContextDeliveryOptions) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
				return request.Binding.Delivery, request.Binding.Ephemeral, nil
			})
			test.mutate(t, &request)
			receipt, err := NewWorkflowContextRuntimeSupervisor(nil).runCanonicalFallback(
				context.Background(), request, newWorkflowContextRuntimeReceipt(request), nil,
			)
			require.Error(t, err)
			assert.Equal(t, WorkflowContextOutcomeBlocked, receipt.Outcome)
			assert.Equal(t, test.want, receipt.Fallback.Reason)
		})
	}
}

type coverageGapDriver struct {
	events       []WorkflowContextRuntimeEvent
	artifacts    int
	cleanupErr   error
	cleanupCalls int
}

func coverageGapSuccessfulDriver() *coverageGapDriver {
	return &coverageGapDriver{artifacts: 1, events: []WorkflowContextRuntimeEvent{
		{Kind: WorkflowContextEventPreCompaction},
		{Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2}},
		{Kind: WorkflowContextEventPostCompaction},
	}}
}

func (driver *coverageGapDriver) ArtifactCount(context.Context) (int, error) {
	return driver.artifacts, nil
}

func (driver *coverageGapDriver) Run(_ context.Context, emit func(WorkflowContextRuntimeEvent) error) error {
	for _, event := range driver.events {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func (driver *coverageGapDriver) Cleanup(context.Context) error {
	driver.cleanupCalls++
	if driver.cleanupErr == nil {
		driver.artifacts = 0
	}
	return driver.cleanupErr
}
