package cli

// Stateful checkpoint exchange for the AC-013 fixture family. The eager
// compaction model writes every checkpoint, compact response and completion
// frame before it reads the next command, so it cannot represent a checkpoint
// that is still waiting for its acknowledgement and no transcript proof query
// can ever run inside the compaction barrier. This model releases each frame
// only when the confirmation that unblocks it arrives, answers page queries
// while a repeated pre-checkpoint waits, and delays the transcript and usage
// mutation until the transaction actually completes. Every other fixture model
// keeps the eager behaviour.

import (
	"encoding/json"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

// handle owns the commands a checkpoint transaction touches. complete applies
// the compaction the fixture session performed; it runs once, when the
// post-checkpoint has been acknowledged and the transaction is settled.
func (fixture *pipelineOMPActiveCheckpointFixture) handle(
	output *json.Encoder,
	command pipelineOMPRPCCommand,
	transcript []json.RawMessage,
	complete func(),
) {
	switch command.Type {
	case "get_messages_page":
		fixture.page(output, command, transcript)
	case "compact":
		fixture.begin(output, command)
	case "extension_ui_response":
		// A checkpoint acknowledgement is not an RPC call and gets no response
		// frame; answering one would put an unrelated response inside the
		// compaction barrier.
		fixture.advance(output, complete)
	case "get_state":
		// One-shot: only the idle read that proves a declined compaction sees
		// a session that is still working.
		fixture.stage = pipelineOMPActiveCheckpointIdle
		writePipelineOMPActiveResponse(output, command, map[string]any{
			"sessionId": pipelineOMPActiveCheckpointSessionID, "isStreaming": false,
			"isCompacting": true, "messageCount": 0, "queuedMessageCount": 0,
			"autoCompactionEnabled": false,
		})
	}
}

func (fixture *pipelineOMPActiveCheckpointFixture) begin(
	output *json.Encoder,
	command pipelineOMPRPCCommand,
) {
	fixture.sequence++
	fixture.compactID = command.ID
	// A declined compaction never starts a native method.
	if !fixture.declines() {
		_ = output.Encode(map[string]any{
			"type": "auto_compaction_start", "reason": "manual", "action": ompprobe.MethodSnapcompact,
		})
	}
	if fixture.variant == pipelineOMPActiveCheckpointTurnBefore {
		// An orphan primary turn frame inside the transaction and ahead of
		// every checkpoint. No proof query exists yet, so the compaction loop
		// itself has to refuse it and acknowledge nothing.
		_ = output.Encode(map[string]any{"type": "turn_start"})
	}
	if fixture.variant == pipelineOMPActiveCheckpointPostFirst {
		fixture.stage = pipelineOMPActiveCheckpointAwaitPost
		_ = output.Encode(fixture.checkpoint("post", WorkflowContextEventPostCompaction, true))
		return
	}
	fixture.stage = pipelineOMPActiveCheckpointFirstPre
	_ = output.Encode(fixture.checkpoint("pre-1", WorkflowContextEventPreCompaction, true))
}

// advance releases the frame the acknowledgement unblocks. The first pre is
// followed by a second one, the second pre by the post-checkpoint or by a
// decline, and the post-checkpoint by the correlated compact response and
// native completion.
func (fixture *pipelineOMPActiveCheckpointFixture) advance(output *json.Encoder, complete func()) {
	switch fixture.stage {
	case pipelineOMPActiveCheckpointFirstPre:
		fixture.stage = pipelineOMPActiveCheckpointSecondPre
		fixture.secondPre(output)
	case pipelineOMPActiveCheckpointSecondPre:
		if fixture.declines() {
			fixture.stage = pipelineOMPActiveCheckpointRefused
			fixture.decline(output)
			return
		}
		fixture.stage = pipelineOMPActiveCheckpointAwaitPost
		if fixture.variant == pipelineOMPActiveCheckpointThird {
			_ = output.Encode(fixture.checkpoint("pre-3", WorkflowContextEventPreCompaction, true))
			return
		}
		_ = output.Encode(fixture.checkpoint("post", WorkflowContextEventPostCompaction, true))
	case pipelineOMPActiveCheckpointAwaitPost:
		fixture.settle(output, complete)
	}
}

func (fixture *pipelineOMPActiveCheckpointFixture) secondPre(output *json.Encoder) {
	switch fixture.variant {
	case pipelineOMPActiveCheckpointPostDup, pipelineOMPActiveCheckpointPreAfterPost:
		fixture.stage = pipelineOMPActiveCheckpointAwaitPost
		_ = output.Encode(fixture.checkpoint("post", WorkflowContextEventPostCompaction, true))
	case pipelineOMPActiveCheckpointReplay:
		// The same request ID a second time, authenticated exactly as the first.
		_ = output.Encode(fixture.checkpoint("pre-1", WorkflowContextEventPreCompaction, true))
	case pipelineOMPActiveCheckpointForged:
		_ = output.Encode(fixture.checkpoint("pre-2", WorkflowContextEventPreCompaction, false))
	default:
		_ = output.Encode(fixture.checkpoint("pre-2", WorkflowContextEventPreCompaction, true))
	}
}

func (fixture *pipelineOMPActiveCheckpointFixture) settle(output *json.Encoder, complete func()) {
	switch fixture.variant {
	case pipelineOMPActiveCheckpointPostDup:
		_ = output.Encode(fixture.checkpoint("post-2", WorkflowContextEventPostCompaction, true))
		return
	case pipelineOMPActiveCheckpointPreAfterPost:
		_ = output.Encode(fixture.checkpoint("pre-late", WorkflowContextEventPreCompaction, true))
		return
	}
	fixture.stage = pipelineOMPActiveCheckpointIdle
	complete()
	fixture.respond(output)
	_ = output.Encode(map[string]any{
		"type": "auto_compaction_end", "reason": "manual", "action": ompprobe.MethodSnapcompact,
		"result": map[string]any{"summary": "safe compacted context"},
	})
}

// respond is the correlated compact success. The early-response variant emits
// exactly this frame one step too soon, so the two must stay identical.
func (fixture *pipelineOMPActiveCheckpointFixture) respond(output *json.Encoder) {
	_ = output.Encode(map[string]any{
		"id": fixture.compactID, "type": "response", "command": "compact",
		"success": true, "data": map[string]any{"summary": "safe compacted context"},
	})
}

// decline answers the compact with the reduction refusal omp/18.1.x reports
// after it has already fired its pre-compaction hooks.
func (fixture *pipelineOMPActiveCheckpointFixture) decline(output *json.Encoder) {
	_ = output.Encode(map[string]any{
		"id": fixture.compactID, "type": "response", "command": "compact",
		"success": false, "error": "Nothing to compact (session too small)",
	})
}

// page answers a transcript query. While the second pre-checkpoint waits, and
// again while a decline is being proved, the query runs inside the compaction
// barrier: that is where the moved-transcript and frame-injection variants
// act.
func (fixture *pipelineOMPActiveCheckpointFixture) page(
	output *json.Encoder,
	command pipelineOMPRPCCommand,
	transcript []json.RawMessage,
) {
	switch fixture.stage {
	case pipelineOMPActiveCheckpointSecondPre:
		fixture.inject(output, command)
		if fixture.variant == pipelineOMPActiveCheckpointMutate {
			transcript = pipelineOMPActiveCheckpointMoved(transcript)
		}
	case pipelineOMPActiveCheckpointRefused:
		if fixture.variant == pipelineOMPActiveCheckpointRefuseMutate {
			transcript = pipelineOMPActiveCheckpointMoved(transcript)
		}
	}
	writePipelineOMPActiveResponse(output, command, pipelineOMPActiveMessagesPage{
		Messages: transcript, TotalMessages: len(transcript), NextCursor: nil,
	})
}

// inject pushes one frame in front of the page response the pending checkpoint
// is waiting for. None of them is the page answer, so all of them must fail
// the transaction: the notice too, even though the ordinary compaction loop
// still records and skips one.
func (fixture *pipelineOMPActiveCheckpointFixture) inject(
	output *json.Encoder,
	command pipelineOMPRPCCommand,
) {
	if frame, injected := fixture.providerFrame(); injected {
		_ = output.Encode(frame)
		return
	}
	switch fixture.variant {
	case pipelineOMPActiveCheckpointNotice:
		_ = output.Encode(map[string]any{
			"id": fixture.checkpointID("notice"), "type": "extension_ui_request",
			"method": "notify", "title": "OMP notice", "message": "context compaction started",
		})
	case pipelineOMPActiveCheckpointExtra:
		_ = output.Encode(fixture.checkpoint("pre-extra", WorkflowContextEventPreCompaction, true))
	case pipelineOMPActiveCheckpointPostDuringProof:
		_ = output.Encode(fixture.checkpoint("post-proof", WorkflowContextEventPostCompaction, true))
	case pipelineOMPActiveCheckpointEarly:
		fixture.respond(output)
	case pipelineOMPActiveCheckpointStray:
		_ = output.Encode(map[string]any{
			"id": fixture.checkpointID("stray"), "type": "response", "command": "get_state",
			"success": true, "data": map[string]any{},
		})
	case pipelineOMPActiveCheckpointWrongCommand:
		// The query's own ID answering a different command.
		_ = output.Encode(map[string]any{
			"id": command.ID, "type": "response", "command": "get_state",
			"success": true, "data": map[string]any{},
		})
	case pipelineOMPActiveCheckpointWrongID:
		// The right command under an ID this query never used.
		_ = output.Encode(map[string]any{
			"id": command.ID + "-other", "type": "response", "command": "get_messages_page",
			"success": true, "data": map[string]any{},
		})
	case pipelineOMPActiveCheckpointExtError:
		_ = output.Encode(map[string]any{
			"type": "extension_error", "error": "checkpoint extension failed",
		})
	}
}

// providerFrame is the primary-turn lifecycle a compaction barrier must never
// see, whichever frame of it arrives first.
func (fixture *pipelineOMPActiveCheckpointFixture) providerFrame() (map[string]any, bool) {
	switch fixture.variant {
	case pipelineOMPActiveCheckpointProvider:
		return map[string]any{"type": "agent_start"}, true
	case pipelineOMPActiveCheckpointTurnStart:
		return map[string]any{"type": "turn_start"}, true
	case pipelineOMPActiveCheckpointTurnEnd:
		return map[string]any{"type": "turn_end"}, true
	case pipelineOMPActiveCheckpointAgentEnd:
		return map[string]any{"type": "agent_end", "isTerminal": true}, true
	case pipelineOMPActiveCheckpointPromptResult:
		return map[string]any{
			"id": fixture.checkpointID("prompt"), "type": "prompt_result", "agentInvoked": true,
		}, true
	}
	return nil, false
}

func (fixture *pipelineOMPActiveCheckpointFixture) checkpointID(suffix string) string {
	return fmt.Sprintf("active-checkpoint-%d-%s", fixture.sequence, suffix)
}

// checkpoint builds an authenticated bridge confirmation. authentic=false
// keeps the shape and breaks only the nonce, so the frame fails on authority
// rather than on structure.
func (fixture *pipelineOMPActiveCheckpointFixture) checkpoint(
	suffix string,
	event string,
	authentic bool,
) map[string]any {
	binding := pipelineOMPActiveFixtureBinding()
	if !authentic {
		// A well-formed hash the bridge never issued. Deriving it instead of
		// blanking the field keeps the envelope shape valid, so authority is
		// the only thing that can reject the frame.
		binding.NonceHash = workflowContextRuntimeHash("checkpoint-forged-nonce")
	}
	envelope, _ := json.Marshal(workflowContextManagedBridgeEnvelope{
		SchemaVersion: binding.SchemaVersion, Event: event, BindingHash: binding.BindingHash,
		OptionsHash: binding.OptionsHash, SessionHash: binding.SessionHash, NonceHash: binding.NonceHash,
	})
	message, _ := json.Marshal(string(envelope))
	return map[string]any{
		"id": fixture.checkpointID(suffix), "type": "extension_ui_request", "method": "confirm",
		"title": "Autopus context " + event, "message": json.RawMessage(message),
	}
}

// pipelineOMPActiveCheckpointModel is the model id that selects one exchange.
func pipelineOMPActiveCheckpointModel(variant string) string {
	return pipelineOMPActiveCheckpointModelPrefix + variant
}
