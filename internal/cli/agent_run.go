package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validTaskID matches safe task IDs: alphanumeric, hyphens, underscores only.
var validTaskID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// taskContext represents the input context for an agent task.
type taskContext struct {
	TaskID                string           `yaml:"task_id"`
	Description           string           `yaml:"description"`
	Provider              string           `yaml:"provider,omitempty"`
	Model                 string           `yaml:"model,omitempty"`
	Effort                string           `yaml:"effort,omitempty"`
	SpecID                string           `yaml:"spec_id,omitempty"`
	RunID                 string           `yaml:"run_id,omitempty"`
	CallID                string           `yaml:"call_id,omitempty"`
	Attempt               int              `yaml:"attempt,omitempty"`
	Phase                 string           `yaml:"phase,omitempty"`
	Role                  string           `yaml:"role,omitempty"`
	ProviderVersion       string           `yaml:"provider_version,omitempty"`
	ModelVersion          string           `yaml:"model_version,omitempty"`
	RiskPolicy            string           `yaml:"risk_policy,omitempty"`
	CacheStratum          string           `yaml:"cache_stratum,omitempty"`
	ConfigHash            string           `yaml:"config_hash,omitempty"`
	EvidenceMode          bool             `yaml:"evidence_mode,omitempty"`
	DiagnosticMode        bool             `yaml:"diagnostic_mode,omitempty"`
	StrictVerdict         bool             `yaml:"strict_verdict,omitempty"`
	ZeroToolCallsRequired bool             `yaml:"zero_tool_calls_required,omitempty"`
	Codex                 taskCodexContext `yaml:"codex,omitempty"`
}

type taskCodexContext struct {
	Sandbox          string `yaml:"sandbox,omitempty"`
	Ephemeral        bool   `yaml:"ephemeral,omitempty"`
	IgnoreUserConfig bool   `yaml:"ignore_user_config,omitempty"`
	IgnoreRules      bool   `yaml:"ignore_rules,omitempty"`
	SkipGitRepoCheck bool   `yaml:"skip_git_repo_check,omitempty"`
	OutputSchema     string `yaml:"output_schema,omitempty"`
	ZeroToolMode     bool   `yaml:"zero_tool_mode,omitempty"`
	RawTokenBudget   int64  `yaml:"raw_token_budget,omitempty"`
}

// taskResult represents the output result of an agent task.
type taskResult struct {
	TaskID                        string                        `yaml:"task_id"`
	Status                        string                        `yaml:"status"`
	Timestamp                     string                        `yaml:"timestamp"`
	CostUSD                       float64                       `yaml:"cost_usd,omitempty"`
	DurationMS                    int64                         `yaml:"duration_ms,omitempty"`
	SessionID                     string                        `yaml:"session_id,omitempty"`
	Provider                      string                        `yaml:"provider,omitempty"`
	Model                         string                        `yaml:"model,omitempty"`
	Effort                        string                        `yaml:"effort,omitempty"`
	RunID                         string                        `yaml:"run_id,omitempty"`
	CallID                        string                        `yaml:"call_id,omitempty"`
	Attempt                       int                           `yaml:"attempt,omitempty"`
	Phase                         string                        `yaml:"phase,omitempty"`
	Role                          string                        `yaml:"role,omitempty"`
	Verdict                       string                        `yaml:"verdict,omitempty"`
	FindingCount                  *int                          `yaml:"finding_count,omitempty"`
	FindingCodes                  []string                      `yaml:"finding_codes,omitempty"`
	FindingScopeHashes            []string                      `yaml:"finding_scope_hashes,omitempty"`
	OutputSHA256                  string                        `yaml:"output_sha256,omitempty"`
	UsageStatus                   string                        `yaml:"usage_status,omitempty"`
	UniqueModelCallCount          *int                          `yaml:"unique_model_call_count,omitempty"`
	RawTotalTokens                *int64                        `yaml:"raw_total_tokens,omitempty"`
	ToolCalls                     *int                          `yaml:"tool_calls,omitempty"`
	OperationalErrorClass         string                        `yaml:"operational_error_class,omitempty"`
	OperationalErrorFingerprint   string                        `yaml:"operational_error_fingerprint,omitempty"`
	OperationalErrorStage         string                        `yaml:"operational_error_stage,omitempty"`
	OperationalErrorSignals       []string                      `yaml:"operational_error_signals,omitempty"`
	OperationalProviderEventKind  string                        `yaml:"operational_provider_event_kind,omitempty"`
	OperationalProviderEventShape []string                      `yaml:"operational_provider_event_shape,omitempty"`
	OperationalProviderEvents     []providerFailureEventReceipt `yaml:"operational_provider_events,omitempty"`
}

