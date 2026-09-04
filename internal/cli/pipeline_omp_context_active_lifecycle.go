package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const pipelineOMPActiveCompactionNoopMessage = "Nothing to compact (session too small)"

func (protocol *pipelineOMPRPCProtocol) manualCompact(
	ctx context.Context,
	binding WorkflowContextBridgeBinding,
	expectedSession string,
	preparePrompt func() (string, error),
) (bool, error) {
	preProof, _, err := protocol.validatePipelineOMPActiveTranscript(ctx, false)
	if err != nil {
		return false, err
	}
	protocol.nextID++
	id := fmt.Sprintf("pipeline-active-compact-%d", protocol.nextID)
	if err := protocol.process.send(pipelineOMPRPCCommand{ID: id, Type: "compact"}); err != nil {
		return false, err
	}
	started, preACKed, postACKed, responded, ended := false, false, false, false, false
	var nativeResult []byte
	for !ended {
		frame, err := protocol.process.next(ctx)
		if err != nil {
			return false, err
		}
		if frame.Type == "extension_error" {
			return false, errors.New("managed active OMP extension failed during manual compaction")
		}
		switch frame.Type {
		case "auto_compaction_start":
			if started || postACKed || frame.Action != "snapcompact" || frame.Reason != "manual" {
				return false, errors.New("managed active OMP manual compaction start is invalid")
			}
			started = true
		case "extension_ui_request":
			event, bridgeErr := validatePipelineOMPActiveBridgeFrame(frame, binding)
			if bridgeErr != nil {
				return false, bridgeErr
			}
			if event == WorkflowContextEventPreCompaction {
				if preACKed || postACKed || responded {
					return false, errors.New("managed active OMP pre-compaction checkpoint is out of order")
				}
				preACKed = true
			} else {
				if !preACKed || postACKed || responded {
					return false, errors.New("managed active OMP post-compaction rehydration is out of order")
				}
				if _, err := preparePrompt(); err != nil {
					return false, err
				}
				postACKed = true
			}
			if err := protocol.confirmPipelineOMPActiveBridge(frame.ID); err != nil {
				return false, err
			}
		case "response":
			if frame.ID == id && frame.Command == "compact" && !frame.Success && !started &&
				!preACKed && !postACKed && !responded &&
				frame.Error == pipelineOMPActiveCompactionNoopMessage {
				postProof, _, proofErr := protocol.validatePipelineOMPActiveTranscript(ctx, false)
				state, stateErr := protocol.readIdleState(ctx, "managed-compaction-noop")
				if proofErr != nil || postProof != preProof || stateErr != nil ||
					state.SessionID != expectedSession || state.AutoCompactionEnabled == nil ||
					*state.AutoCompactionEnabled {
					return false, errors.New("managed active OMP no-op compaction proof is invalid")
				}
				return false, nil
			}
			if frame.ID != id || frame.Command != "compact" || !frame.Success || responded ||
				!preACKed || !postACKed || !validPipelineOMPActiveManualResult(frame.Data) {
				// The bare form of this error cost a whole cohort run to learn
				// nothing: omp/18.1.5 failed here at call 6 of 42 and the message
				// named no field. Reaching a real compaction needs cohort-scale
				// history, so one run has to yield the answer. The error text is
				// body-free — no transcript, no summary, only which gate failed.
				return false, fmt.Errorf(
					"managed active OMP manual compaction response is invalid: "+
						"id_match=%t command=%q success=%t already_responded=%t "+
						"pre_acked=%t post_acked=%t summary_valid=%t error=%q",
					frame.ID == id, frame.Command, frame.Success, responded,
					preACKed, postACKed, validPipelineOMPActiveManualResult(frame.Data),
					frame.Error)
			}
			responded = true
			nativeResult = frame.Data
			if !started {
				ended = true
			}
		case "auto_compaction_end":
			if !started || !responded || ended || !validPipelineOMPActiveNativeEnd(frame) {
				return false, errors.New("managed active OMP manual compaction completion is invalid")
			}
			nativeResult = append(nativeResult, 0)
			nativeResult = append(nativeResult, frame.Result...)
			ended = true
		case "agent_start", "turn_end", "agent_end", "prompt_result":
			return false, errors.New("managed active OMP provider activity crossed the compaction barrier")
		}
	}
	postProof, images, err := protocol.validatePipelineOMPActiveTranscript(ctx, true)
	if err != nil {
		return false, err
	}
	provenance := pipelineOMPActiveHash([]byte(preProof + "\x00" + pipelineOMPActiveHash(nativeResult) + "\x00" + postProof))
	if !validPipelineOMPActiveHash(provenance) {
		return false, errors.New("managed active OMP compaction provenance is invalid")
	}
	for _, digest := range images {
		protocol.safeCompactionImages[digest] = struct{}{}
	}
	state, err := protocol.readIdleState(ctx, "managed-post-compaction")
	if err != nil || state.SessionID != expectedSession || state.AutoCompactionEnabled == nil ||
		*state.AutoCompactionEnabled {
		return false, errors.New("managed active OMP post-compaction state is invalid")
	}
	return true, nil
}

