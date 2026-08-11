package orcarun

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// recordedCall is one observed invocation of the Exec seam.
type recordedCall struct {
	binary string
	args   []string
}

// execRecorder stands in for the orca CLI. Responses are served in call order
// so a test can drive a multi-command sequence without a binary on PATH.
type execRecorder struct {
	responses []string
	err       error
	calls     []recordedCall
}

func (r *execRecorder) exec(_ context.Context, binary string, args ...string) ([]byte, error) {
	index := len(r.calls)
	r.calls = append(r.calls, recordedCall{binary: binary, args: append([]string(nil), args...)})
	if index >= len(r.responses) {
		return nil, r.err
	}
	return []byte(r.responses[index]), r.err
}

func stubbedClient(responses ...string) (Client, *execRecorder) {
	recorder := &execRecorder{responses: responses}
	client := New()
	client.Exec = recorder.exec
	return client, recorder
}

func (r *execRecorder) lastArgs(t *testing.T) []string {
	t.Helper()
	if len(r.calls) == 0 {
		t.Fatal("no command was invoked")
	}
	return r.calls[len(r.calls)-1].args
}

func argvIndex(args []string, flag string) int {
	for index, arg := range args {
		if arg == flag {
			return index
		}
	}
	return -1
}

func argvValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	index := argvIndex(args, flag)
	if index < 0 {
		t.Fatalf("argv %v is missing %s", args, flag)
	}
	if index+1 >= len(args) {
		t.Fatalf("argv %v ends after %s", args, flag)
	}
	return args[index+1]
}

const fencedResponse = `{"id":"req_f","ok":false,` +
	`"error":{"code":"consumer_fenced","message":"This coordinator terminal is no longer bound to Run run_88012357eb61."},` +
	`"result":null,"_meta":{"runtimeId":"rt_1"}}`

// A rejection must surface its error code. The message alone reads like
// transport noise, and the code is the only part a caller can branch on.
func TestCallSurfacesRejectionCode(t *testing.T) {
	client, _ := stubbedClient(fencedResponse)

	_, err := client.CreateTask(context.Background(), "run_88012357eb61", "plan", "spec body")
	if err == nil {
		t.Fatal("expected the fenced response to fail")
	}
	if !strings.Contains(err.Error(), "consumer_fenced") {
		t.Fatalf("error %q does not carry the error code", err)
	}
	if !strings.Contains(err.Error(), "no longer bound") {
		t.Fatalf("error %q dropped the diagnostic message", err)
	}
	if !strings.Contains(err.Error(), "orchestration task-create") {
		t.Fatalf("error %q does not name the command", err)
	}
}

// A rejection carrying no error object must still fail rather than decode into
// a zero-valued success.
func TestCallRejectsWithoutErrorObject(t *testing.T) {
	client, _ := stubbedClient(`{"id":"req_g","ok":false,"error":null,"result":null}`)

	if _, err := client.CreateRun(context.Background(), "objective"); err == nil {
		t.Fatal("expected ok:false to fail even without an error object")
	}
}

// An ok:true envelope with no result is a malformed response, not an empty Run.
func TestCallRejectsMissingResult(t *testing.T) {
	client, _ := stubbedClient(`{"id":"req_h","ok":true,"error":null,"result":null}`)

	if _, err := client.CreateRun(context.Background(), "objective"); err == nil {
		t.Fatal("expected a missing result to fail")
	}
}

// Commands with no payload of interest must still honour the envelope.
func TestAckDecodesEmptyResultCommands(t *testing.T) {
	client, recorder := stubbedClient(
		`{"id":"req_a","ok":true,"error":null,"result":{"count":0,"acknowledged":"delivery_15acfc197f0f"}}`,
		fencedResponse,
	)
	ctx := context.Background()

	if err := client.Ack(ctx, "delivery_15acfc197f0f"); err != nil {
		t.Fatalf("ack: %v", err)
	}
	args := recorder.lastArgs(t)
	if got := strings.Join(args, " "); got != "orchestration check --ack delivery_15acfc197f0f --json" {
		t.Fatalf("unexpected ack argv: %s", got)
	}
	if err := client.Ack(ctx, "delivery_15acfc197f0f"); err == nil {
		t.Fatal("expected a rejected ack to fail")
	}
}

// terminal close is a top-level command, not an orchestration subcommand.
// Sending it under orchestration would silently fail to clean residuals.
func TestCloseTerminalUsesTopLevelCommand(t *testing.T) {
	client, recorder := stubbedClient(
		`{"id":"req_c","ok":true,"error":null,"result":{"close":{"handle":"term_9f2","tabId":"tab_1","ptyKilled":true}}}`,
	)

	if err := client.CloseTerminal(context.Background(), "term_9f2"); err != nil {
		t.Fatalf("close terminal: %v", err)
	}
	if got := strings.Join(recorder.lastArgs(t), " "); got != "terminal close --terminal term_9f2 --json" {
		t.Fatalf("unexpected close argv: %s", got)
	}
}

// A missing CLI must degrade to the sentinel so the caller can report a
// configuration fault instead of a phase failure.
func TestExecReportsUnavailableBinary(t *testing.T) {
	client := New()
	client.Binary = "orca-binary-that-does-not-exist-9f2c"

	_, err := client.CreateRun(context.Background(), "objective")
	if !errors.Is(err, ErrOrcaUnavailable) {
		t.Fatalf("expected ErrOrcaUnavailable, got %v", err)
	}
}

// Output beyond the bound must surface as ErrOutputLimit rather than a decode
// failure on a truncated envelope, so a noisy worker is diagnosable.
//
// This drives the real exec path against `echo`, never the orca CLI.
func TestExecBoundsResponseSize(t *testing.T) {
	client := New()
	client.Binary = "echo"
	client.MaxOutput = 8

	_, err := client.CreateRun(context.Background(), strings.Repeat("x", 512))
	if !errors.Is(err, processprobe.ErrOutputLimit) {
		t.Fatalf("expected ErrOutputLimit, got %v", err)
	}
}

// The configured binary must reach the seam; defaulting it away would make a
// pinned CLI path unreachable.
func TestClientPassesConfiguredBinary(t *testing.T) {
	recorder := &execRecorder{responses: []string{
		`{"id":"req_b","ok":true,"error":null,"result":{"run":{"id":"run_1","objective":"o","coordinator_handle":"term_1"}}}`,
	}}
	client := Client{Binary: "/opt/orca", Exec: recorder.exec}

	if _, err := client.CreateRun(context.Background(), "o"); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if recorder.calls[0].binary != "/opt/orca" {
		t.Fatalf("expected the configured binary, got %q", recorder.calls[0].binary)
	}
}
