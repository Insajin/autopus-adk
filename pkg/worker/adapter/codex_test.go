package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexAdapterName(t *testing.T) {
	a := NewCodexAdapter()
	assert.Equal(t, "codex", a.Name())
}

func TestCodexAdapterBuildCommand(t *testing.T) {
	a := NewCodexAdapter()
	task := TaskConfig{
		TaskID:  "task-c1",
		Prompt:  "fix the bug",
		WorkDir: "/tmp/codex-work",
		EnvVars: map[string]string{"FOO": "bar"},
	}

	cmd := a.BuildCommand(context.Background(), task)

	assert.Contains(t, cmd.Args, "exec")
	assert.Contains(t, cmd.Args, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotContains(t, cmd.Args, "--full-auto")
	assert.Contains(t, cmd.Args, "-")
	assert.NotContains(t, cmd.Args, "fix the bug")
	assert.Contains(t, cmd.Args, "--json")
	assert.NotContains(t, cmd.Args, "resume")
	assert.Equal(t, "/tmp/codex-work", cmd.Dir)

	envContains(t, cmd.Env, "AUTOPUS_TASK_ID=task-c1")
	envContains(t, cmd.Env, "FOO=bar")
}

func TestCodexAdapterBuildCommandWithSession(t *testing.T) {
	a := NewCodexAdapter()
	task := TaskConfig{
		TaskID:    "task-c2",
		SessionID: "my-session",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.NotContains(t, cmd.Args, "resume")
	assert.NotContains(t, cmd.Args, "my-session")
}

func TestCodexAdapterBuildCommandWithModel(t *testing.T) {
	a := NewCodexAdapter()
	task := TaskConfig{
		TaskID: "task-cm1",
		Model:  "o3",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.Contains(t, cmd.Args, "--dangerously-bypass-approvals-and-sandbox")
	assert.Contains(t, cmd.Args, "-m")
	assert.Contains(t, cmd.Args, "o3")
}

func TestCodexAdapterBuildCommand_OmitsOpenAIModelOverride(t *testing.T) {
	a := NewCodexAdapter()
	task := TaskConfig{
		TaskID: "task-cm2",
		Model:  "openai/gpt-5.4",
	}

	cmd := a.BuildCommand(context.Background(), task)
	assert.Contains(t, cmd.Args, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotContains(t, cmd.Args, "-m")
	assert.NotContains(t, cmd.Args, "openai/gpt-5.4")
}

func TestCodexAdapterParseEvent(t *testing.T) {
	a := NewCodexAdapter()

	line := []byte(`{"type":"result","output":"done"}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "result", evt.Type)
	assert.NotEmpty(t, evt.Data)
}

func TestCodexAdapterParseEventSplitsDottedTypes(t *testing.T) {
	a := NewCodexAdapter()

	line := []byte(`{"type":"turn.completed"}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "turn", evt.Type)
	assert.Equal(t, "completed", evt.Subtype)
}

func TestCodexAdapterParseEventPromotesAgentMessageToResult(t *testing.T) {
	a := NewCodexAdapter()

	line := []byte(`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"done via codex"}}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "result", evt.Type)

	result := a.ExtractResult(evt)
	assert.Equal(t, "done via codex", result.Output)
}

// REQ-BUDGET-02: Codex maps tool_call -> EventToolCall (already canonical).
func TestCodexAdapterParseEventToolCall(t *testing.T) {
	a := NewCodexAdapter()

	line := []byte(`{"type":"tool_call","name":"exec_cmd"}`)
	evt, err := a.ParseEvent(line)
	require.NoError(t, err)
	assert.Equal(t, "tool_call", evt.Type)
}

func TestCodexAdapterParseEventInvalid(t *testing.T) {
	a := NewCodexAdapter()
	_, err := a.ParseEvent([]byte("not json"))
	require.Error(t, err)
}

func TestCodexAdapterParseEventMissingType(t *testing.T) {
	a := NewCodexAdapter()
	_, err := a.ParseEvent([]byte(`{"output":"no type"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing type")
}

func TestCodexAdapterExtractResult(t *testing.T) {
	a := NewCodexAdapter()

	evt := StreamEvent{
		Type: "result",
		Data: []byte(`{"type":"result","output":"all done","cost_usd":0.12,"session_id":"s1"}`),
	}

	result := a.ExtractResult(evt)
	assert.InDelta(t, 0.12, result.CostUSD, 0.001)
	assert.Equal(t, "s1", result.SessionID)
	assert.Equal(t, "all done", result.Output)
}

func TestCodexAdapterExtractResultInvalidJSON(t *testing.T) {
	a := NewCodexAdapter()

	evt := StreamEvent{Data: []byte("bad")}
	result := a.ExtractResult(evt)
	assert.Equal(t, "bad", result.Output)
}