// @AX:WARN [AUTO]: managed prompt lifecycle validation contains 11 if branches.
// @AX:REASON [AUTO]: response correlation, safe widget filtering, provider start/turn/end order, and prompt-result proof must fail closed together.
func (protocol *pipelineOMPRPCProtocol) callManagedPrompt(ctx context.Context, prompt string) error {
	protocol.nextID++
	id := fmt.Sprintf("pipeline-active-prompt-%d", protocol.nextID)
	if err := protocol.process.send(pipelineOMPRPCCommand{ID: id, Type: "prompt", Message: prompt}); err != nil {
		return err
	}
	responded, resultSeen := false, false
	started, inTurn, ended := false, false, false
	turns := 0
	// OMP starts session.prompt before the RPC dispatcher writes its success response,
	// so agent_start and turn_start may race ahead of that response. One prompt can
	// also drive several agent cycles: a retry re-enters through agentLoopContinue and
	// emits another agent_start, while the wire-level agent_end stays held until the
	// prompt settles. Completion is the terminal agent_end (isTerminal != false), not
	// the first start/turn cycle.
	for !(responded && ended) {
		frame, err := protocol.process.next(ctx)
		if err != nil {
			return err
		}
		if frame.Type == "extension_ui_request" {
			if frame.Method == "setWidget" && frame.ID != "" && started && !ended {
				continue
			}
			return errors.New("managed active OMP maintenance crossed the primary provider boundary")
		}
		if frame.Type == "extension_error" ||
			frame.Type == "auto_compaction_start" || frame.Type == "auto_compaction_end" {
			return errors.New("managed active OMP maintenance crossed the primary provider boundary")
		}
		switch frame.Type {
		case "response":
			if frame.ID != id || responded || ended || !frame.Success ||
				frame.Command != "prompt" || !validPipelineOMPActivePromptResponseData(frame.Data) {
				return errors.New("managed active OMP prompt was rejected")
			}
			responded = true
		case "agent_start":
			if inTurn || ended {
				return errors.New("managed active OMP primary start is out of order")
			}
			started = true
		case "turn_start":
			if !started || inTurn || ended {
				return errors.New("managed active OMP primary turn start is out of order")
			}
			inTurn = true
		case "turn_end":
			if !inTurn || ended {
				return errors.New("managed active OMP primary turn is out of order")
			}
			inTurn, turns = false, turns+1
		case "agent_end":
			if !started || inTurn || turns == 0 || ended {
				return errors.New("managed active OMP primary terminal event is invalid")
			}
			ended = frame.IsTerminal == nil || *frame.IsTerminal
		case "prompt_result":
			if resultSeen || frame.ID != id || frame.AgentInvoked == nil || !*frame.AgentInvoked {
				return errors.New("managed active OMP prompt did not invoke the agent")
			}
			resultSeen = true
		}
	}
	return nil
}

func validPipelineOMPActivePromptResponseData(data json.RawMessage) bool {
	body := bytes.TrimSpace(data)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return true
	}
	if rejectDuplicatePipelineOMPJSON(body) != nil {
		return false
	}
	var exact map[string]json.RawMessage
	if json.Unmarshal(body, &exact) != nil || len(exact) != 1 {
		return false
	}
	var invoked bool
	value, ok := exact["agentInvoked"]
	return ok && json.Unmarshal(value, &invoked) == nil && invoked
}

func (protocol *pipelineOMPRPCProtocol) confirmPipelineOMPActiveBridge(id string) error {
	confirmed := true
	return protocol.process.send(pipelineOMPRPCCommand{
		ID: id, Type: "extension_ui_response", Confirmed: &confirmed,
	})
}

func validPipelineOMPActiveManualResult(data json.RawMessage) bool {
	var result struct {
		Summary string `json:"summary"`
	}
	return json.Unmarshal(data, &result) == nil && strings.TrimSpace(result.Summary) != ""
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
