package cli

// AC013-C1/C2. The cohort tests read acknowledgements back from the fake OMP's
// command log, which can only show what the child managed to write before the
// harness closed it. This transport closes that gap: it is the process stdin
// and frame channel the real protocol already uses, so every acknowledgement
// the production code attempts is captured synchronously, inside the very
// Write call that would have sent it and before any frame comes back. A
// checkpoint that must not be acknowledged therefore cannot hide behind a
// close or kill race — the write either happened or it never existed.
//
// It drives the real protocol.manualCompact with no production hook and no
// child process, so the whole adversarial matrix runs in-process.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/require"
)

// pipelineOMPActiveCheckpointDeadline bounds a stalled exchange so a missing
// frame fails as a timeout instead of hanging the suite.
const pipelineOMPActiveCheckpointDeadline = 30 * time.Second

type pipelineOMPActiveCheckpointLoopback struct {
	fixture     *pipelineOMPActiveCheckpointFixture
	frames      chan pipelineOMPRPCFrame
	encoder     *json.Encoder
	transcript  []json.RawMessage
	acks        []string
	compactions int
}

func newPipelineOMPActiveCheckpointLoopback(variant string) *pipelineOMPActiveCheckpointLoopback {
	loopback := &pipelineOMPActiveCheckpointLoopback{
		fixture: newPipelineOMPActiveCheckpointFixture(pipelineOMPActiveCheckpointModel(variant)),
		// Bounded and generous: one command is answered with at most three
		// frames, and the protocol drains them before it writes again.
		frames:     make(chan pipelineOMPRPCFrame, 64),
		transcript: []json.RawMessage{json.RawMessage(`{"role":"system","content":"safe system context"}`)},
		acks:       []string{},
	}
	loopback.encoder = json.NewEncoder(pipelineOMPActiveCheckpointFrameSink{loopback: loopback})
	return loopback
}

func (loopback *pipelineOMPActiveCheckpointLoopback) process() *pipelineOMPProcess {
	return &pipelineOMPProcess{stdin: loopback, frames: loopback.frames}
}

// Write is the process stdin. The acknowledgement is recorded before the
// fixture may answer, so no frame can ever be observed ahead of the write that
// caused it.
func (loopback *pipelineOMPActiveCheckpointLoopback) Write(data []byte) (int, error) {
	var command pipelineOMPRPCCommand
	if err := json.Unmarshal(data, &command); err != nil {
		return 0, err
	}
	if command.Type == "extension_ui_response" {
		loopback.acks = append(loopback.acks, command.ID)
	}
	switch {
	case loopback.fixture.handles(command.Type):
		loopback.fixture.handle(loopback.encoder, command, loopback.transcript, loopback.complete)
	case command.Type == "get_state":
		writePipelineOMPActiveResponse(loopback.encoder, command, map[string]any{
			"sessionId": pipelineOMPActiveCheckpointSessionID, "isStreaming": false,
			"isCompacting": false, "messageCount": 2, "queuedMessageCount": 0,
			"autoCompactionEnabled": false,
		})
	default:
		writePipelineOMPActiveResponse(loopback.encoder, command, nil)
	}
	return len(data), nil
}

func (loopback *pipelineOMPActiveCheckpointLoopback) Close() error { return nil }

// complete is the mutation a settled compaction performs. Nothing before that
// point may observe it.
func (loopback *pipelineOMPActiveCheckpointLoopback) complete() {
	loopback.compactions++
	loopback.transcript = append(loopback.transcript, pipelineOMPActiveCheckpointSummary())
}

// pipelineOMPActiveCheckpointFrameSink turns the fixture's encoded frames into
// the typed frames the protocol reads, so the exchange the child process runs
// and the exchange this transport runs are the same one.
type pipelineOMPActiveCheckpointFrameSink struct {
	loopback *pipelineOMPActiveCheckpointLoopback
}

func (sink pipelineOMPActiveCheckpointFrameSink) Write(data []byte) (int, error) {
	var frame pipelineOMPRPCFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return 0, err
	}
	sink.loopback.frames <- frame
	return len(data), nil
}

