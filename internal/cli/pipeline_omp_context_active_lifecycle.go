package cli

import (
	"context"
	"errors"
	"fmt"
)

// A refused compaction is not a protocol violation. OMP declines in two shapes
// and both mean the history is already minimal:
//
//   - "Nothing to compact (session too small)" — 17.2.7 and 18.1.x
//   - "snapcompact would not reduce context locally." — 18.1.x only, measured
//     absent from 17.2.7 and present six times in 18.1.2 and 18.1.5
//
// The second one cost the first v0.50.114 attempt its cohort: the harness knew
// only the first message, so a legitimate no-op was rejected as an invalid
// response at call 6 of 42.
//
// 18.1.x also evaluates the reduction *after* firing the pre-compaction hook,
// so a refusal can arrive with the pre-ACK already acknowledged. The no-op
// proof does not depend on that ordering — it re-reads the transcript and the
// idle state and requires both unchanged — so the pre-ACK is tolerated while
// the post-ACK must still be absent.
var pipelineOMPActiveCompactionNoopMessages = []string{
	"Nothing to compact (session too small)",
	"Nothing to compact (no messages yet)",
	"snapcompact would not reduce context locally.",
}

func pipelineOMPActiveCompactionRefused(message string) bool {
	for _, known := range pipelineOMPActiveCompactionNoopMessages {
		if message == known {
			return true
		}
	}
	return false
}

func (protocol *pipelineOMPRPCProtocol) manualCompact(
	ctx context.Context,
	binding WorkflowContextBridgeBinding,
	expectedSession string,
	preparePrompt func() (string, error),
) (performed bool, runErr error) {
	protocol.probe.beginCompaction()
	defer func() { protocol.probe.finishCompaction(pipelineOMPActiveProbeAbortReason(runErr)) }()
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
		protocol.probe.observeCompactionFrame(frame)
		switch frame.Type {
		case "auto_compaction_start":
			if started || postACKed || frame.Reason != "manual" ||
				!protocol.probe.acceptsCompactionAction(frame.Action) {
				return false, errors.New("managed active OMP manual compaction start is invalid")
			}
			started = true
		case "extension_ui_request":
			event, bridgeErr := validatePipelineOMPActiveBridgeFrame(frame, binding)
			if bridgeErr != nil {
				if protocol.probe.toleratesUIRequest(frame) {
					continue
				}
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
				!postACKed && !responded &&
				pipelineOMPActiveCompactionRefused(frame.Error) {
				protocol.probe.observeCompactRefusal(frame.Error)
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
			protocol.probe.observeCompactResult(frame.Data)
			if !started {
				ended = true
			}
		case "auto_compaction_end":
			if !started || !responded || ended || !protocol.probe.validNativeEnd(frame) {
				return false, errors.New("managed active OMP manual compaction completion is invalid")
			}
			nativeResult = append(nativeResult, 0)
			nativeResult = append(nativeResult, frame.Result...)
			ended = true
		case "agent_start", "turn_end", "agent_end", "prompt_result":
			return false, errors.New("managed active OMP provider activity crossed the compaction barrier")
		}
	}
	protocol.probe.collectTranscript(true)
	postProof, images, err := protocol.validatePipelineOMPActiveTranscript(ctx, true)
	protocol.probe.collectTranscript(false)
	protocol.probe.observeCompactionImages(images)
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
			protocol.probe.observeTurnFrame(frame)
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
