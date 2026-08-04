package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCCompletion_RejectsInterruptedAndAmbiguousFrames(t *testing.T) {
	t.Parallel()
	success := true
	validResponse := workflowContextManagedRPCFrame{
		ID: "compact-1", Type: "response", Command: "compact", Success: &success,
	}
	tests := []struct {
		name   string
		manual bool
		frames []workflowContextManagedRPCFrame
		want   string
	}{
		{name: "native extension error", frames: []workflowContextManagedRPCFrame{{Type: "extension_error"}}, want: "unexpected activity"},
		{name: "native UI request", frames: []workflowContextManagedRPCFrame{{Type: "extension_ui_request"}}, want: "unexpected activity"},
		{name: "manual first extension error", manual: true, frames: []workflowContextManagedRPCFrame{{Type: "extension_error"}}, want: "extension failed"},
		{name: "manual first UI request", manual: true, frames: []workflowContextManagedRPCFrame{{Type: "extension_ui_request"}}, want: "unexpected UI"},
		{name: "manual invalid response", manual: true, frames: []workflowContextManagedRPCFrame{{ID: "wrong", Type: "response", Command: "compact", Success: &success}}, want: "completion is invalid"},
		{name: "manual native extension error", manual: true, frames: []workflowContextManagedRPCFrame{validResponse, {Type: "extension_error"}}, want: "extension failed"},
		{name: "manual native UI request", manual: true, frames: []workflowContextManagedRPCFrame{validResponse, {Type: "extension_ui_request"}}, want: "unexpected UI"},
		{name: "manual duplicate response", manual: true, frames: []workflowContextManagedRPCFrame{validResponse, validResponse}, want: "duplicated"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			protocol := coverageGapProtocol(t, test.frames...)
			var err error
			if test.manual {
				err = protocol.awaitManualCompactionCompletion(context.Background(), "compact-1")
			} else {
				err = protocol.awaitNativeCompactionEnd(context.Background())
			}
			require.ErrorContains(t, err, test.want)
		})
	}
	for _, manual := range []bool{false, true} {
		protocol := coverageGapProtocol(t)
		var err error
		if manual {
			err = protocol.awaitManualCompactionCompletion(context.Background(), "compact-1")
		} else {
			err = protocol.awaitNativeCompactionEnd(context.Background())
		}
		require.ErrorIs(t, err, os.ErrClosed)
	}
}

func TestWorkflowContextManagedRPCDriver_OwnershipLifecycleIsObservable(t *testing.T) {
	options := managedProductTestOptions(t)
	driver, err := NewWorkflowContextManagedRPCDriver(options)
	require.NoError(t, err)
	require.NoError(t, driver.Bind(context.Background(), managedProductBinding()))
	require.ErrorContains(t, driver.Bind(context.Background(), managedProductBinding()), "not replaceable")
	count, err := driver.ArtifactCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	require.NoError(t, driver.Cleanup(context.Background()))
	count, err = driver.ArtifactCount(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
	require.NoError(t, driver.Cleanup(context.Background()), "closed cleanup must be idempotent")
	assert.False(t, driver.Observation().ProcessActiveAfterCleanup)

	invalid := &WorkflowContextManagedRPCDriver{options: WorkflowContextManagedRPCOptions{RuntimeRoot: "\x00"}}
	_, err = invalid.ArtifactCount(context.Background())
	require.ErrorContains(t, err, "inspect managed OMP runtime")
}

func TestPipelineOMPBackend_PreSpawnAuthorityFailuresStayBodyFree(t *testing.T) {
	config, logPath := pipelineOMPBackendTestConfig(t)
	backend, err := newPipelineOMPBackend(config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	tests := []struct {
		name    string
		request func(*testing.T) pipeline.PhaseRequest
		want    string
		canary  string
	}{
		{name: "binding mismatch", request: func(t *testing.T) pipeline.PhaseRequest {
			request := sealedPipelineOMPRequest(t, config, pipeline.PhasePlan, "trusted prompt PRIVATE-BINDING-BODY", nil)
			request.PhaseID = pipeline.PhaseImplement
			return request
		}, want: "binding mismatch", canary: "PRIVATE-BINDING-BODY"},
		{name: "authority mismatch", request: func(t *testing.T) pipeline.PhaseRequest {
			other := config
			other.ProjectDir = t.TempDir()
			other.SpecDir = filepath.Join(other.ProjectDir, "SPEC-OMP-004")
			return sealedPipelineOMPRequest(t, other, pipeline.PhasePlan, "trusted prompt PRIVATE-AUTHORITY-BODY", nil)
		}, want: "authority mismatch", canary: "PRIVATE-AUTHORITY-BODY"},
		{name: "nested auto command", request: func(t *testing.T) pipeline.PhaseRequest {
			return sealedPipelineOMPRequest(t, config, pipeline.PhasePlan, "/auto go SPEC-OMP-004 PRIVATE-NESTED-AUTO-BODY", nil)
		}, want: "cannot reissue /auto", canary: "PRIVATE-NESTED-AUTO-BODY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := backend.Execute(context.Background(), test.request(t))
			require.ErrorContains(t, err, test.want)
			assert.Equal(t, "execution_error", response.FailureClass)
			assert.Empty(t, response.Output)
			assert.NotContains(t, err.Error(), test.canary)
			assert.NotContains(t, response.Output, test.canary)
		})
	}
	_, statErr := os.Stat(logPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	require.NoError(t, backend.Close())
	response, err := backend.Execute(context.Background(), sealedPipelineOMPRequest(t, config, pipeline.PhasePlan, "trusted prompt", nil))
	require.ErrorContains(t, err, "backend is closed")
	assert.Empty(t, response.Output)
	_, err = newPipelineOMPBackend(pipelineOMPBackendConfig{})
	require.ErrorContains(t, err, "is required")
}

func coverageGapProtocol(t *testing.T, frames ...workflowContextManagedRPCFrame) *workflowContextManagedRPCProtocol {
	t.Helper()
	input := make(chan []byte, len(frames))
	for _, frame := range frames {
		managedProductPushFrame(t, input, frame)
	}
	close(input)
	done := make(chan error, 1)
	done <- os.ErrClosed
	return newWorkflowContextManagedRPCProtocol(&bytes.Buffer{}, input, done)
}
