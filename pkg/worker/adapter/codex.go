package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
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
	// Worker tasks already run inside an isolated worktree with platform-level
	// policy enforcement. Git push requires outbound network access, which the
	// Codex workspace sandbox blocks even in --full-auto mode.
	args := []string{"exec", "--dangerously-bypass-approvals-and-sandbox"}
	if task.Prompt != "" {
		// Read the sensitive task prompt from stdin instead of exposing it
		// in the process argv where other local processes can inspect it.
		args = append(args, "-")
	}
	args = append(args, "--json")

	if task.Model != "" && !strings.HasPrefix(task.Model, "openai/") {
		args = append(args, "-m", task.Model)
	} else if strings.HasPrefix(task.Model, "openai/") {
		slog.Warn("openai/* model not supported by codex CLI account, omitting explicit model override",
			"task_id", task.TaskID,
			"model", task.Model)
	}

	if task.ComputerUse {
		slog.Warn("computer_use not supported by codex provider, ignoring",
			"task_id", task.TaskID)
	}

	cmd := exec.CommandContext(ctx, ResolveBinary("codex"), args...)
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

	if raw.Type == "item.completed" {
		var item struct {
			Item struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"item"`
		}
		if err := json.Unmarshal(line, &item); err != nil {
			return StreamEvent{}, fmt.Errorf("codex parse item.completed: %w", err)
		}
		if item.Item.Type == "agent_message" || item.Item.Type == "assistant_message" {
			synthetic, err := json.Marshal(codexResultEvent{
				Type:   "result",
				Output: item.Item.Text,
			})
			if err != nil {
				return StreamEvent{}, fmt.Errorf("codex synthesize result: %w", err)
			}
			return StreamEvent{
				Type: "result",
				Data: synthetic,
			}, nil
		}
	}

	typ, subtype := splitEventType(raw.Type)

	return StreamEvent{
		Type:    typ,
		Subtype: subtype,
		Data:    json.RawMessage(append([]byte(nil), line...)),
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
