package adapter

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
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
	TaskID          string            // unique task identifier
	RunID           string            // stable telemetry run identity
	CallID          string            // stable telemetry provider-call identity
	Attempt         int               // retry attempt for this provider call
	Phase           string            // pipeline phase
	Role            string            // worker role
	Effort          string            // provider reasoning effort
	ProviderVersion string            // trusted provider runtime version, when observed
	ModelVersion    string            // trusted model version, when observed
	RiskPolicy      string            // trusted risk policy identity
	CacheStratum    string            // trusted cache comparison stratum
	ConfigHash      string            // trusted effective configuration hash
	SessionID       string            // for --resume
	Prompt          string            // delivered via stdin
	RequiredContext string            // verified, non-compressible context prepended at dispatch
	PersistentTask  string            // task contract reattached after every pipeline transition
	ResolveContext  bool              // resolve required context after the final worktree is assigned
	ContextSpecID   string            // local SPEC ID used by required-context delivery
	ContextRefs     []string          // supervisor-held task-specific required references
	MCPConfig       string            // path to worker-mcp.json
	WorkDir         string            // working directory for subprocess
	EnvVars         map[string]string // additional env vars
	Model           string            // provider-specific model override
	ComputerUse     bool              // enable computer use for this task
	Codex           CodexTaskOptions  // evidence-safe Codex execution controls
	EvidenceMode    bool              // strict evidence stream semantics are explicitly requested
}

// CodexSandboxMode is the allowlisted sandbox policy for evidence runs.
type CodexSandboxMode string

const (
	CodexSandboxReadOnly       CodexSandboxMode = "read-only"
	CodexSandboxWorkspaceWrite CodexSandboxMode = "workspace-write"
)

// CodexTaskOptions isolates evidence-only controls from legacy worker defaults.
// Its zero value intentionally preserves the historical externally-sandboxed path.
type CodexTaskOptions struct {
	Sandbox          CodexSandboxMode
	Ephemeral        bool
	IgnoreUserConfig bool
	IgnoreRules      bool
	SkipGitRepoCheck bool
	OutputSchema     string
	ZeroToolMode     bool
	RawTokenBudget   int64
}

// StreamEvent represents a parsed event from subprocess output.
type StreamEvent struct {
	Type      string          // e.g., "system.init", "result"
	Subtype   string          // e.g., "init", "task_started"
	Data      json.RawMessage // raw event data
	Usage     []telemetry.UsageEnvelope
	ToolCalls int
}

// TaskResult holds the extracted result from a completed subprocess.
type TaskResult struct {
	CostUSD    float64
	DurationMS int64
	SessionID  string
	Output     string
	IsError    bool
	Error      string
	Artifacts  []Artifact
	Usage      []telemetry.UsageEnvelope
	ToolCalls  int
}

type providerUsageIdentity struct {
	RunID   string `json:"run_id"`
	CallID  string `json:"call_id"`
	TaskID  string `json:"task_id"`
	Attempt int    `json:"attempt"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Phase   string `json:"phase"`
	Role    string `json:"role"`
}

// normalizeProviderUsage preserves provider semantics even before the worker
// binds its stable TaskConfig identity. Unbound receipts must not be aggregated.
func normalizeProviderUsage(input telemetry.UsageInput) telemetry.UsageEnvelope {
	bound := input.RunID != "" && input.CallID != ""
	if !bound {
		input.RunID = "adapter-unbound-run"
		input.CallID = "adapter-unbound-call"
	}
	envelope := telemetry.NormalizeUsage(input)
	if !bound {
		envelope.RunID = ""
		envelope.CallID = ""
	}
	return envelope
}

func applyUsageIdentity(input *telemetry.UsageInput, identity providerUsageIdentity, provider, schema string) {
	input.RunID = identity.RunID
	input.CallID = identity.CallID
	input.TaskID = identity.TaskID
	input.Attempt = identity.Attempt
	input.Provider = provider
	input.Model = identity.Model
	input.Effort = identity.Effort
	input.Phase = identity.Phase
	input.Role = identity.Role
	input.Source = telemetry.UsageSourceProvider
	input.SourceSchema = schema
}

// Artifact holds a single output artifact from task execution.
type Artifact struct {
	Name     string
	MimeType string
	Data     string
}

func splitEventType(full string) (string, string) {
	if before, after, ok := strings.Cut(full, "."); ok {
		return before, after
	}
	return full, ""
}
