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
		response bool
		wantErr  bool
	}{
		{name: "broadcast without response", commands: []any{"auto", map[string]any{"name": "auto-go"}}, wantErr: true},
		{name: "router and detail response", commands: []any{map[string]any{"name": "auto"}, map[string]any{"name": "auto-go"}}, response: true},
		{name: "missing detail", commands: []any{"auto"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			frames := make(chan []byte, 2)
			managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "ready"})
			body, err := json.Marshal(map[string]any{"commands": test.commands})
			if err != nil {
				t.Fatalf("encode command discovery: %v", err)
			}
			if test.response {
				success := true
				managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
					ID: "managed-product-commands", Type: "response", Command: "get_available_commands",
					Success: &success, Data: body,
				})
			} else {
				broadcast, marshalErr := json.Marshal(map[string]any{
					"type": "available_commands_update", "commands": test.commands,
				})
				if marshalErr != nil {
					t.Fatalf("encode command broadcast: %v", marshalErr)
				}
				frames <- broadcast
			}
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

func TestWorkflowContextManagedRPCProduct_ProviderBoundaryRejectsStaleLifecycle(t *testing.T) {
	success := true
	for _, frames := range [][]workflowContextManagedRPCFrame{
		{{Type: "agent_start"}, {ID: "admission", Type: "response", Command: "prompt", Success: &success}},
		{{ID: "admission", Type: "response", Command: "prompt", Success: &success}, {Type: "turn_end"}},
		{{ID: "admission", Type: "response", Command: "prompt", Success: &success}, {Type: "agent_start"}, {Type: "agent_start"}},
	} {
		input := make(chan []byte, len(frames))
		for _, frame := range frames {
			managedProductPushFrame(t, input, frame)
		}
		close(input)
		protocol := newWorkflowContextManagedRPCProtocol(&bytes.Buffer{}, input, make(chan error))
		if err := protocol.awaitProviderBoundary(context.Background(), "admission"); err == nil {
			t.Fatal("stale or out-of-order provider lifecycle was admitted")
		}
	}
}

func TestWorkflowContextManagedRPCProduct_ModelConfigBindsCredentialAuthority(t *testing.T) {
	options := managedProductTestOptions(t)
	path := filepath.Join(options.RuntimeRoot, "models.yml")
	valid := fmt.Sprintf("providers:\n  fake:\n    baseUrl: %s/v1\n    apiKey: AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN\n    authHeader: true\n    api: openai-completions\n    models:\n      - id: product\n", options.AllowedEndpoint)
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatalf("write model config: %v", err)
	}
	if err := verifyWorkflowContextManagedRPCModelConfig(options); err != nil {
		t.Fatalf("valid model authority rejected: %v", err)
	}
	for _, body := range []string{
		strings.Replace(valid, "authHeader: true", "authHeader: false", 1),
		strings.Replace(valid, "apiKey: AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN", "apiKey: OTHER_TOKEN", 1),
		strings.Replace(valid, options.AllowedEndpoint, "http://127.0.0.1:2", 1),
		strings.Replace(valid, "id: product", "id: other", 1),
		strings.Replace(valid, "api: openai-completions", "api: openai-responses", 1),
		strings.Replace(valid, "  fake:", "  other:", 1),
		strings.Replace(valid, "    models:", "    headers:\n      Authorization: leaked\n    models:", 1),
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("mutate model config: %v", err)
		}
		if err := verifyWorkflowContextManagedRPCModelConfig(options); err == nil {
			t.Fatal("mismatched model authority was accepted")
		}
	}
}

func TestWorkflowContextManagedRPCProduct_CompactionCycleBoundsAndDefault(t *testing.T) {
	for _, test := range []struct {
		name         string
		cycles, want int
		wantErr      bool
	}{
		{name: "zero defaults to one", cycles: 0, want: 1},
		{name: "one is accepted", cycles: 1, want: 1},
		{name: "upper bound is accepted", cycles: 8, want: 8},
		{name: "above upper bound is rejected", cycles: 9, wantErr: true},
		{name: "negative count is rejected", cycles: -1, wantErr: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options := managedProductTestOptions(t)
			options.CompactionCycles = test.cycles
			normalized, err := validateWorkflowContextManagedRPCOptions(options)
			if test.wantErr {
				if err == nil {
					t.Fatalf("compaction cycles %d were accepted as %d", test.cycles, normalized.CompactionCycles)
				}
				return
			}
			if err != nil || normalized.CompactionCycles != test.want {
				t.Fatalf("compaction cycles %d normalized to %d, err=%v; want %d", test.cycles, normalized.CompactionCycles, err, test.want)
			}
		})
	}
}

func TestWorkflowContextManagedRPCProduct_CredentialEnvironmentFailsClosed(t *testing.T) {
	const key = "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN"
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowContextManagedRPCOptions)
	}{
		{name: "duplicate", mutate: func(options *WorkflowContextManagedRPCOptions) {
			options.Environment = append(options.Environment, key+"=duplicate")
		}},
		{name: "missing", mutate: func(options *WorkflowContextManagedRPCOptions) {
			options.Environment = options.Environment[:2]
		}},
		{name: "empty", mutate: func(options *WorkflowContextManagedRPCOptions) {
			options.Environment[2] = key + "="
		}},
		{name: "ambient key", mutate: func(options *WorkflowContextManagedRPCOptions) {
			path := filepath.Join(options.RuntimeRoot, "models.yml")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(body), "apiKey: "+key, "apiKey: PATH", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "process control key", mutate: func(options *WorkflowContextManagedRPCOptions) {
			path := filepath.Join(options.RuntimeRoot, "models.yml")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(body), "apiKey: "+key, "apiKey: NODE_OPTIONS", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			options.Environment = append(options.Environment, "NODE_OPTIONS=--require=malicious")
		}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options := managedProductTestOptions(t)
			test.mutate(&options)
			if _, err := validateWorkflowContextManagedRPCOptions(options); err == nil {
				t.Fatal("invalid credential authority was accepted")
			}
		})
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
