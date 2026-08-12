package orcarun

import (
	"context"
	"strings"
	"testing"
	"time"
)

const workerDoneResponse = `{"id":"req_5","ok":true,"error":null,"result":{` +
	`"runId":"run_88012357eb61","deliveryId":"delivery_15acfc197f0f","messages":[` +
	`{"id":"msg_d32888d3808e","run_id":"run_88012357eb61","from_handle":"term_4d1",` +
	`"to_handle":"run:run_88012357eb61","subject":"probe ok","body":"probe ok","type":"worker_done",` +
	`"priority":"normal","thread_id":null,` +
	`"payload":"{\"taskId\":\"task_a38759fca084\",\"dispatchId\":\"ctx_532647ebc127\",\"outcome\":\"succeeded\"}",` +
	`"read":0,"sequence":83,"created_at":"2026-08-11T00:00:00Z","delivered_at":null,` +
	`"sender_pane_key":"pane_1","delivery_contract":"current_delivery"}],` +
	`"count":1,"replayed":true,"acknowledged":null,"timedOut":false,"cancelled":false,` +
	`"connectionLost":false,"mutation":{"requestId":"mut_5"}},"_meta":{"runtimeId":"rt_1"}}`

const brokenPayloadResponse = `{"id":"req_6","ok":true,"error":null,"result":{` +
	`"runId":"run_88012357eb61","deliveryId":"delivery_broken","messages":[` +
	`{"id":"msg_broken","subject":"truncated","body":"truncated","type":"worker_done","payload":"{not json"},` +
	`{"id":"msg_absent","subject":"asking","body":"asking","type":"question","payload":null},` +
	`{"id":"msg_ok","subject":"done","body":"done","type":"worker_done",` +
	`"payload":"{\"taskId\":\"task_a38759fca084\",\"dispatchId\":\"ctx_532647ebc127\",\"outcome\":\"failed\"}"}],` +
	`"count":3,"timedOut":false},"_meta":{"runtimeId":"rt_1"}}`

// Shaped after a live dispatch: the injected preamble is a long user message,
// and the assistant contributes tool calls plus a short line of text.
const transcriptResponse = `{"id":"req_7","ok":true,"error":null,"result":{` +
	`"dispatchId":"ctx_532647ebc127","source":"transcript","sourceIdentity":"claude-session",` +
	`"provider":"claude","transcript":{"messages":[` +
	`{"id":"m1","role":"user","blocks":[{"type":"text","text":"You are a dispatched worker. run the plan phase"}]},` +
	`{"id":"m2","role":"assistant","blocks":[` +
	`{"type":"thinking","text":"internal deliberation"},` +
	`{"type":"tool_use","name":"bash","input":{"command":"ls"}},` +
	`{"type":"text","text":"plan written"}]},` +
	`{"id":"m3","role":"tool","blocks":[{"type":"text","text":"tool stdout"}]}]},` +
	`"cursor":83,"status":"complete","fallbackReason":null,"warnings":[]},"_meta":{"runtimeId":"rt_1"}}`

// Every message whose blocks are entirely tool calls, which is what a worker
// that reports through worker_done actually produces.
const toolOnlyTranscriptResponse = `{"id":"req_8","ok":true,"error":null,"result":{` +
	`"dispatchId":"ctx_532647ebc127","source":"transcript","sourceIdentity":"claude-session",` +
	`"provider":"claude","transcript":{"messages":[` +
	`{"id":"m1","role":"user","blocks":[{"type":"text","text":"You are a dispatched worker. run validate"}]},` +
	`{"id":"m2","role":"assistant","blocks":[{"type":"tool-call","name":"bash"}]}]},` +
	`"cursor":9,"status":"complete","fallbackReason":null,"warnings":[]},"_meta":{"runtimeId":"rt_1"}}`

// The payload is a JSON document transported as a JSON string. Without the
// second decoding the caller cannot tell whose dispatch settled, or how.
func TestWaitDecodesNestedPayload(t *testing.T) {
	client, recorder := stubbedClient(workerDoneResponse)

	delivery, err := client.Wait(context.Background(), 5*time.Minute)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if delivery.DeliveryID != "delivery_15acfc197f0f" || delivery.Count != 1 || delivery.TimedOut {
		t.Fatalf("unexpected delivery %+v", delivery)
	}
	if len(delivery.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(delivery.Messages))
	}
	message := delivery.Messages[0]
	if message.Type != MessageWorkerDone || message.ID != "msg_d32888d3808e" {
		t.Fatalf("unexpected message %+v", message)
	}
	if message.DispatchID != "ctx_532647ebc127" {
		t.Fatalf("dispatch id was not decoded: %+v", message)
	}
	if message.TaskID != "task_a38759fca084" || message.Outcome != OutcomeSucceeded {
		t.Fatalf("payload was not decoded: %+v", message)
	}
	if message.Subject != "probe ok" || message.Body != "probe ok" {
		t.Fatalf("envelope fields were lost: %+v", message)
	}

	want := "orchestration check --wait --types worker_done,escalation,question --timeout-ms 300000 --json"
	if got := strings.Join(recorder.lastArgs(t), " "); got != want {
		t.Fatalf("unexpected argv: %s", got)
	}
}

