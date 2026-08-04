package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCProduct_TwoCycleAdmissionReusesSessionAndFailsClosed(t *testing.T) {
	tests := []struct {
		name               string
		secondPostID       string
		stateSessions      []string
		completeBoundaries []bool
		compactCommand     string
		manualNativeEnd    string
		wantFirstError     string
		wantSecondError    string
		rejectSecond       bool
		wantBarrier        bool
	}{
		{name: "manual compact response before native end admits twice", wantBarrier: true},
		{name: "stale native end before compact response is blocked", manualNativeEnd: "before-response", rejectSecond: true},
		{name: "duplicate native end before state is blocked", manualNativeEnd: "duplicate", rejectSecond: true},
		{
			name:            "second cycle session drift is blocked",
			stateSessions:   []string{"session-1", "session-1", "session-1", "session-drift", "session-drift"},
			wantSecondError: "post-compaction state is not admission-safe",
		},
		{name: "provider turn session drift is blocked", stateSessions: []string{"session-1", "session-drift"},
			wantFirstError: "admitted provider state is not session-bound"},
		{name: "second cycle post hook replay is blocked", secondPostID: "post-cycle-1", wantSecondError: "replayed"},
		{name: "second cycle provider boundary omission is blocked", completeBoundaries: []bool{true, true, false},
			wantSecondError: "provider boundary"},
		{
			name:           "second cycle rejects a mismatched compact response",
			compactCommand: "prompt", wantSecondError: "manual compaction completion is invalid",
		},
		{name: "second cycle missing native end is blocked", manualNativeEnd: "missing", rejectSecond: true},
		{name: "second cycle aborted native end is blocked", manualNativeEnd: "aborted", rejectSecond: true},
		{name: "second cycle skipped native end is blocked", manualNativeEnd: "skipped", rejectSecond: true},
		{name: "second cycle native end missing result is blocked", manualNativeEnd: "missing-result", rejectSecond: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.secondPostID == "" {
				test.secondPostID = "post-cycle-2"
			}
			if test.stateSessions == nil {
				test.stateSessions = []string{"session-1", "session-1", "session-1", "session-1", "session-1"}
			}
			if test.completeBoundaries == nil {
				test.completeBoundaries = []bool{true, true, true}
			}
			frames := make(chan []byte, 32)
			writer := &workflowContextMultiCycleRPCWriter{
				frames: frames, stateSessions: test.stateSessions,
				completeBoundaries: test.completeBoundaries, compactCommand: test.compactCommand,
				manualNativeEnd: test.manualNativeEnd, autoCompaction: true,
			}
			protocol := newWorkflowContextManagedRPCProtocol(writer, frames, make(chan error))
			binding := managedProductBinding()
			process, err := os.FindProcess(os.Getpid())
			require.NoError(t, err)
			driver := &WorkflowContextManagedRPCDriver{
				running: true, protocol: protocol,
				process: &workflowContextManagedRPCProcess{cmd: &exec.Cmd{Process: process}, started: true},
				binding: binding, protocolPostID: "post-cycle-1", protocolSessionID: "session-1",
				dispatchState: workflowContextManagedDispatchPending,
				options:       WorkflowContextManagedRPCOptions{CompactionCycles: 2},
				observation:   WorkflowContextManagedRPCObservation{PID: os.Getpid()},
			}
			request := newWorkflowContextRuntimeFixture(t)

			managedProductPushFrame(t, frames, workflowContextMultiCycleManualNativeEnd(""))
			firstErr := dispatchWorkflowContextMultiCycle(t, driver, request)
			if test.wantFirstError != "" {
				assert.ErrorContains(t, firstErr, test.wantFirstError)
				return
			}
			require.NoError(t, firstErr)
			if test.wantBarrier {
				assert.False(t, writer.autoCompaction)
			}
			driver.setPendingWorkflowContextManagedDispatch(test.secondPostID, "session-1")
			secondErr := dispatchWorkflowContextMultiCycle(t, driver, request)

			if test.rejectSecond {
				require.Error(t, secondErr)
				assert.Equal(t, 2, writer.completedProviderBoundaries(),
					"an invalid second cycle must not become provider-observed")
				assert.Equal(t, []string{"managed-admission-1", "managed-cycle-trigger-2"},
					writer.providerPromptIDs(), "invalid native completion must fail before a second admission prompt")
				assert.Len(t, writer.providerAdmissionIDs(), 1)
				assert.Equal(t, 1, driver.Observation().NativeEnds,
					"only the first correlated native completion may be counted")
				return
			}
			if test.wantSecondError != "" {
				assert.ErrorContains(t, secondErr, test.wantSecondError)
				assert.Equal(t, 2, writer.completedProviderBoundaries(),
					"an invalid second cycle must not become provider-observed")
				return
			}
			assert.NoError(t, secondErr)
			assert.Equal(t, 3, writer.completedProviderBoundaries())
			admissionIDs := writer.providerAdmissionIDs()
			assert.Len(t, admissionIDs, 2)
			if len(admissionIDs) == 2 {
				assert.NotEqual(t, admissionIDs[0], admissionIDs[1],
					"provider admission RPC IDs must be unique per compaction cycle")
			}
			assert.Equal(t, []string{"managed-admission-1", "managed-cycle-trigger-2", "managed-admission-2"},
				writer.providerPromptIDs())
			assert.Equal(t, []string{"managed-compact-2"}, writer.manualCompactIDs())
			observation := driver.Observation()
			assert.Equal(t, 2, observation.PostACKs)
			assert.Equal(t, 2, observation.NativeEnds)
			assert.Equal(t, 3, observation.ProviderTurns)
			assert.True(t, observation.SameSession)
			assert.True(t, observation.SameProcess)
		})
	}
}