var _ io.WriteCloser = (*pipelineOMPActiveCheckpointLoopback)(nil)

type pipelineOMPActiveCheckpointExchange struct {
	performed bool
	err       error
	record    ompprobe.Record
	// acks is every acknowledgement the protocol attempted to write, in order.
	acks         []string
	readmissions int
	// readmissionAt is how many acknowledgements had been written when the
	// canonical prompt was re-admitted, so AC-013 can assert that it happened
	// at the post-checkpoint and not at either pre.
	readmissionAt int
	loopback      *pipelineOMPActiveCheckpointLoopback
}

// runPipelineOMPActiveCheckpointExchange drives one real manual compaction on
// the measured probe profile against one fixture exchange.
func runPipelineOMPActiveCheckpointExchange(
	t *testing.T,
	variant string,
) pipelineOMPActiveCheckpointExchange {
	t.Helper()
	binding := pipelineOMPActiveCheckpointBinding(t)
	loopback := newPipelineOMPActiveCheckpointLoopback(variant)
	require.Equal(t, variant, loopback.fixture.variant, "the fixture knows this exchange")
	directory := filepath.Join(t.TempDir(), "probe")
	recorder, err := ompprobe.NewRecorder(directory)
	require.NoError(t, err)
	probe := &pipelineOMPActiveProbe{
		recorder: recorder, sequence: 6, variant: "B", sessionSequence: 3, sessionSegment: 1,
	}
	probe.bindRuntimeIdentity(
		workflowContextCheckpointProbeVersion, workflowContextCheckpointProbeDigest)
	protocol := newPipelineOMPRPCProtocol(loopback.process())
	protocol.probe = probe
	version, digest := probe.runtimeIdentity()
	protocol.configureProbeCheckpointProfile(version, digest, true, pipelineOMPActiveOverlayChain(true))
	require.Equal(t, pipelineOMPActiveBoundedPreCheckpoints, protocol.preCheckpointLimit)
	ctx, cancel := context.WithTimeout(context.Background(), pipelineOMPActiveCheckpointDeadline)
	defer cancel()

	exchange := pipelineOMPActiveCheckpointExchange{loopback: loopback, readmissionAt: -1}
	exchange.performed, exchange.err = protocol.manualCompact(
		ctx, binding, pipelineOMPActiveCheckpointSessionID,
		func() (string, error) {
			exchange.readmissions++
			exchange.readmissionAt = len(loopback.acks)
			return "safe rehydrated prompt", nil
		})

	exchange.acks = loopback.acks
	require.NoError(t, recorder.Close())
	body, readErr := os.ReadFile(filepath.Join(directory, "probe.jsonl"))
	require.NoError(t, readErr)
	records := decodeWorkflowContextObserveProbeRecords(t, body)
	require.Len(t, records, 1, "one attempt emits exactly one compaction record")
	exchange.record = records[0]
	return exchange
}

// pipelineOMPActiveCheckpointBinding installs a bridge authority for one test
// and returns it, so the fixture's envelopes and the binding the protocol
// checks them against come from the same source.
func pipelineOMPActiveCheckpointBinding(t *testing.T) WorkflowContextBridgeBinding {
	t.Helper()
	for key, value := range map[string]string{
		"AUTOPUS_OMP_CONTEXT_BINDING_HASH": workflowContextRuntimeHash("checkpoint-binding"),
		"AUTOPUS_OMP_CONTEXT_OPTIONS_HASH": workflowContextRuntimeHash("checkpoint-options"),
		"AUTOPUS_OMP_CONTEXT_SESSION_HASH": workflowContextRuntimeHash("checkpoint-session"),
		"AUTOPUS_OMP_CONTEXT_NONCE_HASH":   workflowContextRuntimeHash("checkpoint-nonce"),
	} {
		t.Setenv(key, value)
	}
	binding := pipelineOMPActiveFixtureBinding()
	require.NotEmpty(t, binding.NonceHash, "the fixture and the protocol share one authority")
	return binding
}
