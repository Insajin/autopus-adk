package orcarun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

const (
	// defaultBinary is resolved on PATH rather than pinned to an install
	// location, matching how the execplane account probe finds the same CLI.
	defaultBinary = "orca"
	// defaultMaxOutput bounds one response. Orchestration responses are control
	// metadata plus, at most, one transcript page; anything larger is a
	// malfunction and must not be buffered into the pipeline's memory.
	defaultMaxOutput = 1 << 20
)

// Client invokes the orca orchestration CLI with bounded output. The zero value
// is usable and behaves exactly like New.
//
// Exec is the only seam that touches the outside world. A nil Exec runs the real
// CLI, so tests substitute recorded responses without a binary on PATH.
type Client struct {
	Binary    string
	MaxOutput int
	Exec      func(ctx context.Context, binary string, args ...string) ([]byte, error)
}

// New returns a Client wired to the real orca CLI with the default bounds.
func New() Client {
	return Client{Binary: defaultBinary, MaxOutput: defaultMaxOutput}
}

func (c Client) binary() string {
	if c.Binary != "" {
		return c.Binary
	}
	return defaultBinary
}

func (c Client) maxOutput() int {
	if c.MaxOutput > 0 {
		return c.MaxOutput
	}
	return defaultMaxOutput
}

// execOrca is the default Exec seam: locate the CLI, run it under the caller's
// context, and cap what it may hand back.
//
// Only stdout is returned. `check --wait` emits a keepalive JSON line on stderr
// every 15 seconds; OutputLimited caps stderr separately and discards it, so a
// long wait cannot pollute the decoded envelope or grow without bound.
func (c Client) execOrca(ctx context.Context, binary string, args ...string) ([]byte, error) {
	resolved, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOrcaUnavailable, err)
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	return processprobe.OutputLimited(cmd, c.maxOutput())
}

// call runs one orca command and decodes its envelope into result, which may be
// nil when the command's payload carries nothing the caller needs.
//
// A non-zero exit and a well-formed `ok:false` envelope arrive together, so the
// envelope is decoded first: the error code it carries is strictly more
// diagnostic than the exit status that accompanied it.
func (c Client) call(ctx context.Context, result any, args ...string) error {
	run := c.Exec
	if run == nil {
		run = c.execOrca
	}
	output, execErr := run(ctx, c.binary(), args...)

	decodeErr := decodeEnvelope(output, result)
	if decodeErr == nil {
		return nil
	}
	if execErr != nil && errors.Is(decodeErr, errEnvelopeUnreadable) {
		return fmt.Errorf("run orca %s: %w", commandLabel(args), execErr)
	}
	return fmt.Errorf("orca %s: %w", commandLabel(args), decodeErr)
}

// decodeEnvelope unwraps the shared response envelope and performs the second
// decoding into a command-specific payload.
func decodeEnvelope(output []byte, result any) error {
	var wrapper envelope
	if err := json.Unmarshal(output, &wrapper); err != nil {
		return fmt.Errorf("%w: %w", errEnvelopeUnreadable, err)
	}
	if !wrapper.OK {
		return envelopeFailure(wrapper.Error)
	}
	if result == nil {
		return nil
	}
	if isEmptyJSON(wrapper.Result) {
		return fmt.Errorf("%w: ok response carried no result", errEnvelopeUnreadable)
	}
	if err := json.Unmarshal(wrapper.Result, result); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

// envelopeFailure renders a rejection. Both the code and the message are kept:
// the code is what a caller can branch on, and an error missing it cannot be
// diagnosed after the fact.
func envelopeFailure(failure *envelopeError) error {
	if failure == nil {
		return errors.New("rejected without an error code")
	}
	code := failure.Code
	if code == "" {
		code = "unknown"
	}
	if failure.Message == "" {
		return fmt.Errorf("rejected: %s", code)
	}
	return fmt.Errorf("rejected: %s: %s", code, failure.Message)
}

// commandLabel names the invoked command for error messages, using the leading
// non-flag argv words so `orchestration worker-start` stays recognizable.
func commandLabel(args []string) string {
	words := make([]string, 0, 2)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			break
		}
		words = append(words, arg)
		if len(words) == 2 {
			break
		}
	}
	if len(words) == 0 {
		return "command"
	}
	return strings.Join(words, " ")
}

// isEmptyJSON reports a value orca omitted. A JSON null decodes silently into a
// zero-valued payload, so it must be rejected explicitly rather than accepted
// as an empty result.
func isEmptyJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

// rawString reads a JSON value that orca has not pinned to a type. A JSON
// string is unquoted, any other scalar is kept verbatim, and null is empty.
func rawString(raw json.RawMessage) string {
	if isEmptyJSON(raw) {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return strings.TrimSpace(string(raw))
}
