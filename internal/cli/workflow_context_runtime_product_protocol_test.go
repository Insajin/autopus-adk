package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestWorkflowContextManagedRPCProduct_ProtocolUsesExactPromptsAndACKLifecycle(t *testing.T) {
	binding := WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   "sha256:" + strings.Repeat("a", 64),
		OptionsHash:   "sha256:" + strings.Repeat("b", 64),
		SessionHash:   "sha256:" + strings.Repeat("c", 64),
		NonceHash:     "sha256:" + strings.Repeat("d", 64),
	}
	frames := make(chan []byte, 16)
	success := true
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
		Type: "extension_ui_request", Method: "setWidget",
	})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
		ID: "managed-product-prompt-1", Type: "response", Command: "prompt", Success: &success,
	})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "agent_start"})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "turn_end"})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "agent_end"})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
		ID: "managed-product-prompt-2", Type: "response", Command: "prompt", Success: &success,
	})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "agent_start"})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{Type: "turn_end"})
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
		Type: "auto_compaction_start", Reason: "threshold", Action: "snapcompact",
	})
	managedProductPushFrame(t, frames, managedProductBridgeFrame(t, "pre-ack", WorkflowContextEventPreCompaction, binding))
	managedProductPushFrame(t, frames, managedProductBridgeFrame(t, "post-ack", WorkflowContextEventPostCompaction, binding))
	managedProductPushFrame(t, frames, workflowContextManagedRPCFrame{
		Type: "auto_compaction_end", Action: "snapcompact", Result: json.RawMessage(`{}`),
	})
	close(frames)
	done := make(chan error, 1)
	done <- nil
	close(done)

	var output bytes.Buffer
	protocol := newWorkflowContextManagedRPCProtocol(&output, frames, done)
	prompts := []string{
		"/auto go SPEC-OMP-004 --auto",
		"Use the assembled authoritative request exactly; preserve every ordered document body.",
	}
	var events []string
	err := runWorkflowContextManagedRPCProduct(
		context.Background(), protocol, binding, "product-session", prompts,
		func(event WorkflowContextRuntimeEvent) error {
			events = append(events, event.Kind)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("run product protocol: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("product lifecycle left %d scripted frames unread; want native end for Dispatch", len(frames))
	}

	var sentPrompts []string
	confirmed := map[string]bool{}
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		var frame map[string]any
		if err := decoder.Decode(&frame); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode sent RPC frame: %v", err)
		}
		switch frame["type"] {
		case "prompt":
			message, ok := frame["message"].(string)
			if !ok {
				t.Fatalf("prompt message is not text: %#v", frame)
			}
			sentPrompts = append(sentPrompts, message)
		case "extension_ui_response":
			id, _ := frame["id"].(string)
			confirmed[id] = frame["confirmed"] == true
		}
	}
	if len(sentPrompts) != len(prompts) {
		t.Fatalf("sent prompts = %q; want %q", sentPrompts, prompts)
	}
	for index := range prompts {
		if sentPrompts[index] != prompts[index] {
			t.Fatalf("prompt %d = %q; want exact %q", index, sentPrompts[index], prompts[index])
		}
	}
	joined := strings.Join(sentPrompts, "\n")
	for _, synthetic := range []string{"bounded seed context", "bounded threshold context"} {
		if strings.Contains(joined, synthetic) {
			t.Fatalf("product prompts contain synthetic canary text %q", synthetic)
		}
	}
	if !confirmed["pre-ack"] || confirmed["post-ack"] {
		t.Fatalf("ACK confirmations = %#v; want pre only and post reserved for Dispatch", confirmed)
	}
	wantEvents := []string{
		WorkflowContextEventPreCompaction,
		WorkflowContextEventCompacted,
		WorkflowContextEventPostCompaction,
	}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("events = %q; want %q", events, wantEvents)
	}
}
