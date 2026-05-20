package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// GeminiAdapter implements ProviderAdapter for the Gemini-family provider via AGY CLI.
type GeminiAdapter struct{}

// NewGeminiAdapter creates a new GeminiAdapter.
func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{}
}

// Name returns "gemini".
func (a *GeminiAdapter) Name() string { return "gemini" }

// BuildCommand constructs the exec.Cmd for the Antigravity AGY CLI.
func (a *GeminiAdapter) BuildCommand(ctx context.Context, task TaskConfig) *exec.Cmd {
	args := []string{"--print"}

	if task.Model != "" {
		slog.Warn("model override not supported by antigravity cli provider, using agy default model",
			"task_id", task.TaskID,
			"model", task.Model)
	}

	if task.SessionID != "" {
		slog.Info("session resume not supported by antigravity cli provider, starting a new conversation",
			"task_id", task.TaskID,
			"session_id", task.SessionID)
	}

	if task.ComputerUse {
		slog.Warn("computer_use not supported by gemini provider, ignoring",
			"task_id", task.TaskID)
	}

	cmd := exec.CommandContext(ctx, ResolveBinary("agy"), args...)
	cmd.Dir = task.WorkDir

	// Build environment: inherit current env plus task-specific vars.
	env := cmd.Environ()
	env = append(env, fmt.Sprintf("AUTOPUS_TASK_ID=%s", task.TaskID))
	for k, v := range task.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = EnvironWithToolPath(env)

	return cmd
}

// geminiResultEvent is the JSON structure of a Gemini-family result line.
type geminiResultEvent struct {
	Type       string  `json:"type"`
	Output     string  `json:"output,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
	DurationMS int64   `json:"duration_ms,omitempty"`
	SessionID  string  `json:"session_id,omitempty"`
}

// ParseEvent parses a single line of Gemini JSON output into a StreamEvent.
// Maps Gemini's "tool_call" type to the canonical EventToolCall type.
func (a *GeminiAdapter) ParseEvent(line []byte) (StreamEvent, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return StreamEvent{}, fmt.Errorf("gemini: empty output line")
	}
	if !strings.HasPrefix(trimmed, "{") {
		synthetic, err := json.Marshal(geminiResultEvent{
			Type:   "result",
			Output: trimmed,
		})
		if err != nil {
			return StreamEvent{}, fmt.Errorf("gemini synthesize result: %w", err)
		}
		return StreamEvent{
			Type: "result",
			Data: synthetic,
		}, nil
	}

	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return StreamEvent{}, fmt.Errorf("gemini parse: %w", err)
	}
	if raw.Type == "" {
		return StreamEvent{}, fmt.Errorf("gemini: missing type field")
	}

	typ, subtype := splitEventType(raw.Type)

	return StreamEvent{
		Type:    typ,
		Subtype: subtype,
		Data:    json.RawMessage(append([]byte(nil), trimmed...)),
	}, nil
}

// ExtractResult extracts the final task result from a Gemini result event.
func (a *GeminiAdapter) ExtractResult(event StreamEvent) TaskResult {
	var re geminiResultEvent
	if err := json.Unmarshal(event.Data, &re); err != nil {
		return TaskResult{Output: string(event.Data)}
	}
	return TaskResult{
		CostUSD:    re.CostUSD,
		DurationMS: re.DurationMS,
		SessionID:  re.SessionID,
		Output:     re.Output,
	}
}
