package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type workflowContextManagedRPCProtocol struct {
	encoder *json.Encoder
	frames  <-chan []byte
	done    <-chan error
	usedUI  map[string]struct{}
}

type workflowContextManagedRPCFrame struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Command      string            `json:"command"`
	Method       string            `json:"method"`
	Title        string            `json:"title"`
	Message      json.RawMessage   `json:"message"`
	Reason       string            `json:"reason"`
	Action       string            `json:"action"`
	ErrorMessage string            `json:"errorMessage"`
	Success      *bool             `json:"success"`
	Aborted      bool              `json:"aborted"`
	Skipped      bool              `json:"skipped"`
	Result       json.RawMessage   `json:"result"`
	Data         json.RawMessage   `json:"data"`
	Commands     []json.RawMessage `json:"commands"`
}

type workflowContextManagedRPCState struct {
	IsStreaming           bool   `json:"isStreaming"`
	IsCompacting          bool   `json:"isCompacting"`
	SessionID             string `json:"sessionId"`
	AutoCompactionEnabled bool   `json:"autoCompactionEnabled"`
	QueuedMessageCount    int    `json:"queuedMessageCount"`
}

type workflowContextManagedBridgeEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	Event         string `json:"event"`
	BindingHash   string `json:"binding_hash"`
	OptionsHash   string `json:"options_hash"`
	SessionHash   string `json:"session_hash"`
	NonceHash     string `json:"nonce_hash"`
}

func newWorkflowContextManagedRPCProtocol(
	stdin io.Writer, frames <-chan []byte, done <-chan error,
) *workflowContextManagedRPCProtocol {
	return &workflowContextManagedRPCProtocol{
		encoder: json.NewEncoder(stdin), frames: frames, done: done, usedUI: make(map[string]struct{}),
	}
}

func (protocol *workflowContextManagedRPCProtocol) send(value any) error {
	if err := protocol.encoder.Encode(value); err != nil {
		return fmt.Errorf("write managed OMP RPC frame: %w", err)
	}
	return nil
}

func (protocol *workflowContextManagedRPCProtocol) next(
	ctx context.Context,
) (workflowContextManagedRPCFrame, error) {
	select {
	case <-ctx.Done():
		return workflowContextManagedRPCFrame{}, ctx.Err()
	case frame, ok := <-protocol.frames:
		if !ok {
			select {
			case err := <-protocol.done:
				if err != nil {
					return workflowContextManagedRPCFrame{}, err
				}
			default:
			}
			return workflowContextManagedRPCFrame{}, io.EOF
		}
		var parsed workflowContextManagedRPCFrame
		if err := json.Unmarshal(frame, &parsed); err != nil {
			return workflowContextManagedRPCFrame{}, fmt.Errorf("decode managed OMP RPC frame: %w", err)
		}
		return parsed, nil
	}
}

func (protocol *workflowContextManagedRPCProtocol) awaitReady(ctx context.Context) error {
	for {
		frame, err := protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("await managed OMP ready: %w", err)
		}
		if frame.Type == "extension_error" {
			return errors.New("managed OMP extension failed during startup")
		}
		if frame.Type == "ready" {
			return nil
		}
	}
}

func (protocol *workflowContextManagedRPCProtocol) awaitResponse(
	ctx context.Context, id string,
) (workflowContextManagedRPCFrame, error) {
	for {
		frame, err := protocol.next(ctx)
		if err != nil {
			return workflowContextManagedRPCFrame{}, fmt.Errorf("await managed OMP response %s: %w", id, err)
		}
		if frame.Type == "extension_error" {
			return workflowContextManagedRPCFrame{}, errors.New("managed OMP extension runtime error")
		}
		if frame.Type == "extension_ui_request" && frame.Method == "confirm" {
			return workflowContextManagedRPCFrame{}, errors.New("managed OMP bridge confirmation arrived outside its lifecycle stage")
		}
		if frame.Type != "response" || frame.ID != id {
			continue
		}
		if frame.Success == nil || !*frame.Success {
			return workflowContextManagedRPCFrame{}, fmt.Errorf("managed OMP command %s failed", frame.Command)
		}
		return frame, nil
	}
}

func (protocol *workflowContextManagedRPCProtocol) state(
	ctx context.Context, id string,
) (workflowContextManagedRPCState, error) {
	if err := protocol.send(map[string]any{"id": id, "type": "get_state"}); err != nil {
		return workflowContextManagedRPCState{}, err
	}
	frame, err := protocol.awaitResponse(ctx, id)
	if err != nil {
		return workflowContextManagedRPCState{}, err
	}
	var state workflowContextManagedRPCState
	if len(frame.Data) == 0 || json.Unmarshal(frame.Data, &state) != nil || state.SessionID == "" {
		return state, errors.New("managed OMP state readback is invalid")
	}
	return state, nil
}

