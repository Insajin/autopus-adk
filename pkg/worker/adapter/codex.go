package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

// CodexAdapter implements ProviderAdapter for OpenAI Codex CLI.
type CodexAdapter struct{}

// NewCodexAdapter creates a new CodexAdapter.
func NewCodexAdapter() *CodexAdapter {
	return &CodexAdapter{}
}

// Name returns "codex".
func (a *CodexAdapter) Name() string { return "codex" }

// BuildCommand constructs the exec.Cmd for Codex CLI.
func (a *CodexAdapter) BuildCommand(ctx context.Context, task TaskConfig) *exec.Cmd {
	sessionID := task.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("worker-sess-%s", task.TaskID)
	}

	args := []string{"exec"}
	if task.Prompt != "" {
		args = append(args, task.Prompt)
	}
	args = append(args, "--json", "resume", sessionID)

	if task.Model != "" {
		args = append(args, "-m", task.Model)
	}

	if task.ComputerUse {
		slog.Warn("computer_use not supported by codex provider, ignoring",
			"task_id", task.TaskID)
	}

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = task.WorkDir

	// Build environment: inherit current env plus task-specific vars.
	env := cmd.Environ()
	env = append(env, fmt.Sprintf("AUTOPUS_TASK_ID=%s", task.TaskID))
	for k, v := range task.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	return cmd
}

// codexResultEvent is the JSON structure of a Codex result line.
type codexResultEvent struct {
	Type      string  `json:"type"`
	Output    string  `json:"output,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
}

// ParseEvent parses a single line of Codex JSON output into a StreamEvent.
// Maps Codex's "tool_call" type to the canonical EventToolCall type.
func (a *CodexAdapter) ParseEvent(line []byte) (StreamEvent, error) {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return StreamEvent{}, fmt.Errorf("codex parse: %w", err)
	}
	if raw.Type == "" {
		return StreamEvent{}, fmt.Errorf("codex: missing type field")
	}

	typ := raw.Type
	if typ == "tool_call" {
		typ = "tool_call" // already canonical
	}

	return StreamEvent{
		Type: typ,
		Data: json.RawMessage(append([]byte(nil), line...)),
	}, nil
}

// ExtractResult extracts the final task result from a Codex result event.
func (a *CodexAdapter) ExtractResult(event StreamEvent) TaskResult {
	var re codexResultEvent
	if err := json.Unmarshal(event.Data, &re); err != nil {
		return TaskResult{Output: string(event.Data)}
	}
	return TaskResult{
		CostUSD:   re.CostUSD,
		SessionID: re.SessionID,
		Output:    re.Output,
	}
}
