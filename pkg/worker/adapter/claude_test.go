package adapter

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeAdapterName(t *testing.T) {
	a := NewClaudeAdapter()
	assert.Equal(t, "claude", a.Name())
}

func TestClaudeAdapterBuildCommand(t *testing.T) {
	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID:    "task-123",
		Prompt:    "do something",
		MCPConfig: "/tmp/worker-mcp.json",
		WorkDir:   "/tmp/work",
		EnvVars:   map[string]string{"EXTRA": "val"},
	}

	cmd := a.BuildCommand(context.Background(), task)

	assert.Equal(t, "claude", cmd.Path[len(cmd.Path)-len("claude"):])
	assert.Contains(t, cmd.Args, "--print")
	assert.Contains(t, cmd.Args, "--output-format")
	assert.Contains(t, cmd.Args, "stream-json")
	assert.Contains(t, cmd.Args, "--verbose")
	assert.NotContains(t, cmd.Args, "--bare")
	assert.Contains(t, cmd.Args, "--resume")
	assert.Contains(t, cmd.Args, "worker-sess-task-123")
	assert.Contains(t, cmd.Args, "--mcp-config")
	assert.Contains(t, cmd.Args, "/tmp/worker-mcp.json")
	assert.Equal(t, "/tmp/work", cmd.Dir)

	envContains(t, cmd.Env, "AUTOPUS_TASK_ID=task-123")
	envContains(t, cmd.Env, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	envContains(t, cmd.Env, "EXTRA=val")
}

func TestClaudeAdapterBuildCommandWithSessionID(t *testing.T) {
	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID:    "task-456",
		SessionID: "custom-session",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.Contains(t, cmd.Args, "custom-session")
}

func TestClaudeAdapterBuildCommandNoMCPConfig(t *testing.T) {
	a := NewClaudeAdapter()
	task := TaskConfig{TaskID: "task-789"}

	cmd := a.BuildCommand(context.Background(), task)
	assert.False(t, slices.Contains(cmd.Args, "--mcp-config"))
}

func TestClaudeAdapterBuildCommandWithModel(t *testing.T) {
	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID: "task-model-1",
		Model:  "claude-3-5-haiku-20241022",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.Contains(t, cmd.Args, "--model")
	assert.Contains(t, cmd.Args, "claude-3-5-haiku-20241022")
}

func TestClaudeAdapterBuildCommandEmptyModel(t *testing.T) {
	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID: "task-model-2",
		Model:  "",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.False(t, slices.Contains(cmd.Args, "--model"))
}

func TestClaudeAdapterParseEvent(t *testing.T) {
	a := NewClaudeAdapter()

	line := []byte(`{"type":"system.init","mcp_servers":["fs"]}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "system", evt.Type)
	assert.Equal(t, "init", evt.Subtype)
	assert.NotEmpty(t, evt.Data)
}

// REQ-BUDGET-02: Claude maps tool_use -> EventToolCall.
func TestClaudeAdapterParseEventToolUse(t *testing.T) {
	a := NewClaudeAdapter()

	line := []byte(`{"type":"tool_use","name":"read_file","input":{"path":"foo.go"}}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "tool_call", evt.Type, "tool_use should be mapped to tool_call")
}

func TestClaudeAdapterParseEventInvalid(t *testing.T) {
	a := NewClaudeAdapter()
	_, err := a.ParseEvent([]byte("not json"))
	require.Error(t, err)
}

func TestClaudeAdapterExtractResult(t *testing.T) {
	a := NewClaudeAdapter()

	evt := StreamEvent{
		Type:    "result",
		Subtype: "",
		Data:    []byte(`{"cost_usd":0.05,"duration_ms":1200,"session_id":"sess-1","output":"done"}`),
	}

	result := a.ExtractResult(evt)
	assert.InDelta(t, 0.05, result.CostUSD, 0.001)
	assert.Equal(t, int64(1200), result.DurationMS)
	assert.Equal(t, "sess-1", result.SessionID)
	assert.Equal(t, "done", result.Output)
}

