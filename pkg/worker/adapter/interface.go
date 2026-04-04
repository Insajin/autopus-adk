package adapter

import (
	"context"
	"encoding/json"
	"os/exec"
)

// ProviderAdapter abstracts CLI provider execution.
type ProviderAdapter interface {
	// Name returns the provider name (e.g., "claude", "codex", "gemini").
	Name() string
	// BuildCommand constructs the exec.Cmd for this provider with the given task context.
	BuildCommand(ctx context.Context, task TaskConfig) *exec.Cmd
	// ParseEvent parses a single line of stream output into a typed event.
	ParseEvent(line []byte) (StreamEvent, error)
	// ExtractResult extracts the final task result from a result event.
	ExtractResult(event StreamEvent) TaskResult
}

// TaskConfig holds the configuration for a subprocess execution.
type TaskConfig struct {
	TaskID    string            // unique task identifier
	SessionID string            // for --resume
	Prompt    string            // delivered via stdin
	MCPConfig string            // path to worker-mcp.json
	WorkDir   string            // working directory for subprocess
	EnvVars   map[string]string // additional env vars
	Model       string            // provider-specific model override
	ComputerUse bool              // enable computer use for this task
}

// StreamEvent represents a parsed event from subprocess output.
type StreamEvent struct {
	Type    string          // e.g., "system.init", "result"
	Subtype string          // e.g., "init", "task_started"
	Data    json.RawMessage // raw event data
}

// TaskResult holds the extracted result from a completed subprocess.
type TaskResult struct {
	CostUSD    float64
	DurationMS int64
	SessionID  string
	Output     string
	Artifacts  []Artifact
}

// Artifact holds a single output artifact from task execution.
type Artifact struct {
	Name     string
	MimeType string
	Data     string
}