// @AX:WARN [AUTO]: bridge authority validation has more than 15 manual cyclomatic decision points.
// @AX:REASON [AUTO]: request shape, replay protection, exact JSON envelope, constant-time hashes, title, and lifecycle event checks converge here.
func (protocol *workflowContextManagedRPCProtocol) bridgeRequest(
	frame workflowContextManagedRPCFrame,
	binding WorkflowContextBridgeBinding,
) (string, error) {
	if frame.Type != "extension_ui_request" || frame.Method != "confirm" || frame.ID == "" {
		return "", errors.New("managed OMP bridge request shape is invalid")
	}
	if _, replay := protocol.usedUI[frame.ID]; replay {
		return "", errors.New("managed OMP bridge request was replayed")
	}
	var envelope workflowContextManagedBridgeEnvelope
	var exact map[string]any
	var message string
	if json.Unmarshal(frame.Message, &message) != nil || json.Unmarshal([]byte(message), &envelope) != nil ||
		json.Unmarshal([]byte(message), &exact) != nil || len(exact) != 6 {
		return "", errors.New("managed OMP bridge envelope is invalid")
	}
	if envelope.SchemaVersion != binding.SchemaVersion ||
		!workflowContextSecureEqual(envelope.BindingHash, binding.BindingHash) ||
		!workflowContextSecureEqual(envelope.OptionsHash, binding.OptionsHash) ||
		!workflowContextSecureEqual(envelope.SessionHash, binding.SessionHash) ||
		!workflowContextSecureEqual(envelope.NonceHash, binding.NonceHash) ||
		(frame.Title != "Autopus context "+envelope.Event) ||
		(envelope.Event != WorkflowContextEventPreCompaction && envelope.Event != WorkflowContextEventPostCompaction) {
		return "", errors.New("managed OMP bridge authority mismatch")
	}
	protocol.usedUI[frame.ID] = struct{}{}
	return envelope.Event, nil
}

func (protocol *workflowContextManagedRPCProtocol) confirm(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("managed OMP bridge response id is missing")
	}
	return protocol.send(map[string]any{"type": "extension_ui_response", "id": id, "confirmed": true})
}

func workflowContextManagedProductNotification(method string) bool {
	switch method {
	case "notify", "setStatus", "setWidget", "setTitle", "set_editor_text":
		return true
	default:
		return false
	}
}

func sendWorkflowContextManagedProductPrompt(
	protocol *workflowContextManagedRPCProtocol, index int, prompt string,
) error {
	return protocol.send(map[string]any{
		"id": fmt.Sprintf("managed-product-prompt-%d", index+1), "type": "prompt", "message": prompt,
	})
}

// @AX:WARN [AUTO]: provider-boundary verification has cyclomatic complexity 15.
// @AX:REASON [AUTO]: prompt acceptance and agent start, turn end, and agent end must all be observed in order before admission.
func (protocol *workflowContextManagedRPCProtocol) awaitProviderBoundary(
	ctx context.Context, id string,
) error {
	accepted, started, turned, ended := false, false, false, false
	for !(accepted && started && turned && ended) {
		frame, err := protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("await managed OMP provider boundary: %w", err)
		}
		if frame.Type == "extension_error" {
			return errors.New("managed OMP extension failed during provider admission")
		}
		if frame.Type == "response" && frame.ID == id {
			if frame.Success == nil || !*frame.Success || frame.Command != "prompt" {
				return errors.New("managed OMP admission prompt was rejected")
			}
			accepted = true
		}
		switch frame.Type {
		case "agent_start":
			started = true
		case "turn_end":
			turned = started
		case "agent_end":
			ended = turned
		}
	}
	return nil
}

func (protocol *workflowContextManagedRPCProtocol) awaitNativeCompactionEnd(ctx context.Context) error {
	for {
		frame, err := protocol.next(ctx)
		if err != nil {
			return fmt.Errorf("await managed OMP native compaction end: %w", err)
		}
		if frame.Type == "extension_error" || frame.Type == "extension_ui_request" {
			return errors.New("managed OMP emitted unexpected activity before native completion")
		}
		if frame.Type != "auto_compaction_end" {
			continue
		}
		missingResult := len(frame.Result) == 0 || string(frame.Result) == "null"
		if frame.Action != "snapcompact" || frame.Aborted || frame.Skipped ||
			frame.ErrorMessage != "" || missingResult {
			return errors.New("managed OMP native compaction end is invalid")
		}
		return nil
	}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: the RPC frame reader is bounded by both process-pipe closure and caller cancellation.
// @AX:REASON [AUTO]: managed shutdown must unblock scanner reads and a full frame channel without leaving a producer goroutine behind.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: 128 queued frames and a 2 MiB scanner ceiling bound RPC buffering and individual frame size.
func workflowContextManagedRPCFrames(ctx context.Context, reader io.Reader) (<-chan []byte, <-chan error) {
	frames, done := make(chan []byte, 128), make(chan error, 1)
	go func() {
		defer close(frames)
		defer close(done)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 2<<20)
		for scanner.Scan() {
			frame := append([]byte(nil), scanner.Bytes()...)
			select {
			case frames <- frame:
			case <-ctx.Done():
				done <- ctx.Err()
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = ctx.Err()
		}
		done <- err
	}()
	return frames, done
}
