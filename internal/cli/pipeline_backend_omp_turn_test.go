package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shapes captured from omp 18.1.10 `agent_end` frames: a normal stop carries
// the executed provider/model; a provider rejection carries stopReason=error
// with errorStatus/errorMessage and no text.
func TestSettlePipelineOMPTurn(t *testing.T) {
	t.Parallel()
	stop := json.RawMessage(`[
		{"role":"user","content":[{"type":"text","text":"Reply OK."}]},
		{"role":"assistant","content":[{"type":"text","text":"OK."}],"provider":"anthropic","model":"claude-fable-5-1","stopReason":"stop"}
	]`)
	identity, err := settlePipelineOMPTurn(stop)
	require.NoError(t, err)
	assert.Equal(t, pipelineOMPTurnIdentity{Provider: "anthropic", Model: "claude-fable-5-1"}, identity)

	failed := json.RawMessage(`[
		{"role":"user","content":[]},
		{"role":"assistant","content":[],"provider":"anthropic","model":"claude-mythos-5","stopReason":"error","errorStatus":404,"errorMessage":"404 {\"type\":\"error\",\"error\":{\"type\":\"not_found_error\",\"message\":\"model: claude-mythos-5\"}}"}
	]`)
	identity, err = settlePipelineOMPTurn(failed)
	var turnErr *pipelineOMPTurnError
	require.ErrorAs(t, err, &turnErr)
	assert.Equal(t, 404, turnErr.Status)
	assert.False(t, turnErr.Transient())
	assert.Contains(t, err.Error(), "provider error status 404: 404 {")
	assert.Equal(t, "claude-mythos-5", identity.Model)

	overloaded := &pipelineOMPTurnError{Status: 529, Message: "overloaded_error"}
	assert.True(t, overloaded.Transient())
	assert.True(t, (&pipelineOMPTurnError{Status: 429}).Transient())
	assert.False(t, (&pipelineOMPTurnError{Status: 401}).Transient())
	assert.False(t, (&pipelineOMPTurnError{}).Transient(), "an error without a status is not retried blindly")

	_, err = settlePipelineOMPTurn(json.RawMessage(`[{"role":"assistant","content":[],"stopReason":"aborted"}]`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aborted")
	assert.False(t, errors.As(err, &turnErr))

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`[]`)} {
		identity, err = settlePipelineOMPTurn(raw)
		require.NoError(t, err, "older runtimes omit messages")
		assert.Equal(t, pipelineOMPTurnIdentity{}, identity)
	}
	_, err = settlePipelineOMPTurn(json.RawMessage(`{"not":"a list"}`))
	require.Error(t, err)
}
