package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodePipelineOMPPhysicalFrame_RPCChunkPreservesResponsePayload(t *testing.T) {
	t.Parallel()
	logical := []byte(`{"id":"pipeline-7","type":"response","command":"get_last_assistant_text","success":true,"data":{"text":"hello π"}}`)
	split := len(logical) / 2
	first := pipelineOMPChunkFrame(t, "chunk-1", 0, 2, len(logical), logical[:split])
	second := pipelineOMPChunkFrame(t, "chunk-1", 1, 2, len(logical), logical[split:])

	frame, sequence, complete, err := decodePipelineOMPPhysicalFrame(first, nil)
	require.NoError(t, err)
	assert.False(t, complete)
	assert.Empty(t, frame.Type)
	require.NotNil(t, sequence)

	frame, sequence, complete, err = decodePipelineOMPPhysicalFrame(second, sequence)
	require.NoError(t, err)
	assert.True(t, complete)
	assert.Nil(t, sequence)
	assert.Equal(t, "pipeline-7", frame.ID)
	assert.Equal(t, "get_last_assistant_text", frame.Command)
	assert.True(t, frame.Success)
	assert.JSONEq(t, `{"text":"hello π"}`, string(frame.Data))
}

func TestDecodePipelineOMPPhysicalFrame_RejectsMalformedSequences(t *testing.T) {
	t.Parallel()
	logical := []byte(`{"type":"response","success":true}`)
	half := len(logical) / 2
	chunk := func(id string, index, count, length int, data []byte) []byte {
		return pipelineOMPChunkFrame(t, id, index, count, length, data)
	}
	tests := []struct {
		name, want string
		frames     [][]byte
	}{
		{"interleaved", "interleaved", [][]byte{chunk("a", 0, 2, len(logical), logical[:half]), chunk("b", 1, 2, len(logical), logical[half:])}},
		{"index", "start at zero", [][]byte{chunk("a", 1, 2, len(logical), logical[:half])}},
		{"count", "interleaved", [][]byte{chunk("a", 0, 2, len(logical), logical[:half]), chunk("a", 1, 3, len(logical), logical[half:])}},
		{"byte length", "interleaved", [][]byte{chunk("a", 0, 2, len(logical), logical[:half]), chunk("a", 1, 2, len(logical)+1, logical[half:])}},
		{"base64", "chunk data", [][]byte{[]byte(`{"type":"rpc_chunk","chunkId":"a","index":0,"count":1,"byteLength":1,"data":"%%%"}`)}},
		{"physical UTF-8", "not UTF-8", [][]byte{append([]byte(`{"type":"ready","bad":"`), 0xff)}},
		{"duplicate key", "invalid OMP pipeline RPC frame", [][]byte{[]byte(`{"type":"response","type":"ready"}`)}},
		{"trailing JSON", "invalid OMP pipeline RPC frame", [][]byte{[]byte(`{"type":"ready"} {}`)}},
		{"physical oversize", "exceeds limit", [][]byte{bytes.Repeat([]byte{'x'}, pipelineOMPMaxPhysicalFrame+1)}},
		{"declared oversize", "metadata", [][]byte{chunk("a", 0, 1, pipelineOMPMaxReassembledFrame+1, logical)}},
		{"reassembled UTF-8", "reassembled frame", [][]byte{chunk("a", 0, 1, 1, []byte{0xff})}},
		{"reassembled duplicate", "reassembled JSON", [][]byte{chunk("a", 0, 1, 34, []byte(`{"type":"ready","type":"response"}`))}},
		{"reassembled trailing", "reassembled JSON", [][]byte{chunk("a", 0, 1, 18, []byte(`{"type":"ready"}{}`))}},
		{"actual byte length", "reassembled frame", [][]byte{chunk("a", 0, 1, len(logical)+1, logical)}},
		{"interrupted", "interrupted", [][]byte{chunk("a", 0, 2, len(logical), logical[:half]), []byte(`{"type":"ready"}`)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sequence *pipelineOMPChunkSequence
			var err error
			for _, physical := range test.frames {
				_, sequence, _, err = decodePipelineOMPPhysicalFrame(physical, sequence)
				if err != nil {
					break
				}
			}
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestReadPipelineOMPFrames_InterruptedAndCanceledReadersComplete(t *testing.T) {
	t.Parallel()
	t.Run("interrupted", func(t *testing.T) {
		logical := []byte(`{"type":"ready"}`)
		line := append(pipelineOMPChunkFrame(t, "a", 0, 2, len(logical), logical[:2]), '\n')
		frames, done := readPipelineOMPFrames(context.Background(), bytes.NewReader(line))
		require.ErrorContains(t, pipelineOMPDoneWithin(t, done), "interrupted")
		_, open := <-frames
		assert.False(t, open)
	})
	t.Run("canceled blocked send", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		frames, done := readPipelineOMPFrames(ctx, strings.NewReader(`{"type":"ready"}`+"\n"))
		cancel()
		require.ErrorIs(t, pipelineOMPDoneWithin(t, done), context.Canceled)
		_, open := <-frames
		assert.False(t, open)
	})
}

func TestPipelineOMPRPCProtocol_PromptLifecycleOrderingAndFailures(t *testing.T) {
	t.Parallel()
	success := pipelineOMPRPCFrame{ID: "pipeline-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`{"agentInvoked":true}`)}
	t.Run("lifecycle before response", func(t *testing.T) {
		protocol, sent := pipelineOMPProtocolFixture([]pipelineOMPRPCFrame{
			{Type: "agent_start"}, {Type: "agent_end"}, success,
		})
		data, err := protocol.call(context.Background(), pipelineOMPRPCCommand{Type: "prompt", Message: "phase"}, true)
		require.NoError(t, err)
		assert.JSONEq(t, `{"agentInvoked":true}`, string(data))
		assert.Contains(t, sent.String(), `"message":"phase"`)
	})
	t.Run("retry cycles settle on the terminal end", func(t *testing.T) {
		nonTerminal := false
		accepted := map[string][]pipelineOMPRPCFrame{
			"held agent end": {
				success, {Type: "agent_start"}, {Type: "agent_start"}, {Type: "agent_end"},
			},
			"explicit non-terminal end": {
				success, {Type: "agent_start"}, {Type: "agent_end", IsTerminal: &nonTerminal},
				{Type: "agent_start"}, {Type: "agent_end"},
			},
		}
		for name, frames := range accepted {
			t.Run(name, func(t *testing.T) {
				protocol, _ := pipelineOMPProtocolFixture(frames)
				data, err := protocol.call(context.Background(), pipelineOMPRPCCommand{Type: "prompt", Message: "phase"}, true)
				require.NoError(t, err)
				assert.JSONEq(t, `{"agentInvoked":true}`, string(data))
			})
		}
	})
	// omp 18.1.x acknowledges a prompt with a bare success and proves the run
	// only through the lifecycle frames.
	t.Run("bare acknowledgement settles on lifecycle", func(t *testing.T) {
		protocol, _ := pipelineOMPProtocolFixture([]pipelineOMPRPCFrame{
			{ID: "pipeline-1", Type: "response", Command: "prompt", Success: true},
			{Type: "agent_start"}, {Type: "agent_end"},
		})
		data, err := protocol.call(context.Background(), pipelineOMPRPCCommand{Type: "prompt", Message: "phase"}, true)
		require.NoError(t, err)
		assert.Empty(t, data)
	})
	tests := []struct {
		name, want string
		frames     []pipelineOMPRPCFrame
	}{
		{"stale agent end", "out of order", []pipelineOMPRPCFrame{{Type: "agent_end"}, success}},
		{"start after terminal end", "ambiguous", []pipelineOMPRPCFrame{{Type: "agent_start"}, {Type: "agent_end"}, {Type: "agent_start"}, success}},
		{"missing invocation proof", "malformed", []pipelineOMPRPCFrame{{Type: "agent_start"}, {Type: "agent_end"}, {ID: "pipeline-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`[]`)}}},
		{"response says not invoked", "without invoking", []pipelineOMPRPCFrame{{ID: "pipeline-1", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`{"agentInvoked":false}`)}}},
		{"prompt result says not invoked", "did not enter", []pipelineOMPRPCFrame{{ID: "pipeline-1", Type: "prompt_result", AgentInvoked: boolPointer(false)}}},
		{"late same-id failure", "late failure", []pipelineOMPRPCFrame{success, {ID: "pipeline-1", Type: "response", Command: "prompt", Error: "late failure"}}},
		{"failure after agent end", "after lifecycle", []pipelineOMPRPCFrame{{Type: "agent_start"}, {Type: "agent_end"}, {ID: "pipeline-1", Type: "response", Command: "prompt", Error: "after lifecycle"}}},
		{"extension error", "extension error", []pipelineOMPRPCFrame{{Type: "extension_error", Error: "bridge failed"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			protocol, _ := pipelineOMPProtocolFixture(test.frames)
			_, err := protocol.call(context.Background(), pipelineOMPRPCCommand{Type: "prompt", Message: "phase"}, true)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestPipelineOMPRPCProtocol_StaleLifecycleCannotBypassIdleReadback(t *testing.T) {
	protocol, _ := pipelineOMPProtocolFixture([]pipelineOMPRPCFrame{
		{ID: "pipeline-1", Type: "response", Command: "get_state", Success: true, Data: json.RawMessage(`{"sessionId":"session","isStreaming":false,"isCompacting":false,"messageCount":2,"queuedMessageCount":0}`)},
		{Type: "agent_start"}, {Type: "agent_end"},
		{ID: "pipeline-2", Type: "response", Command: "prompt", Success: true, Data: json.RawMessage(`{"agentInvoked":true}`)},
		{ID: "pipeline-3", Type: "response", Command: "get_state", Success: true, Data: json.RawMessage(`{"sessionId":"session","isStreaming":false,"isCompacting":false,"messageCount":2,"queuedMessageCount":0}`)},
	})

	output, err := protocol.execute(context.Background(), "", "phase")

	require.ErrorContains(t, err, "not bound to a completed turn")
	assert.Empty(t, output)
}

func pipelineOMPChunkFrame(t *testing.T, id string, index, count, length int, data []byte) []byte {
	t.Helper()
	body, err := json.Marshal(pipelineOMPRPCChunk{Type: "rpc_chunk", ChunkID: id, Index: index, Count: count, ByteLength: length, Data: base64.StdEncoding.EncodeToString(data)})
	require.NoError(t, err)
	return body
}

func pipelineOMPDoneWithin(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("OMP frame reader did not complete within one second")
		return nil
	}
}

type pipelineOMPTestWriteCloser struct{ bytes.Buffer }

func (writer *pipelineOMPTestWriteCloser) Close() error { return nil }

func pipelineOMPProtocolFixture(frames []pipelineOMPRPCFrame) (*pipelineOMPRPCProtocol, *pipelineOMPTestWriteCloser) {
	stream := make(chan pipelineOMPRPCFrame, len(frames))
	for _, frame := range frames {
		stream <- frame
	}
	close(stream)
	written := &pipelineOMPTestWriteCloser{}
	return newPipelineOMPRPCProtocol(&pipelineOMPProcess{stdin: written, frames: stream}), written
}

func boolPointer(value bool) *bool { return &value }