func dispatchWorkflowContextMultiCycle(
	t *testing.T, driver *WorkflowContextManagedRPCDriver, request WorkflowContextRuntimeRequest,
) error {
	t.Helper()
	store := promptlayer.NewOMPContextTransientStore()
	binding, err := store.Checkpoint(request.Binding)
	if err != nil {
		return err
	}
	delivery, err := promptlayer.BuildContextDelivery(request.Binding.DeliveryOptions)
	if err != nil {
		return err
	}
	_, err = store.RehydrateCanonical(
		binding.BindingHash, request.Binding.DeliveryOptions, delivery,
		func(view promptlayer.OMPContextTransientView) error {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_, dispatchErr := driver.Dispatch(ctx, WorkflowContextDispatch{
				Mode: WorkflowContextDispatchOptimized, Delivery: delivery, Transient: view,
			})
			return dispatchErr
		},
	)
	return err
}

type workflowContextMultiCycleRPCWriter struct {
	mu                 sync.Mutex
	frames             chan []byte
	stateSessions      []string
	completeBoundaries []bool
	compactCommand     string
	manualNativeEnd    string
	autoCompaction     bool
	stateIndex         int
	promptIDs          []string
	compactIDs         []string
	completed          int
}

func (writer *workflowContextMultiCycleRPCWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	var request map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &request); err != nil {
		return 0, err
	}
	id, _ := request["id"].(string)
	typeName, _ := request["type"].(string)
	switch typeName {
	case "get_state":
		if writer.stateIndex >= len(writer.stateSessions) {
			return 0, errors.New("unexpected state readback")
		}
		state := workflowContextManagedRPCState{
			SessionID: writer.stateSessions[writer.stateIndex], AutoCompactionEnabled: writer.autoCompaction,
			MessageCount: writer.stateIndex * 2,
		}
		writer.stateIndex++
		writer.push(workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: "get_state", Success: workflowContextMultiCycleBoolPointer(true),
			Data: mustWorkflowContextMultiCycleJSON(state),
		})
	case "set_auto_compaction":
		writer.autoCompaction, _ = request["enabled"].(bool)
		writer.push(workflowContextManagedRPCFrame{ID: id, Type: "response", Command: typeName,
			Success: workflowContextMultiCycleBoolPointer(true)})
	case "prompt":
		writer.promptIDs = append(writer.promptIDs, id)
		index := len(writer.promptIDs) - 1
		writer.push(workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: "prompt", Success: workflowContextMultiCycleBoolPointer(true),
		})
		writer.push(workflowContextManagedRPCFrame{Type: "agent_start"})
		writer.push(workflowContextManagedRPCFrame{Type: "turn_end"})
		if index < len(writer.completeBoundaries) && writer.completeBoundaries[index] {
			writer.push(workflowContextManagedRPCFrame{Type: "agent_end"})
			writer.completed++
		} else {
			close(writer.frames)
		}
	case "compact":
		writer.compactIDs = append(writer.compactIDs, id)
		command := writer.compactCommand
		if command == "" {
			command = "compact"
		}
		response := workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: command, Success: workflowContextMultiCycleBoolPointer(true),
		}
		nativeEnd := workflowContextMultiCycleManualNativeEnd(writer.manualNativeEnd)
		if writer.manualNativeEnd == "before-response" {
			writer.push(nativeEnd)
		}
		writer.push(response)
		if writer.manualNativeEnd != "missing" && writer.manualNativeEnd != "before-response" {
			writer.push(nativeEnd)
		}
		if writer.manualNativeEnd == "duplicate" {
			writer.push(nativeEnd)
		}
	}
	return len(data), nil
}

func workflowContextMultiCycleManualNativeEnd(mode string) workflowContextManagedRPCFrame {
	frame := workflowContextManagedRPCFrame{
		Type: "auto_compaction_end", Action: "snapcompact", Result: json.RawMessage(`{}`),
	}
	switch mode {
	case "aborted":
		frame.Aborted = true
	case "skipped":
		frame.Skipped = true
	case "missing-result":
		frame.Result = nil
	}
	return frame
}

func (writer *workflowContextMultiCycleRPCWriter) push(frame workflowContextManagedRPCFrame) {
	writer.frames <- mustWorkflowContextMultiCycleJSON(frame)
}

func (writer *workflowContextMultiCycleRPCWriter) providerAdmissionIDs() []string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	result := make([]string, 0, 2)
	for _, id := range writer.promptIDs {
		if strings.HasPrefix(id, "managed-admission-") {
			result = append(result, id)
		}
	}
	return result
}

func (writer *workflowContextMultiCycleRPCWriter) providerPromptIDs() []string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]string(nil), writer.promptIDs...)
}

func (writer *workflowContextMultiCycleRPCWriter) manualCompactIDs() []string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]string(nil), writer.compactIDs...)
}

func (writer *workflowContextMultiCycleRPCWriter) completedProviderBoundaries() int {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.completed
}

func mustWorkflowContextMultiCycleJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func workflowContextMultiCycleBoolPointer(value bool) *bool { return &value }