func TestClaudeAdapterExtractResultCurrentClaudeResultField(t *testing.T) {
	a := NewClaudeAdapter()

	evt := StreamEvent{
		Type: "result",
		Data: []byte(`{"cost_usd":0.02,"duration_ms":900,"session_id":"sess-2","result":"done from result field"}`),
	}

	result := a.ExtractResult(evt)
	assert.Equal(t, "done from result field", result.Output)
	assert.False(t, result.IsError)
}

func TestClaudeAdapterExtractResultError(t *testing.T) {
	a := NewClaudeAdapter()

	evt := StreamEvent{
		Type: "result",
		Data: []byte(`{"is_error":true,"api_error_status":401,"result":"Failed to authenticate. API Error: 401 Invalid authentication credentials","session_id":"sess-err"}`),
	}

	result := a.ExtractResult(evt)
	assert.True(t, result.IsError)
	assert.Equal(t, "Failed to authenticate. API Error: 401 Invalid authentication credentials", result.Output)
	assert.Equal(t, "claude api error 401: Failed to authenticate. API Error: 401 Invalid authentication credentials", result.Error)
	assert.Equal(t, "sess-err", result.SessionID)
}

func TestClaudeAdapterExtractResultInvalidJSON(t *testing.T) {
	a := NewClaudeAdapter()

	evt := StreamEvent{Data: []byte("bad json")}
	result := a.ExtractResult(evt)
	assert.Equal(t, "bad json", result.Output)
}

func TestClaudeAdapterExtractResult_AnthropicUsagePreservesCacheBreakdown(t *testing.T) {
	a := NewClaudeAdapter()
	evt, err := a.ParseEvent([]byte(`{"type":"result","run_id":"run-1","call_id":"call-1","task_id":"task-1","attempt":2,"phase":"implementation","role":"executor","effort":"high","model":"claude-opus-4-6","result":"done","usage":{"input_tokens":600,"cache_creation_input_tokens":100,"cache_read_input_tokens":300,"output_tokens":200}}`))
	require.NoError(t, err)

	result := a.ExtractResult(evt)
	require.Len(t, result.Usage, 1)
	usage := result.Usage[0]
	assert.Equal(t, telemetry.UsageStatusActual, usage.UsageStatus)
	assert.Equal(t, "run-1", usage.RunID)
	assert.Equal(t, "call-1", usage.CallID)
	assert.Equal(t, int64(1000), *usage.InputTokensTotal)
	assert.Equal(t, int64(1200), *usage.RawTotalTokens)
	assert.Equal(t, int64(100), *usage.CacheCreationInputTokens)
	assert.Equal(t, int64(300), *usage.CacheReadInputTokens)

	payload, marshalErr := json.Marshal(usage)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(payload), "done")
}

func TestClaudeAdapterExtractResult_CostOnlyDoesNotInventTokens(t *testing.T) {
	a := NewClaudeAdapter()
	evt, err := a.ParseEvent([]byte(`{"type":"result","run_id":"run-2","call_id":"call-2","result":"done","total_cost_usd":0.25}`))
	require.NoError(t, err)

	result := a.ExtractResult(evt)
	require.Len(t, result.Usage, 1)
	assert.InDelta(t, 0.25, result.CostUSD, 0.001)
	assert.Equal(t, telemetry.UsageStatusCostOnly, result.Usage[0].UsageStatus)
	assert.Nil(t, result.Usage[0].InputTokensTotal)
	assert.Nil(t, result.Usage[0].RawTotalTokens)
}

// envContains checks that the env slice contains the expected key=value pair.
func envContains(t *testing.T, env []string, expected string) {
	t.Helper()
	if !slices.Contains(env, expected) {
		t.Errorf("env does not contain %q", expected)
	}
}
