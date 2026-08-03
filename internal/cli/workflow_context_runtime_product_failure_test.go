package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWorkflowContextManagedRPCProduct_DispatchLeaseIsOneShot(t *testing.T) {
	driver := &WorkflowContextManagedRPCDriver{
		running: true, protocol: &workflowContextManagedRPCProtocol{},
		process: &workflowContextManagedRPCProcess{}, protocolPostID: "post-1",
		protocolSessionID: "session-1", dispatchState: workflowContextManagedDispatchPending,
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := driver.beginWorkflowContextManagedDispatch()
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("dispatch lease successes = %d; want exactly one", successes)
	}
	driver.finishWorkflowContextManagedDispatch(false)
	if _, err := driver.beginWorkflowContextManagedDispatch(); err == nil {
		t.Fatal("failed dispatch lease was reusable")
	}
}

func TestWorkflowContextManagedRPCProduct_InvalidLifecycleFailsBeforeAdmission(t *testing.T) {
	binding := managedProductBinding()
	tests := []struct {
		name   string
		frames []workflowContextManagedRPCFrame
	}{
		{name: "early compaction", frames: []workflowContextManagedRPCFrame{
			{Type: "turn_end"}, {Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact"},
		}},
		{name: "unsupported interactive UI", frames: []workflowContextManagedRPCFrame{
			{Type: "extension_ui_request", Method: "input"},
		}},
		{name: "post before pre", frames: append(managedProductCompletedTurns(),
			workflowContextManagedRPCFrame{Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact"},
			managedProductBridgeFrame(t, "post", WorkflowContextEventPostCompaction, binding))},
		{name: "native end before post", frames: append(managedProductCompletedTurns(),
			workflowContextManagedRPCFrame{Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact"},
			managedProductBridgeFrame(t, "pre", WorkflowContextEventPreCompaction, binding),
			workflowContextManagedRPCFrame{Type: "auto_compaction_end", Action: "snapcompact", Result: json.RawMessage(`{}`)})},
		{name: "missing bridge confirmations", frames: append(managedProductCompletedTurns(),
			workflowContextManagedRPCFrame{Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frames := make(chan []byte, len(test.frames))
			for _, frame := range test.frames {
				managedProductPushFrame(t, frames, frame)
			}
			close(frames)
			done := make(chan error, 1)
			done <- nil
			close(done)
			var output bytes.Buffer
			protocol := newWorkflowContextManagedRPCProtocol(&output, frames, done)
			err := runWorkflowContextManagedRPCProduct(context.Background(), protocol, binding, "session-1",
				[]string{"/auto go SPEC-OMP-004 --auto", "continue authoritative work"},
				func(WorkflowContextRuntimeEvent) error { return nil })
			if err == nil {
				t.Fatal("invalid product lifecycle was admitted")
			}
			if strings.Contains(output.String(), `"id":"managed-admission"`) {
				t.Fatal("invalid lifecycle reached provider admission")
			}
		})
	}
}

func TestWorkflowContextManagedRPCProduct_RequiresNativeCommandDiscovery(t *testing.T) {
	for _, test := range []struct {
		name     string
		commands []any
		wantErr  bool
	}{
		{name: "router and detail", commands: []any{"auto", map[string]any{"name": "auto-go"}}},
		{name: "missing detail", commands: []any{"auto"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			frames := make(chan []byte, 2)
			managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "ready"})
			body, err := json.Marshal(map[string]any{"type": "available_commands_update", "commands": test.commands})
			if err != nil {
				t.Fatalf("encode command discovery: %v", err)
			}
			frames <- body
			close(frames)
			done := make(chan error, 1)
			done <- nil
			close(done)
			protocol := newWorkflowContextManagedRPCProtocol(&bytes.Buffer{}, frames, done)
			err = protocol.awaitProductReady(context.Background(), "/auto go SPEC-OMP-004 --auto")
			if (err != nil) != test.wantErr {
				t.Fatalf("awaitProductReady error = %v; wantErr=%t", err, test.wantErr)
			}
		})
	}
}

func TestWorkflowContextManagedRPCProduct_ModelConfigBindsAuthNoneLoopback(t *testing.T) {
	options := managedProductTestOptions(t)
	path := filepath.Join(options.RuntimeRoot, "models.yml")
	valid := fmt.Sprintf("providers:\n  fake:\n    baseUrl: %s/v1\n    auth: none\n    models:\n      - id: product\n", options.AllowedEndpoint)
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatalf("write model config: %v", err)
	}
	if err := verifyWorkflowContextManagedRPCModelConfig(options); err != nil {
		t.Fatalf("valid model authority rejected: %v", err)
	}
	for _, body := range []string{
		strings.Replace(valid, "auth: none", "auth: api-key", 1),
		strings.Replace(valid, options.AllowedEndpoint, "http://127.0.0.1:2", 1),
		strings.Replace(valid, "id: product", "id: other", 1),
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("mutate model config: %v", err)
		}
		if err := verifyWorkflowContextManagedRPCModelConfig(options); err == nil {
			t.Fatal("mismatched model authority was accepted")
		}
	}
}

func managedProductBinding() WorkflowContextBridgeBinding {
	return WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   "sha256:" + strings.Repeat("a", 64),
		OptionsHash:   "sha256:" + strings.Repeat("b", 64),
		SessionHash:   "sha256:" + strings.Repeat("c", 64),
		NonceHash:     "sha256:" + strings.Repeat("d", 64),
	}
}

func managedProductCompletedTurns() []workflowContextManagedRPCFrame {
	success := true
	return []workflowContextManagedRPCFrame{
		{ID: "managed-product-prompt-1", Type: "response", Command: "prompt", Success: &success},
		{Type: "agent_start"}, {Type: "turn_end"}, {Type: "agent_end"},
		{ID: "managed-product-prompt-2", Type: "response", Command: "prompt", Success: &success},
		{Type: "agent_start"}, {Type: "turn_end"},
	}
}