// A batch replays until acknowledged, so an undecodable payload must not remove
// the message from the batch. Empty identifiers let the caller skip it while
// still acknowledging the delivery.
func TestWaitKeepsMessagesWithUnreadablePayload(t *testing.T) {
	client, _ := stubbedClient(brokenPayloadResponse)

	delivery, err := client.Wait(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if len(delivery.Messages) != 3 || delivery.Count != 3 {
		t.Fatalf("expected every message to survive, got %+v", delivery.Messages)
	}
	for _, index := range []int{0, 1} {
		message := delivery.Messages[index]
		if message.TaskID != "" || message.DispatchID != "" || message.Outcome != "" {
			t.Fatalf("message %d invented identifiers: %+v", index, message)
		}
		if message.ID == "" || message.Type == "" {
			t.Fatalf("message %d lost its envelope fields: %+v", index, message)
		}
	}
	settled := delivery.Messages[2]
	if settled.DispatchID != "ctx_532647ebc127" || settled.Outcome != OutcomeFailed {
		t.Fatalf("a readable payload after a broken one was lost: %+v", settled)
	}
}

// A non-positive window has no deadline to send, and must not put a zero or
// negative millisecond value on the wire.
func TestWaitOmitsNonPositiveWindow(t *testing.T) {
	client, recorder := stubbedClient(workerDoneResponse)

	if _, err := client.Wait(context.Background(), 0); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if argvIndex(recorder.lastArgs(t), "--timeout-ms") >= 0 {
		t.Fatalf("argv %v must omit --timeout-ms", recorder.lastArgs(t))
	}
}

// Transcript text is the worker's own text and nothing else. Two things must
// stay out: non-text blocks, which carry no result, and non-assistant
// messages, because the dispatched preamble arrives as a user message and
// echoing it back would feed a phase its own prompt.
func TestReadTranscriptKeepsOnlyAssistantText(t *testing.T) {
	client, recorder := stubbedClient(transcriptResponse)

	transcript, err := client.ReadTranscript(context.Background(), "ctx_532647ebc127", 200)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if transcript.Text != "plan written" {
		t.Fatalf("unexpected transcript text %q", transcript.Text)
	}
	if strings.Contains(transcript.Text, "dispatched worker") {
		t.Fatal("the injected preamble was returned as worker output")
	}
	if strings.Contains(transcript.Text, "internal deliberation") ||
		strings.Contains(transcript.Text, "tool stdout") {
		t.Fatal("non-text blocks or tool messages leaked into the transcript")
	}
	if transcript.Provider != "claude" || transcript.Source != "transcript" {
		t.Fatalf("unexpected transcript %+v", transcript)
	}
	if transcript.Status != "complete" || transcript.Cursor != "83" {
		t.Fatalf("cursor or status was lost: %+v", transcript)
	}

	want := "orchestration worker-read --dispatch ctx_532647ebc127 --limit 200 --json"
	if got := strings.Join(recorder.lastArgs(t), " "); got != want {
		t.Fatalf("unexpected argv: %s", got)
	}
}

// A worker that reports through worker_done leaves no assistant text at all.
// The transcript must then be empty rather than the preamble, so the caller
// can tell there is no detail to add and fall back to the report.
func TestReadTranscriptIsEmptyWhenWorkerOnlyCalledTools(t *testing.T) {
	client, _ := stubbedClient(toolOnlyTranscriptResponse)

	transcript, err := client.ReadTranscript(context.Background(), "ctx_532647ebc127", 200)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if transcript.Text != "" {
		t.Fatalf("expected no worker text, got %q", transcript.Text)
	}
}

func TestReadTranscriptOmitsNonPositiveLimit(t *testing.T) {
	client, recorder := stubbedClient(transcriptResponse)

	if _, err := client.ReadTranscript(context.Background(), "ctx_532647ebc127", 0); err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if argvIndex(recorder.lastArgs(t), "--limit") >= 0 {
		t.Fatalf("argv %v must omit --limit", recorder.lastArgs(t))
	}
}

// Release and abandon are different settlements and must not collapse onto one
// command: abandon is the only correct answer when the worker cannot be claimed
// to have stopped.
func TestSettlementCommands(t *testing.T) {
	released := `{"id":"req_8","ok":true,"error":null,"result":{"dispatchId":"ctx_532647ebc127",` +
		`"state":"released","reason":null,"processAction":"closed_agent_terminal",` +
		`"archive":{"source":"transcript","status":"captured"}}}`
	retained := `{"id":"req_9","ok":true,"error":null,"result":{"dispatchId":"ctx_7a1b2c3d4e5f",` +
		`"state":"retained","reason":"no_owned_resource","processAction":"none","archive":null}}`
	client, recorder := stubbedClient(released, retained)
	ctx := context.Background()

	settlement, err := client.Release(ctx, "ctx_532647ebc127")
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if settlement.State != "released" || settlement.ProcessAction != "closed_agent_terminal" {
		t.Fatalf("unexpected settlement %+v", settlement)
	}
	want := "orchestration worker-release --dispatch ctx_532647ebc127 --json"
	if got := strings.Join(recorder.lastArgs(t), " "); got != want {
		t.Fatalf("unexpected release argv: %s", got)
	}

	settlement, err = client.Abandon(ctx, "ctx_7a1b2c3d4e5f")
	if err != nil {
		t.Fatalf("abandon: %v", err)
	}
	if settlement.State != "retained" || settlement.Reason != "no_owned_resource" {
		t.Fatalf("unexpected settlement %+v", settlement)
	}
	want = "orchestration worker-abandon --dispatch ctx_7a1b2c3d4e5f --json"
	if got := strings.Join(recorder.lastArgs(t), " "); got != want {
		t.Fatalf("unexpected abandon argv: %s", got)
	}
}