// newAgentRunSubCmd creates the `auto agent run <task-id>` subcommand.
func newAgentRunSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Run an agent task",
		Long:  "Execute a single pipeline task, reading context from .autopus/runs/<task-id>/context.yaml and writing results to result.yaml.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentTask(args[0])
		},
	}
	return cmd
}

// runAgentTask reads context.yaml for the given task ID and writes result.yaml upon completion.
func runAgentTask(taskID string) error {
	// Validate task ID to prevent path traversal (V-001).
	if !validTaskID.MatchString(taskID) {
		return fmt.Errorf("invalid task ID %q: must be alphanumeric with hyphens or underscores", taskID)
	}
	runsDir := filepath.Join(".autopus", "runs", taskID)
	contextPath := filepath.Join(runsDir, "context.yaml")

	// Read and parse context.yaml.
	data, err := os.ReadFile(contextPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task context not found: %s", taskID)
		}
		return fmt.Errorf("read context for %s: %w", taskID, err)
	}

	ctx, err := decodeTaskContext(data)
	if err != nil {
		if ctx.EvidenceMode {
			return persistEvidencePreflightFailure(taskID, runsDir, ctx, fmt.Errorf("parse context: %w", err))
		}
		return fmt.Errorf("parse context for %s: %w", taskID, err)
	}

	if ctx.EvidenceMode {
		if err := validateEvidenceContext(taskID, runsDir, &ctx); err != nil {
			return persistEvidencePreflightFailure(taskID, runsDir, ctx, err)
		}
		return runEvidenceAgentTask(taskID, runsDir, ctx)
	}
	return runLegacyAgentTask(taskID, runsDir, ctx)
}

func decodeTaskContext(data []byte) (taskContext, error) {
	var ctx taskContext
	if err := yaml.Unmarshal(data, &ctx); err != nil {
		return ctx, err
	}
	if !ctx.EvidenceMode {
		return ctx, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&ctx); err != nil {
		return ctx, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ctx, fmt.Errorf("multiple YAML documents are not allowed")
		}
		return ctx, err
	}
	return ctx, nil
}

func runLegacyAgentTask(taskID, runsDir string, ctx taskContext) error {
	providerName := strings.TrimSpace(ctx.Provider)
	if providerName == "" {
		providerName = "claude"
	}
	reg := buildDefaultRegistry()
	taskCfg := buildAgentTaskConfig(taskID, runsDir, ctx)
	res, execErr := executeAgentTask(context.Background(), reg, providerName, taskCfg)

	// Build result based on execution outcome.
	result := taskResult{
		TaskID:    taskID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if execErr != nil {
		result.Status = "failed"
	} else {
		result.Status = res.Status
		result.CostUSD = res.CostUSD
		result.DurationMS = res.DurationMS
		result.SessionID = res.SessionID
	}
	return writeTaskResult(runsDir, result)
}

func writeTaskResult(runsDir string, result taskResult) error {
	resultData, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result for %s: %w", result.TaskID, err)
	}
	if err := os.WriteFile(filepath.Join(runsDir, "result.yaml"), resultData, 0o600); err != nil {
		return fmt.Errorf("write result for %s: %w", result.TaskID, err)
	}
	return nil
}
