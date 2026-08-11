package cli

import (
	"context"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/orcarun"
	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// pipelineOrcaWaitOutcome is what one bounded supervision loop observed.
type pipelineOrcaWaitOutcome struct {
	done *orcarun.Message
	// notes carry evidence this backend could not act on: escalations,
	// questions, and acknowledgement failures.
	notes []string
	// awaitingHuman records that the worker asked for a decision no
	// automated execution owner can give.
	awaitingHuman bool
	failure       string
	timedOut      bool
}

// superviseWorker waits for the worker to settle, collects its bounded output,
// and closes the dispatch out exactly once.
func (backend *pipelineOrcaBackend) superviseWorker(
	ctx context.Context,
	dispatchID string,
	response *pipeline.PhaseResponse,
) (*pipeline.PhaseResponse, error) {
	outcome, waitErr := backend.awaitWorker(ctx, dispatchID)
	fallback := strings.TrimSpace(strings.Join(outcome.notes, "\n"))
	if outcome.done != nil {
		fallback = outcome.done.Body
	}
	response.Output = backend.readWorkerOutput(ctx, dispatchID, fallback)
	response.TimedOut = outcome.timedOut
	// A cleanup failure is recorded, never a reason to drop the attempt result.
	settleErr := backend.settleDispatch(ctx, dispatchID, outcome.done != nil)
	switch {
	case waitErr != nil:
		response.FailureClass = pipelineOrcaFailureClass(
			outcome.failure, "", joinPipelineOrcaDetail("", waitErr, settleErr))
		return response, waitErr
	case outcome.done == nil:
		response.FailureClass = pipelineOrcaFailureClass(
			outcome.failure, "", joinPipelineOrcaDetail("", settleErr))
	case outcome.done.Outcome != orcarun.OutcomeSucceeded:
		response.FailureClass = pipelineOrcaFailureClass(
			"worker_reported_"+outcome.done.Outcome, "", joinPipelineOrcaDetail("", settleErr))
	default:
		response.ExitCode = 0
		response.FailureClass = pipelineOrcaFailureClass("", "", joinPipelineOrcaDetail("", settleErr))
	}
	return response, nil
}

// awaitWorker rolls WaitWindow-sized waits until the phase deadline.
//
// An empty delivery or an expired wait window is a checkpoint, not a failure:
// supervised coding work routinely runs far longer than one window. Every
// delivery is fully processed and then acknowledged, and only a worker_done
// naming this dispatch ends the loop — a FIFO batch may carry messages that
// belong to another dispatch.
//
// @AX:WARN [AUTO]: the supervision loop branches across deadline, window sizing, delivery errors, message kinds, and acknowledgement.
// @AX:REASON [AUTO]: Each branch must either keep waiting inside the deadline or leave the loop with a settle-able outcome.
func (backend *pipelineOrcaBackend) awaitWorker(
	ctx context.Context,
	dispatchID string,
) (pipelineOrcaWaitOutcome, error) {
	outcome := pipelineOrcaWaitOutcome{}
	deadline := time.Now().Add(backend.config.PhaseTimeout)
	for {
		if err := ctx.Err(); err != nil {
			outcome.failure = "worker_wait_cancelled"
			return outcome, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			outcome.timedOut = true
			outcome.failure = "worker_wait_timeout"
			if outcome.awaitingHuman {
				outcome.failure = "worker_awaiting_human"
			}
			return outcome, nil
		}
		window := backend.config.WaitWindow
		if window > remaining {
			window = remaining
		}
		delivery, err := backend.config.Client.Wait(ctx, window)
		if err != nil {
			outcome.failure = "worker_wait_error"
			return outcome, err
		}
		backend.consumeDelivery(ctx, dispatchID, delivery, &outcome)
		if outcome.done != nil {
			return outcome, nil
		}
	}
}

// consumeDelivery records every message that belongs to this dispatch and then
// acknowledges the delivery, so the batch is not replayed forever.
func (backend *pipelineOrcaBackend) consumeDelivery(
	ctx context.Context,
	dispatchID string,
	delivery orcarun.Delivery,
	outcome *pipelineOrcaWaitOutcome,
) {
	for index := range delivery.Messages {
		message := delivery.Messages[index]
		if message.DispatchID != dispatchID {
			continue
		}
		switch message.Type {
		case orcarun.MessageWorkerDone:
			outcome.done = &delivery.Messages[index]
		case orcarun.MessageEscalation, orcarun.MessageQuestion:
			// This backend cannot answer; the note is kept as evidence and
			// the wait continues until the phase deadline.
			outcome.awaitingHuman = true
			outcome.notes = append(outcome.notes,
				message.Type+": "+pipelineOrcaCompact(message.Subject+" "+message.Body))
		}
	}
	if delivery.DeliveryID == "" {
		return
	}
	if err := backend.config.Client.Ack(ctx, delivery.DeliveryID); err != nil {
		outcome.notes = append(outcome.notes, "ack_failed: "+pipelineOrcaCompact(err.Error()))
	}
}

// joinPipelineOrcaDetail folds free text and error values into one detail line.
func joinPipelineOrcaDetail(text string, errs ...error) string {
	parts := make([]string, 0, len(errs)+1)
	if strings.TrimSpace(text) != "" {
		parts = append(parts, text)
	}
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return strings.Join(parts, "; ")
}

// pipelineOrcaFailureClass renders a stage- and detail-qualified failure class
// so the phase receipt shows why the process plane refused or lost the worker.
func pipelineOrcaFailureClass(kind, stage, detail string) string {
	parts := make([]string, 0, 3)
	if trimmed := strings.TrimSpace(kind); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(stage); trimmed != "" {
		parts = append(parts, "stage="+pipelineOrcaCompact(trimmed))
	}
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		parts = append(parts, "error="+pipelineOrcaCompact(trimmed))
	}
	return strings.Join(parts, "; ")
}

// pipelineOrcaCompact flattens and bounds a detail string so a noisy worker
// cannot push unbounded text into a receipt.
func pipelineOrcaCompact(text string) string {
	compact := strings.Join(strings.Fields(text), " ")
	if len(compact) > pipelineOrcaFailureDetailLimit {
		return compact[:pipelineOrcaFailureDetailLimit] + "..."
	}
	return compact
}
