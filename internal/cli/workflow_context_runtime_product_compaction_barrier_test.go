package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCProduct_DispatchRequestsCompactionPauseBeforeAdmission(t *testing.T) {
	tests := []struct {
		name, disableResponse string
		lifecycleOnAdmission  bool
		nextCompaction        bool
		wantErr               bool
	}{
		{name: "barrier orders canonical admission and explicit next compact"},
		{name: "valid next compaction confirm is deferred", nextCompaction: true},
		{name: "missing disable response fails closed", disableResponse: "missing", wantErr: true},
		{name: "wrong disable response fails closed", disableResponse: "wrong", wantErr: true},
		{name: "lifecycle during admission fails closed", lifecycleOnAdmission: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frames := make(chan []byte, 64)
			writer := &workflowContextCompactionBarrierWriter{
				frames: frames, autoCompaction: true, disableResponse: test.disableResponse,
				lifecycleOnAdmission: test.lifecycleOnAdmission, nextCompaction: test.nextCompaction,
			}
			protocol := newWorkflowContextManagedRPCProtocol(writer, frames, make(chan error))
			process, err := os.FindProcess(os.Getpid())
			require.NoError(t, err)
			driver := &WorkflowContextManagedRPCDriver{
				running: true, protocol: protocol,
				process: &workflowContextManagedRPCProcess{cmd: &exec.Cmd{Process: process}, started: true},
				binding: managedProductBinding(), protocolPostID: "post-cycle-1", protocolSessionID: "session-1",
				dispatchState: workflowContextManagedDispatchPending,
				options:       WorkflowContextManagedRPCOptions{CompactionCycles: 2},
				observation:   WorkflowContextManagedRPCObservation{PID: os.Getpid()},
			}
			managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
				Type: "auto_compaction_end", Action: "snapcompact", Result: json.RawMessage(`{}`),
			})

			dispatchErr := dispatchWorkflowContextMultiCycle(t, driver, newWorkflowContextRuntimeFixture(t))
			if test.wantErr {
				require.Error(t, dispatchErr)
				assert.False(t, driver.Observation().ProviderObserved)
				assert.Equal(t, 0, writer.completedProviderBoundaries())
				assert.Equal(t, "set_auto_compaction:managed-compaction-barrier-1", writer.firstCommand())
				if test.lifecycleOnAdmission {
					assert.Equal(t, []string{"managed-admission-1"}, writer.providerPromptIDs())
				} else {
					assert.Empty(t, writer.providerPromptIDs())
				}
				return
			}

			require.NoError(t, dispatchErr)
			if test.nextCompaction {
				assert.Equal(t, []string{"set_auto_compaction:managed-compaction-barrier-1",
					"get_state:managed-state-after", "prompt:managed-admission-1"}, writer.commandsSnapshot())
				assert.Zero(t, writer.manualCompacts())
				assert.True(t, driver.Observation().ProviderObserved)
				return
			}
			assert.Equal(t, []string{
				"set_auto_compaction:managed-compaction-barrier-1",
				"get_state:managed-state-after",
				"prompt:managed-admission-1",
				"get_state:managed-state-admitted",
				"prompt:managed-cycle-trigger-2",
				"get_state:managed-cycle-trigger-state-2",
				"compact:managed-compact-2",
			}, writer.commandsSnapshot())
			assert.Zero(t, writer.promptsWhileCompactionEnabled())
			assert.Equal(t, 2, writer.completedProviderBoundaries())
			assert.Equal(t, 1, writer.manualCompacts())
			assert.True(t, writer.commandIDsUnique())
		})
	}
}

type workflowContextCompactionBarrierWriter struct {
	mu                   sync.Mutex
	frames               chan []byte
	autoCompaction       bool
	disableResponse      string
	lifecycleOnAdmission bool
	nextCompaction       bool
	stateReads           int
	completed            int
	manualCompactCount   int
	promptWhileEnabled   int
	commands             []string
	commandIDs           []string
	prompts              []string
}

func (writer *workflowContextCompactionBarrierWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	var request map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &request); err != nil {
		return 0, err
	}
	id, _ := request["id"].(string)
	typeName, _ := request["type"].(string)
	if typeName != "extension_ui_response" {
		writer.commands = append(writer.commands, typeName+":"+id)
		writer.commandIDs = append(writer.commandIDs, id)
	}
	switch typeName {
	case "set_auto_compaction":
		if writer.disableResponse == "missing" {
			return len(data), nil
		}
		command := "set_auto_compaction"
		if writer.disableResponse == "wrong" {
			command = "set_auto_retry"
		} else if enabled, ok := request["enabled"].(bool); ok {
			writer.autoCompaction = enabled
		}
		writer.push(workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: command, Success: workflowContextMultiCycleBoolPointer(true),
		})
	case "get_state":
		state := workflowContextManagedRPCState{
			SessionID: "session-1", AutoCompactionEnabled: writer.autoCompaction,
			MessageCount: writer.stateReads * 2,
		}
		writer.stateReads++
		writer.push(workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: "get_state", Success: workflowContextMultiCycleBoolPointer(true),
			Data: mustWorkflowContextMultiCycleJSON(state),
		})
	case "prompt":
		writer.prompts = append(writer.prompts, id)
		if writer.autoCompaction {
			writer.promptWhileEnabled++
		}
		if writer.lifecycleOnAdmission && id == "managed-admission-1" {
			writer.push(workflowContextManagedRPCFrame{
				Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact",
			})
			return len(data), nil
		}
		writer.push(workflowContextManagedRPCFrame{
			ID: id, Type: "response", Command: "prompt", Success: workflowContextMultiCycleBoolPointer(true),
		})
		writer.push(workflowContextManagedRPCFrame{Type: "agent_start"})
		writer.push(workflowContextManagedRPCFrame{Type: "turn_end"})
		if writer.nextCompaction && id == "managed-admission-1" {
			writer.push(workflowContextManagedRPCFrame{Type: "extension_ui_request", Method: "confirm", ID: "pre-2"})
			return len(data), nil
		}
		writer.push(workflowContextManagedRPCFrame{Type: "agent_end"})
		writer.completed++
	case "compact":
		writer.manualCompactCount++
	}
	return len(data), nil
}

func (writer *workflowContextCompactionBarrierWriter) push(frame workflowContextManagedRPCFrame) {
	writer.frames <- mustWorkflowContextMultiCycleJSON(frame)
}

func (writer *workflowContextCompactionBarrierWriter) snapshot() ([]string, []string, int, int, int) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]string(nil), writer.commands...), append([]string(nil), writer.commandIDs...),
		writer.completed, writer.manualCompactCount, writer.promptWhileEnabled
}

func (writer *workflowContextCompactionBarrierWriter) commandsSnapshot() []string {
	commands, _, _, _, _ := writer.snapshot()
	return commands
}
func (writer *workflowContextCompactionBarrierWriter) firstCommand() string {
	commands := writer.commandsSnapshot()
	if len(commands) == 0 {
		return ""
	}
	return commands[0]
}
func (writer *workflowContextCompactionBarrierWriter) providerPromptIDs() []string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]string(nil), writer.prompts...)
}
func (writer *workflowContextCompactionBarrierWriter) completedProviderBoundaries() int {
	_, _, completed, _, _ := writer.snapshot()
	return completed
}
func (writer *workflowContextCompactionBarrierWriter) manualCompacts() int {
	_, _, _, compacts, _ := writer.snapshot()
	return compacts
}
func (writer *workflowContextCompactionBarrierWriter) promptsWhileCompactionEnabled() int {
	_, _, _, _, prompts := writer.snapshot()
	return prompts
}
func (writer *workflowContextCompactionBarrierWriter) commandIDsUnique() bool {
	_, ids, _, _, _ := writer.snapshot()
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
