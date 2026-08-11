package orcarun

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// waitGrace is how much longer than its own deadline a `check --wait` may take
// before the context cancels it. The flag already bounds the wait server-side;
// this only defends against a CLI that never returns at all.
const waitGrace = 30 * time.Second

// waitTypes is the message set a phase cares about. status and heartbeat
// traffic is deliberately excluded so a chatty worker cannot wake the poller.
var waitTypes = strings.Join([]string{MessageWorkerDone, MessageEscalation, MessageQuestion}, ",")

// CreateRun opens a Run bound to the calling coordinator terminal. Every later
// call in this package must originate from that same terminal or orca rejects
// it with consumer_fenced.
func (c Client) CreateRun(ctx context.Context, objective string) (Run, error) {
	var payload runCreatePayload
	err := c.call(ctx, &payload, "orchestration", "run-create", "--objective", objective, "--json")
	if err != nil {
		return Run{}, err
	}
	return Run{
		ID:                payload.Run.ID,
		Objective:         payload.Run.Objective,
		CoordinatorHandle: payload.Run.CoordinatorHandle,
	}, nil
}

// CreateTask registers one unit of supervised work.
//
// No dependency edge is ever requested: --deps is absent from the argv on every
// path, because the phase graph lives in the policy plane and a second graph in
// the process plane would duplicate ordering, gates and retries.
func (c Client) CreateTask(ctx context.Context, runID, title, spec string) (Task, error) {
	args := []string{"orchestration", "task-create", "--spec", spec, "--task-title", title}
	if runID != "" {
		args = append(args, "--run", runID)
	}
	args = append(args, "--json")

	var payload taskCreatePayload
	if err := c.call(ctx, &payload, args...); err != nil {
		return Task{}, err
	}
	return Task{
		ID:     payload.Task.ID,
		Status: payload.Task.Status,
		Deps:   payload.Task.Deps,
	}, nil
}

// StartWorker launches one supervised worker and returns as soon as the
// dispatch input is accepted. It does not block until the worker settles;
// callers observe settlement through Wait.
//
// Model and Effort are opaque provider identifiers. An empty one omits its flag
// entirely rather than passing a blank value, which orca would reject.
func (c Client) StartWorker(ctx context.Context, req StartRequest) (Worker, error) {
	args := []string{
		"orchestration", "worker-start",
		"--task", req.TaskID,
		"--worktree", req.Worktree,
		"--agent", req.Agent,
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}
	if req.RunID != "" {
		args = append(args, "--run", req.RunID)
	}
	if millis := req.Timeout.Milliseconds(); millis > 0 {
		args = append(args, "--timeout-ms", strconv.FormatInt(millis, 10))
	}
	args = append(args, "--json")

	var payload workerStartPayload
	if err := c.call(ctx, &payload, args...); err != nil {
		return Worker{}, err
	}
	return Worker{
		RunID:      payload.RunID,
		TaskID:     payload.TaskID,
		DispatchID: payload.DispatchID,
		State:      payload.State,
		Stage:      payload.Stage,
		LastError:  payload.LastError,
		Requested:  toLaunch(payload.Launch.Requested),
		Effective:  toLaunch(payload.Launch.Effective),
		Residual:   toResources(payload.ResidualResources),
	}, nil
}

// Wait blocks for one delivery batch or until the window elapses. A timed-out
// window is a checkpoint, not a failure: coding workers routinely run far
// longer than any single window, so callers roll the window forward and bound
// the total elsewhere.
//
// The returned batch replays until Ack, and may include messages belonging to
// other dispatches, so callers must filter on Message.DispatchID.
func (c Client) Wait(ctx context.Context, window time.Duration) (Delivery, error) {
	args := []string{"orchestration", "check", "--wait", "--types", waitTypes}
	if millis := window.Milliseconds(); millis > 0 {
		args = append(args, "--timeout-ms", strconv.FormatInt(millis, 10))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, window+waitGrace)
		defer cancel()
	}
	args = append(args, "--json")

	var payload checkPayload
	if err := c.call(ctx, &payload, args...); err != nil {
		return Delivery{}, err
	}
	return Delivery{
		RunID:      payload.RunID,
		DeliveryID: payload.DeliveryID,
		Messages:   toMessages(payload.Messages),
		Count:      payload.Count,
		TimedOut:   payload.TimedOut,
	}, nil
}

// Ack retires a delivery batch. Without it the same batch is served again on
// the next Wait.
func (c Client) Ack(ctx context.Context, deliveryID string) error {
	return c.call(ctx, nil, "orchestration", "check", "--ack", deliveryID, "--json")
}

