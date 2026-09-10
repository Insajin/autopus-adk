package cli

// The repeated pre-checkpoint proof query of REQ-CHECKPOINT-001 runs while the
// compaction barrier is held, so it cannot use the correlating reader: that one
// walks past every frame it did not ask for, which here would let a provider
// event, a further checkpoint or an early compact response pass unseen and then
// accept the page answer as if the barrier had held. This reader accepts
// exactly the successful page response of its own query and fails the
// transaction on anything else, while the shared transcript validator keeps
// owning pagination, the structural walk and the proof hash.

import (
	"context"
	"encoding/json"
	"errors"
)

// proveRepeatedPreCheckpoint re-reads the transcript before the second pre is
// acknowledged and requires it to equal the proof taken when the transaction
// opened. A changed transcript means the history was already rewritten, so the
// repeat is not the same transaction and nothing is acknowledged. The query
// runs on the transaction's own context, so a retry never resets the deadline.
func (protocol *pipelineOMPRPCProtocol) proveRepeatedPreCheckpoint(
	ctx context.Context,
	binding WorkflowContextBridgeBinding,
	checkpoints *pipelineOMPActiveCheckpoints,
	preProof string,
) error {
	barrier := pipelineOMPActiveProofBarrier{protocol: protocol, binding: binding, checkpoints: checkpoints}
	previousReader := protocol.pageReader
	protocol.pageReader = barrier.read
	defer func() { protocol.pageReader = previousReader }()
	proof, _, err := protocol.validatePipelineOMPActiveTranscript(ctx, false)
	if barrier.failure != nil {
		// The shared validator flattens every reader failure into one body-free
		// transcript error. The barrier's own reason names which frame broke
		// the query, so it wins.
		return barrier.failure
	}
	if err != nil {
		return err
	}
	if proof != preProof {
		return errors.New("managed active OMP repeated pre-compaction proof changed")
	}
	return nil
}

// pipelineOMPActiveProofBarrier answers the pages of one repeated-pre proof
// query without releasing the compaction barrier.
type pipelineOMPActiveProofBarrier struct {
	protocol    *pipelineOMPRPCProtocol
	binding     WorkflowContextBridgeBinding
	checkpoints *pipelineOMPActiveCheckpoints
	failure     error
}

func (barrier *pipelineOMPActiveProofBarrier) read(
	ctx context.Context,
	command pipelineOMPRPCCommand,
) (json.RawMessage, error) {
	protocol := barrier.protocol
	command.ID = protocol.nextCommandID()
	if err := protocol.process.send(command); err != nil {
		return nil, barrier.fail(err)
	}
	frame, err := protocol.process.next(ctx)
	if err != nil {
		return nil, barrier.fail(err)
	}
	protocol.probe.observeCompactionFrame(frame)
	if frame.Type == "extension_ui_request" {
		return nil, barrier.fail(barrier.refuseUIRequest(frame))
	}
	if frame.Type == "response" && frame.ID == command.ID &&
		frame.Command == command.Type && frame.Success {
		return append(json.RawMessage(nil), frame.Data...), nil
	}
	return nil, barrier.fail(pipelineOMPActiveProofFrameError(frame))
}

// refuseUIRequest counts authenticated checkpoints but acknowledges none.
// Unlike the outer compaction loop, a proof query permits no unrelated frame,
// including notifications: only its exact response can establish the proof.
func (barrier *pipelineOMPActiveProofBarrier) refuseUIRequest(frame pipelineOMPRPCFrame) error {
	event, bridgeErr := validatePipelineOMPActiveBridgeFrame(frame, barrier.binding)
	if bridgeErr == nil {
		barrier.checkpoints.countDuringProof(event)
		return errors.New("managed active OMP checkpoint arrived during the compaction proof")
	}
	return bridgeErr
}

// fail keeps the first precise reason this reader produced.
func (barrier *pipelineOMPActiveProofBarrier) fail(err error) error {
	if barrier.failure == nil {
		barrier.failure = err
	}
	return err
}

// pipelineOMPActiveProofFrameError names why a frame is not the page answer.
// No frame body is read; only the protocol type decides the reason, so the
// message stays body-free.
func pipelineOMPActiveProofFrameError(frame pipelineOMPRPCFrame) error {
	switch frame.Type {
	case "extension_error":
		return errors.New("managed active OMP extension failed during manual compaction")
	case "agent_start", "turn_start", "turn_end", "agent_end", "prompt_result":
		return errors.New("managed active OMP provider activity crossed the compaction barrier")
	}
	return errors.New("managed active OMP compaction proof query was not answered")
}
