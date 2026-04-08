package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

// GeminiAdapter implements ProviderAdapter for Gemini CLI.
type GeminiAdapter struct{}

// NewGeminiAdapter creates a new GeminiAdapter.
func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{}
}

// Name returns "gemini".
func (a *GeminiAdapter) Name() string { return "gemini" }

// BuildCommand constructs the exec.Cmd for Gemini CLI with stream-json output.
func (a *GeminiAdapter) BuildCommand(ctx context.Context, task TaskConfig) *exec.Cmd {
	sessionID := task.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("worker-sess-%s", task.TaskID)
	}

	args := []string{
		"--output-format", "stream-json",
		"--resume", sessionID,
	}

	if task.Prompt != "" {
		args = append(args, "-p", task.Prompt)
	}

	if task.Model != "" {
		args = append(args, "--model", task.Model)
	}

	if task.ComputerUse {
		slog.Warn("computer_use not supported by gemini provider, ignoring",
			"task_id", task.TaskID)
	}

	cmd := exec.CommandContext(ctx, ResolveBinary("gemini"), args...)
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

// geminiResultEvent is the JSON structure of a Gemini result line.
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
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return StreamEvent{}, fmt.Errorf("gemini parse: %w", err)
	}
	if raw.Type == "" {
		return StreamEvent{}, fmt.Errorf("gemini: missing type field")
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
