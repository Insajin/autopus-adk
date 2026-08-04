package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

func (protocol *pipelineOMPRPCProtocol) manualCompact(
	ctx context.Context,
	binding WorkflowContextBridgeBinding,
	expectedSession string,
	preparePrompt func() (string, error),
) error {
	preProof, _, err := protocol.validatePipelineOMPActiveTranscript(ctx, false)
	if err != nil {
		return err
	}
	protocol.nextID++
	id := fmt.Sprintf("pipeline-active-compact-%d", protocol.nextID)
	if err := protocol.process.send(pipelineOMPRPCCommand{ID: id, Type: "compact"}); err != nil {
		return err
	}
	started, preACKed, postACKed, responded, ended := false, false, false, false, false
	var nativeResult []byte
	for !ended {
		frame, err := protocol.process.next(ctx)
		if err != nil {
			return err
		}
		if frame.Type == "extension_error" {
			return errors.New("managed active OMP extension failed during manual compaction")
		}
		switch frame.Type {
		case "auto_compaction_start":
			if started || postACKed || frame.Action != "snapcompact" || frame.Reason != "manual" {
				return errors.New("managed active OMP manual compaction start is invalid")
			}
			started = true
		case "extension_ui_request":
			event, bridgeErr := validatePipelineOMPActiveBridgeFrame(frame, binding)
			if bridgeErr != nil {
				return bridgeErr
			}
			if event == WorkflowContextEventPreCompaction {
				if preACKed || postACKed || responded {
					return errors.New("managed active OMP pre-compaction checkpoint is out of order")
				}
				preACKed = true
			} else {
				if !started || !preACKed || postACKed || responded {
					return errors.New("managed active OMP post-compaction rehydration is out of order")
				}
				if _, err := preparePrompt(); err != nil {
					return err
				}
				postACKed = true
			}
			if err := protocol.confirmPipelineOMPActiveBridge(frame.ID); err != nil {
				return err
			}
		case "response":
			if frame.ID != id || frame.Command != "compact" || !frame.Success || responded ||
				!started || !preACKed || !postACKed {
				return errors.New("managed active OMP manual compaction response is invalid")
			}
			responded = true
		case "auto_compaction_end":
			if !responded || ended || !validPipelineOMPActiveNativeEnd(frame) {
				return errors.New("managed active OMP manual compaction completion is invalid")
			}
			nativeResult = append([]byte(nil), frame.Result...)
			ended = true
		case "agent_start", "turn_end", "agent_end", "prompt_result":
			return errors.New("managed active OMP provider activity crossed the compaction barrier")
		}
	}
	postProof, images, err := protocol.validatePipelineOMPActiveTranscript(ctx, true)
	if err != nil {
		return err
	}
	provenance := pipelineOMPActiveHash([]byte(preProof + "\x00" + pipelineOMPActiveHash(nativeResult) + "\x00" + postProof))
	if !validPipelineOMPActiveHash(provenance) {
		return errors.New("managed active OMP compaction provenance is invalid")
	}
	for _, digest := range images {
		protocol.safeCompactionImages[digest] = struct{}{}
	}
	state, err := protocol.readIdleState(ctx, "managed-post-compaction")
	if err != nil || state.SessionID != expectedSession || state.AutoCompactionEnabled == nil ||
		*state.AutoCompactionEnabled {
		return errors.New("managed active OMP post-compaction state is invalid")
	}
	return nil
}

func (protocol *pipelineOMPRPCProtocol) callManagedPrompt(ctx context.Context, prompt string) error {
	protocol.nextID++
	id := fmt.Sprintf("pipeline-active-prompt-%d", protocol.nextID)
	if err := protocol.process.send(pipelineOMPRPCCommand{ID: id, Type: "prompt", Message: prompt}); err != nil {
		return err
	}
	responded, started, turned, ended := false, false, false, false
	for !(responded && started && turned && ended) {
		frame, err := protocol.process.next(ctx)
		if err != nil {
			return err
		}
		if frame.Type == "extension_error" || frame.Type == "extension_ui_request" ||
			frame.Type == "auto_compaction_start" || frame.Type == "auto_compaction_end" {
			return errors.New("managed active OMP maintenance crossed the primary provider boundary")
		}
		switch frame.Type {
		case "response":
			var result struct {
				AgentInvoked *bool `json:"agentInvoked"`
			}
			if frame.ID != id || responded || started || !frame.Success || frame.Command != "prompt" ||
				json.Unmarshal(frame.Data, &result) != nil || result.AgentInvoked == nil || !*result.AgentInvoked {
				return errors.New("managed active OMP prompt was rejected")
			}
			responded = true
		case "agent_start":
			if !responded || started || turned || ended {
				return errors.New("managed active OMP primary start is out of order")
			}
			started = true
		case "turn_end":
			if !started || turned || ended {
				return errors.New("managed active OMP primary turn is out of order")
			}
			turned = true
		case "agent_end":
			if !turned || ended || frame.IsTerminal != nil && !*frame.IsTerminal {
				return errors.New("managed active OMP primary terminal event is invalid")
			}
			ended = true
		case "prompt_result":
			if frame.ID != id || frame.AgentInvoked == nil || !*frame.AgentInvoked {
				return errors.New("managed active OMP prompt did not invoke the agent")
			}
		}
	}
	return nil
}

func (protocol *pipelineOMPRPCProtocol) confirmPipelineOMPActiveBridge(id string) error {
	confirmed := true
	return protocol.process.send(pipelineOMPRPCCommand{
		ID: id, Type: "extension_ui_response", Confirmed: &confirmed,
	})
}

func validPipelineOMPActiveNativeEnd(frame pipelineOMPRPCFrame) bool {
	result := bytes.TrimSpace(frame.Result)
	return frame.Type == "auto_compaction_end" && frame.Action == "snapcompact" &&
		!frame.Aborted && !frame.Skipped && frame.ErrorMessage == "" &&
		len(result) > 0 && !bytes.Equal(result, []byte("null"))
}

func validatePipelineOMPActiveBridgeFrame(
	frame pipelineOMPRPCFrame,
	binding WorkflowContextBridgeBinding,
) (string, error) {
	if frame.Type != "extension_ui_request" || frame.Method != "confirm" || frame.ID == "" {
		return "", errors.New("managed active OMP emitted unsupported UI activity")
	}
	var message string
	var envelope workflowContextManagedBridgeEnvelope
	var exact map[string]any
	if json.Unmarshal(frame.Message, &message) != nil || json.Unmarshal([]byte(message), &envelope) != nil ||
		json.Unmarshal([]byte(message), &exact) != nil || len(exact) != 6 ||
		envelope.SchemaVersion != binding.SchemaVersion || frame.Title != "Autopus context "+envelope.Event ||
		!workflowContextSecureEqual(envelope.BindingHash, binding.BindingHash) ||
		!workflowContextSecureEqual(envelope.OptionsHash, binding.OptionsHash) ||
		!workflowContextSecureEqual(envelope.SessionHash, binding.SessionHash) ||
		!workflowContextSecureEqual(envelope.NonceHash, binding.NonceHash) ||
		(envelope.Event != WorkflowContextEventPreCompaction && envelope.Event != WorkflowContextEventPostCompaction) {
		return "", errors.New("managed active OMP bridge authority mismatch")
	}
	return envelope.Event, nil
}
