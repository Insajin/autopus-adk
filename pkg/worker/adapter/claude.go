package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/insajin/autopus-adk/pkg/worker/stream"
)

// ClaudeAdapter implements ProviderAdapter for Claude Code CLI.
type ClaudeAdapter struct{}

// NewClaudeAdapter creates a new ClaudeAdapter.
func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{}
}

// Name returns "claude".
func (a *ClaudeAdapter) Name() string { return "claude" }

// BuildCommand constructs the exec.Cmd for Claude Code with stream-json output.
func (a *ClaudeAdapter) BuildCommand(ctx context.Context, task TaskConfig) *exec.Cmd {
	sessionID := task.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("worker-sess-%s", task.TaskID)
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
	}

	// Only add --resume if sessionID is a valid UUID (claude requires UUID format).
	if len(sessionID) == 36 && sessionID[8] == '-' {
		args = append(args, "--resume", sessionID)
	}

	if task.MCPConfig != "" {
		args = append(args, "--mcp-config", task.MCPConfig)
	}

	if task.Model != "" {
		args = append(args, "--model", task.Model)
	}

	if task.ComputerUse {
		args = append(args, "--computer-use")
	}

	cmd := exec.CommandContext(ctx, ResolveBinary("claude"), args...)
	cmd.Dir = task.WorkDir

	// Build environment: inherit current env plus task-specific vars.
	env := cmd.Environ()
	env = append(env,
		fmt.Sprintf("AUTOPUS_TASK_ID=%s", task.TaskID),
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1",
	)
	for k, v := range task.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	return cmd
}

// ParseEvent parses a single line of stream-json output into a StreamEvent.
// Maps Claude's "tool_use" type to the canonical EventToolCall type.
func (a *ClaudeAdapter) ParseEvent(line []byte) (StreamEvent, error) {
	evt, err := stream.ParseLine(line)
	if err != nil {
		return StreamEvent{}, err
	}
	typ := evt.Type
	if typ == "tool_use" {
		typ = stream.EventToolCall
	}
	return StreamEvent{
		Type:    typ,
		Subtype: evt.Subtype,
		Data:    evt.Raw,
	}, nil
}

// ExtractResult extracts the final task result from a result event.
func (a *ClaudeAdapter) ExtractResult(event StreamEvent) TaskResult {
	var rd stream.ResultData
	if err := json.Unmarshal(event.Data, &rd); err != nil {
		return TaskResult{Output: string(event.Data)}
	}
	return TaskResult{
		CostUSD:    rd.CostUSD,
		DurationMS: rd.DurationMS,
		SessionID:  rd.SessionID,
		Output:     rd.Output,
	}
}