// ReadTranscript extracts what a worker produced, capped at limit messages.
// The archive survives release, so a settled dispatch is still readable.
func (c Client) ReadTranscript(ctx context.Context, dispatchID string, limit int) (Transcript, error) {
	args := []string{"orchestration", "worker-read", "--dispatch", dispatchID}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	args = append(args, "--json")

	var payload workerReadPayload
	if err := c.call(ctx, &payload, args...); err != nil {
		return Transcript{}, err
	}
	return Transcript{
		DispatchID: payload.DispatchID,
		Source:     payload.Source,
		Provider:   payload.Provider,
		Status:     payload.Status,
		Cursor:     rawString(payload.Cursor),
		Text:       flattenTranscript(payload),
	}, nil
}

// Release settles a dispatch and closes the resources it owns. It is idempotent
// and is only correct once the worker is known to have stopped; a timeout, an
// idle TUI or a stale worker_done calls for Abandon instead.
func (c Client) Release(ctx context.Context, dispatchID string) (Settlement, error) {
	return c.settle(ctx, "worker-release", dispatchID)
}

// Stop fences a dispatch and closes the exact agent terminal it started. Unlike
// Abandon it actually stops the worker, which is what a give-up path wants: an
// abandoned agent keeps running in the caller's worktree and keeps spending.
func (c Client) Stop(ctx context.Context, dispatchID string) (Settlement, error) {
	return c.settle(ctx, "worker-stop", dispatchID)
}

// Abandon fences a dispatch without touching its processes or files. It is the
// settlement for every case where the worker cannot be claimed to have stopped.
func (c Client) Abandon(ctx context.Context, dispatchID string) (Settlement, error) {
	return c.settle(ctx, "worker-abandon", dispatchID)
}

func (c Client) settle(ctx context.Context, command, dispatchID string) (Settlement, error) {
	var payload settlementPayload
	err := c.call(ctx, &payload, "orchestration", command, "--dispatch", dispatchID, "--json")
	if err != nil {
		return Settlement{}, err
	}
	return Settlement{
		DispatchID:    payload.DispatchID,
		State:         payload.State,
		Reason:        payload.Reason,
		ProcessAction: payload.ProcessAction,
	}, nil
}

// CloseTerminal kills a terminal by handle. This is the cleanup for residual
// resources: a terminal left behind by a failed launch is owned by no dispatch,
// so no settlement command can reach it.
//
// This is a top-level terminal command, not an orchestration subcommand.
func (c Client) CloseTerminal(ctx context.Context, handle string) error {
	return c.call(ctx, nil, "terminal", "close", "--terminal", handle, "--json")
}

func toLaunch(payload launchPayload) Launch {
	return Launch{Agent: payload.Agent, Model: payload.Model, Effort: payload.Effort}
}

func toResources(payloads []resourcePayload) []Resource {
	if len(payloads) == 0 {
		return nil
	}
	resources := make([]Resource, len(payloads))
	for index, payload := range payloads {
		resources[index] = Resource{
			Kind:   payload.Kind,
			Role:   payload.Role,
			ID:     payload.ID,
			Action: payload.Action,
		}
	}
	return resources
}

// toMessages performs the second payload decoding. A message whose payload is
// absent or malformed keeps empty identifiers instead of being dropped: the
// caller filters on DispatchID anyway, and discarding the message would hide a
// delivery that still has to be acknowledged.
func toMessages(payloads []messagePayload) []Message {
	if len(payloads) == 0 {
		return nil
	}
	messages := make([]Message, len(payloads))
	for index, payload := range payloads {
		message := Message{
			ID:      payload.ID,
			Type:    payload.Type,
			Subject: payload.Subject,
			Body:    payload.Body,
		}
		var body messageBody
		if err := json.Unmarshal([]byte(payload.Payload), &body); err == nil {
			message.TaskID = body.TaskID
			message.DispatchID = body.DispatchID
			message.Outcome = body.Outcome
		}
		messages[index] = message
	}
	return messages
}

// flattenTranscript joins the worker's own text in order.
//
// Two things are dropped. Non-text blocks carry no phase result: on a real run
// the agent's substantive report leaves as a tool call, not as text. Non-
// assistant messages are not output at all — the dispatched preamble arrives
// as a user message, and echoing it back would feed a phase its own prompt.
// Measured on a live 7-dispatch run, that preamble was 1214 bytes per phase
// against 0 bytes of assistant text in six of seven phases.
func flattenTranscript(payload workerReadPayload) string {
	var builder strings.Builder
	for _, message := range payload.Transcript.Messages {
		if message.Role != transcriptRoleAssistant {
			continue
		}
		for _, block := range message.Blocks {
			if block.Type != "text" || block.Text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(block.Text)
		}
	}
	return builder.String()
}
