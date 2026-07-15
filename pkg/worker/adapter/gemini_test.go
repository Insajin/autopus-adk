package adapter

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiAdapterName(t *testing.T) {
	a := NewGeminiAdapter()
	assert.Equal(t, "gemini", a.Name())
}

func TestGeminiAdapterBuildCommand(t *testing.T) {
	a := NewGeminiAdapter()
	task := TaskConfig{
		TaskID:  "task-g1",
		Prompt:  "analyze code",
		WorkDir: "/tmp/gemini-work",
		EnvVars: map[string]string{"KEY": "val"},
	}

	cmd := a.BuildCommand(context.Background(), task)

	assert.Contains(t, cmd.Args, "--print")
	assert.NotContains(t, cmd.Args, "--output-format")
	assert.NotContains(t, cmd.Args, "stream-json")
	assert.NotContains(t, cmd.Args, "--resume")
	assert.NotContains(t, cmd.Args, "worker-sess-task-g1")
	assert.NotContains(t, cmd.Args, "-p")
	assert.NotContains(t, cmd.Args, "analyze code")
	assert.Equal(t, "/tmp/gemini-work", cmd.Dir)

	envContains(t, cmd.Env, "AUTOPUS_TASK_ID=task-g1")
	envContains(t, cmd.Env, "KEY=val")
}

func TestGeminiAdapterBuildCommandWithSession(t *testing.T) {
	a := NewGeminiAdapter()
	task := TaskConfig{
		TaskID:    "task-g2",
		SessionID: "gem-sess",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.NotContains(t, cmd.Args, "--resume")
	assert.NotContains(t, cmd.Args, "gem-sess")
}

func TestGeminiAdapterBuildCommandWithModel(t *testing.T) {
	a := NewGeminiAdapter()
	task := TaskConfig{
		TaskID: "task-gm1",
		Model:  "gemini-2.0-flash",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.NotContains(t, cmd.Args, "--model")
	assert.NotContains(t, cmd.Args, "gemini-2.0-flash")
}

func TestGeminiAdapterParseEvent(t *testing.T) {
	a := NewGeminiAdapter()

	line := []byte(`{"type":"result","output":"ok"}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "result", evt.Type)
}

// REQ-BUDGET-02: Gemini maps tool_call -> EventToolCall (already canonical).
func TestGeminiAdapterParseEventToolCall(t *testing.T) {
	a := NewGeminiAdapter()

	line := []byte(`{"type":"tool_call","name":"search"}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "tool_call", evt.Type)
}

func TestGeminiAdapterParseEventPlainText(t *testing.T) {
	a := NewGeminiAdapter()

	evt, err := a.ParseEvent([]byte("plain agy output"))
	require.NoError(t, err)
	assert.Equal(t, "result", evt.Type)

	result := a.ExtractResult(evt)
	assert.Equal(t, "plain agy output", result.Output)
}

func TestGeminiAdapterParseEventInvalid(t *testing.T) {
	a := NewGeminiAdapter()
	_, err := a.ParseEvent([]byte(`{"type":`))
	require.Error(t, err)
}

func TestGeminiAdapterExtractResult(t *testing.T) {
	a := NewGeminiAdapter()

	evt := StreamEvent{
		Type: "result",
		Data: []byte(`{"type":"result","output":"finished","cost_usd":0.03,"duration_ms":900,"session_id":"gs1"}`),
	}

	result := a.ExtractResult(evt)
	assert.InDelta(t, 0.03, result.CostUSD, 0.001)
	assert.Equal(t, int64(900), result.DurationMS)
	assert.Equal(t, "gs1", result.SessionID)
	assert.Equal(t, "finished", result.Output)
}

func TestGeminiAdapterExtractResultInvalidJSON(t *testing.T) {
	a := NewGeminiAdapter()

	evt := StreamEvent{Data: []byte("nope")}
	result := a.ExtractResult(evt)
	assert.Equal(t, "nope", result.Output)
}

func TestGeminiAdapterParseEvent_PlainTextMarksUsageUnavailable(t *testing.T) {
	a := NewGeminiAdapter()
	evt, err := a.ParseEvent([]byte("plain agy output"))
	require.NoError(t, err)

	result := a.ExtractResult(evt)
	require.Len(t, result.Usage, 1)
	assert.Equal(t, telemetry.UsageStatusUnavailable, result.Usage[0].UsageStatus)
	assert.Equal(t, telemetry.UsageReasonProviderAbsent, result.Usage[0].UnavailableReason)
}

func TestGeminiAdapterExtractResult_StructuredCostOnly(t *testing.T) {
	a := NewGeminiAdapter()
	evt, err := a.ParseEvent([]byte(`{"type":"result","run_id":"run-g","call_id":"call-g","output":"done","cost_usd":0.15}`))
	require.NoError(t, err)

	result := a.ExtractResult(evt)
	require.Len(t, result.Usage, 1)
	assert.Equal(t, telemetry.UsageStatusCostOnly, result.Usage[0].UsageStatus)
	assert.Nil(t, result.Usage[0].RawTotalTokens)
}

func TestGeminiAdapter_MultiplePlainEventsPreserveOutputAndUnavailableReceipts(t *testing.T) {
	a := NewGeminiAdapter()
	firstEvent, err := a.ParseEvent([]byte("line one"))
	require.NoError(t, err)
	secondEvent, err := a.ParseEvent([]byte("line two"))
	require.NoError(t, err)

	got := MergeSequentialResult("gemini", a.ExtractResult(firstEvent), true, a.ExtractResult(secondEvent))

	assert.Equal(t, "line one\nline two", got.Output)
	require.Len(t, got.Usage, 2, "unbound receipts must remain ordered until worker identity binding")
}
