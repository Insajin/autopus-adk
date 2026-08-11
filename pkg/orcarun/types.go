// Package orcarun invokes the orca orchestration CLI as a supervised worker
// plane. It owns no ordering: every call maps onto exactly one orca command, so
// the policy plane keeps sole ownership of the phase graph.
package orcarun

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	// WorkerStateReady is the state a worker reports once its dispatch input
	// was accepted. worker-start returns before the worker settles, so this is
	// an admission signal and never a completion signal.
	WorkerStateReady = "ready"
	// OutcomeSucceeded and OutcomeFailed are the two values a worker_done
	// payload carries. Anything else is an unknown outcome, not a success.
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
	// MessageWorkerDone, MessageEscalation and MessageQuestion are the message
	// types Wait subscribes to. Only worker_done settles a dispatch; the other
	// two are checkpoints the caller decides on.
	MessageWorkerDone = "worker_done"
	MessageEscalation = "escalation"
	MessageQuestion   = "question"
	// WorktreeCurrent starts a worker in the worktree the coordinator already
	// occupies, rather than provisioning a new one per phase.
	WorktreeCurrent = "current"
	// transcriptRoleAssistant is the only transcript role that is worker
	// output. User messages are the dispatched preamble.
	transcriptRoleAssistant = "assistant"
)

// ErrOrcaUnavailable reports that the orca CLI could not be located on PATH.
// It is a sentinel because the caller's response is prescribed: no CLI means
// no supervised execution, which the backend turns into a configuration error
// rather than a phase failure.
var ErrOrcaUnavailable = errors.New("orcarun: orca CLI is not available on PATH")

// errEnvelopeUnreadable marks output that is not a response envelope at all,
// which lets the caller prefer the underlying process error when both happen.
var errEnvelopeUnreadable = errors.New("orcarun: response envelope is unreadable")

// Run is a coordinator-bound orchestration Run. The Run is fenced to the
// terminal that created it, so CoordinatorHandle is retained for diagnosis of
// consumer_fenced failures.
type Run struct {
	ID                string
	Objective         string
	CoordinatorHandle string
}

// Task is one unit of supervised work. Deps is kept verbatim as the JSON string
// orca returns ("[]" when no dependency was requested) so callers can assert
// the no-second-DAG invariant on observed state instead of on intent.
type Task struct {
	ID     string
	Status string
	Deps   string
}

// StartRequest describes one worker launch. Model and Effort are opaque
// provider identifiers; policy-plane tier names never reach this struct.
type StartRequest struct {
	RunID    string
	TaskID   string
	Agent    string
	Model    string
	Effort   string
	Worktree string
	Timeout  time.Duration
}

// Launch records what was asked for and what orca actually resolved. The two
// are kept apart because a provider may substitute a model silently.
type Launch struct {
	Agent  string
	Model  string
	Effort string
}

// Resource is one orca-managed object touched by a launch. A non-empty residual
// set is the only handle on a terminal that outlived its dispatch.
type Resource struct {
	Kind   string
	Role   string
	ID     string
	Action string
}

// Worker is the immediate result of worker-start. It is an admission receipt,
// not a completion: State is "ready" long before the worker settles.
type Worker struct {
	RunID      string
	TaskID     string
	DispatchID string
	State      string
	Stage      string
	LastError  string
	Requested  Launch
	Effective  Launch
	Residual   []Resource
}

// Message is one delivered orchestration message. TaskID, DispatchID and
// Outcome come from the payload, which orca transports as a JSON string nested
// inside the response JSON.
type Message struct {
	ID         string
	Type       string
	Subject    string
	Body       string
	TaskID     string
	DispatchID string
	Outcome    string
}

// Delivery is one FIFO batch. The same batch replays until it is acknowledged,
// and it may carry messages belonging to other dispatches, so callers must
// filter on Message.DispatchID before acting.
type Delivery struct {
	RunID      string
	DeliveryID string
	Messages   []Message
	Count      int
	TimedOut   bool
}

// Transcript is the flattened text a worker produced. Non-text blocks are
// dropped because the engine consumes a single output string.
type Transcript struct {
	DispatchID string
	Source     string
	Provider   string
	Status     string
	Cursor     string
	Text       string
}

// Settlement is the terminal state of a dispatch after release or abandon.
type Settlement struct {
	DispatchID    string
	State         string
	Reason        string
	ProcessAction string
}

// envelope is the response shape shared by every `orca ... --json` command.
type envelope struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Error  *envelopeError  `json:"error"`
	Result json.RawMessage `json:"result"`
}

// envelopeError carries both a stable code and a human message. Both are
// surfaced: the code is the only machine-actionable part, and dropping it makes
// failures such as consumer_fenced indistinguishable from transport noise.
type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type runCreatePayload struct {
	Run struct {
		ID                string `json:"id"`
		Objective         string `json:"objective"`
		CoordinatorHandle string `json:"coordinator_handle"`
	} `json:"run"`
}

type taskCreatePayload struct {
	Task struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Deps   string `json:"deps"`
	} `json:"task"`
}

type launchPayload struct {
	Agent  string `json:"agent"`
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type resourcePayload struct {
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	ID     string `json:"id"`
	Action string `json:"action"`
}

type workerStartPayload struct {
	RunID      string `json:"runId"`
	TaskID     string `json:"taskId"`
	DispatchID string `json:"dispatchId"`
	State      string `json:"state"`
	Stage      string `json:"stage"`
	LastError  string `json:"lastError"`
	Launch     struct {
		Requested launchPayload `json:"requested"`
		Effective launchPayload `json:"effective"`
	} `json:"launch"`
	ResidualResources []resourcePayload `json:"residualResources"`
}

type messagePayload struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Payload string `json:"payload"`
}

// messageBody is the second decoding layer: message.payload is a JSON document
// serialized into a JSON string.
type messageBody struct {
	TaskID     string `json:"taskId"`
	DispatchID string `json:"dispatchId"`
	Outcome    string `json:"outcome"`
}

type checkPayload struct {
	RunID      string           `json:"runId"`
	DeliveryID string           `json:"deliveryId"`
	Messages   []messagePayload `json:"messages"`
	Count      int              `json:"count"`
	TimedOut   bool             `json:"timedOut"`
}

type blockPayload struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type workerReadPayload struct {
	DispatchID string `json:"dispatchId"`
	Source     string `json:"source"`
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	// Cursor is raw because orca has not fixed it to a JSON type; a numeric
	// cursor must not fail the whole read.
	Cursor     json.RawMessage `json:"cursor"`
	Transcript struct {
		Messages []struct {
			// Role separates what the worker produced from what was dispatched
			// into it. The injected preamble arrives as a user message, so a
			// role-blind read would hand the caller back its own prompt.
			Role   string         `json:"role"`
			Blocks []blockPayload `json:"blocks"`
		} `json:"messages"`
	} `json:"transcript"`
}

type settlementPayload struct {
	DispatchID    string `json:"dispatchId"`
	State         string `json:"state"`
	Reason        string `json:"reason"`
	ProcessAction string `json:"processAction"`
}
