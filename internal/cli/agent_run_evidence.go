package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

type strictVerdictReceipt struct {
	Verdict      string
	FindingCount int
}

func validateEvidenceContext(taskID, runsDir string, ctx *taskContext) error {
	if ctx.Provider != "codex" {
		return fmt.Errorf("evidence preflight: provider must be exactly codex")
	}
	if ctx.Model != "gpt-5.6-sol" {
		return fmt.Errorf("evidence preflight: model must be exactly gpt-5.6-sol")
	}
	if ctx.Effort != "xhigh" && ctx.Effort != "max" {
		return fmt.Errorf("evidence preflight: effort must be xhigh or max")
	}
	if ctx.TaskID == "" || ctx.TaskID != taskID {
		return fmt.Errorf("evidence preflight: task identity mismatch")
	}
	if strings.TrimSpace(ctx.Description) == "" {
		return fmt.Errorf("evidence preflight: task description must be nonempty")
	}
	if ctx.Attempt != 1 {
		return fmt.Errorf("evidence preflight: attempt must be exactly 1")
	}
	stable := map[string]string{
		"spec_id": ctx.SpecID, "run_id": ctx.RunID, "call_id": ctx.CallID, "phase": ctx.Phase,
		"role": ctx.Role, "provider_version": ctx.ProviderVersion, "model_version": ctx.ModelVersion,
		"risk_policy": ctx.RiskPolicy, "cache_stratum": ctx.CacheStratum, "config_hash": ctx.ConfigHash,
	}
	for name, value := range stable {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("evidence preflight: %s must be a nonempty stable identity", name)
		}
	}
	if ctx.Codex.Sandbox != string(adapter.CodexSandboxReadOnly) {
		return fmt.Errorf("evidence preflight: codex sandbox must be read-only")
	}
	if !ctx.Codex.Ephemeral || !ctx.Codex.IgnoreUserConfig || !ctx.Codex.IgnoreRules ||
		!ctx.Codex.SkipGitRepoCheck || !ctx.Codex.ZeroToolMode {
		return fmt.Errorf("evidence preflight: all codex isolation controls are required")
	}
	if !ctx.StrictVerdict || !ctx.ZeroToolCallsRequired {
		return fmt.Errorf("evidence preflight: strict verdict and zero tool calls must be required")
	}
	if ctx.Codex.RawTokenBudget <= 0 {
		return fmt.Errorf("evidence preflight: raw token budget must be positive")
	}
	diagnosticSchema := ctx.Codex.OutputSchema == diagnosticVerdictSchemaBasename
	if ctx.DiagnosticMode != diagnosticSchema {
		return fmt.Errorf("evidence preflight: diagnostic mode requires the exact diagnostic schema")
	}
	return validateEvidenceSchema(runsDir, ctx.Codex.OutputSchema)
}

func validateEvidenceSchema(runsDir, name string) error {
	if name == "" || name != strings.TrimSpace(name) || filepath.IsAbs(name) {
		return fmt.Errorf("evidence preflight: output schema must be a safe relative path")
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence preflight: output schema escapes run directory")
	}
	root, err := filepath.EvalSymlinks(runsDir)
	if err != nil {
		return fmt.Errorf("evidence preflight: resolve run directory: %w", err)
	}
	target, err := filepath.EvalSymlinks(filepath.Join(runsDir, clean))
	if err != nil {
		return fmt.Errorf("evidence preflight: resolve output schema: %w", err)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence preflight: output schema escapes run directory")
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("evidence preflight: output schema must be a regular file")
	}
	return nil
}

func buildAgentTaskConfig(taskID, runsDir string, ctx taskContext) adapter.TaskConfig {
	return adapter.TaskConfig{
		TaskID: taskID, RunID: ctx.RunID, CallID: ctx.CallID, Attempt: ctx.Attempt,
		Phase: ctx.Phase, Role: ctx.Role, Effort: ctx.Effort,
		ProviderVersion: ctx.ProviderVersion, ModelVersion: ctx.ModelVersion,
		RiskPolicy: ctx.RiskPolicy, CacheStratum: ctx.CacheStratum, ConfigHash: ctx.ConfigHash,
		Prompt: ctx.Description, WorkDir: runsDir, Model: ctx.Model, EvidenceMode: ctx.EvidenceMode,
		Codex: adapter.CodexTaskOptions{
			Sandbox: adapter.CodexSandboxMode(ctx.Codex.Sandbox), Ephemeral: ctx.Codex.Ephemeral,
			IgnoreUserConfig: ctx.Codex.IgnoreUserConfig, IgnoreRules: ctx.Codex.IgnoreRules,
			SkipGitRepoCheck: ctx.Codex.SkipGitRepoCheck, OutputSchema: ctx.Codex.OutputSchema,
			ZeroToolMode: ctx.Codex.ZeroToolMode, RawTokenBudget: ctx.Codex.RawTokenBudget,
		},
	}
}

func runEvidenceAgentTask(taskID, runsDir string, ctx taskContext) error {
	started := time.Now().UTC()
	res, execErr := executeAgentTask(context.Background(), buildDefaultRegistry(), ctx.Provider,
		buildAgentTaskConfig(taskID, runsDir, ctx))
	result, outcomeErr := evaluateEvidenceResult(taskID, ctx, res, execErr)
	return persistEvidenceOutcome(runsDir, ctx, started, res.Usage, result, outcomeErr)
}

func persistEvidencePreflightFailure(taskID, runsDir string, ctx taskContext, cause error) error {
	started := time.Now().UTC()
	result := baseEvidenceResult(taskID, ctx)
	result.Status = "failed"
	return persistEvidenceOutcome(runsDir, ctx, started, nil, result, cause)
}

func evaluateEvidenceResult(taskID string, ctx taskContext, res execResult, execErr error) (taskResult, error) {
	result := baseEvidenceResult(taskID, ctx)
	result.CostUSD, result.DurationMS = res.CostUSD, res.DurationMS
	result.OperationalErrorClass = res.OperationalErrorClass
	result.OperationalErrorFingerprint = res.OperationalErrorFingerprint
	result.OperationalErrorStage = res.OperationalErrorStage
	result.OperationalErrorSignals = append([]string(nil), res.OperationalErrorSignals...)
	result.OperationalProviderEventKind = res.OperationalProviderEventKind
	result.OperationalProviderEventShape = append([]string(nil), res.OperationalProviderEventShape...)
	result.OperationalProviderEvents = cloneProviderFailureEvents(res.OperationalProviderEvents)
	result.OutputSHA256 = evidenceOutputSHA256(res.Output)
	toolCalls := res.ToolCalls
	result.ToolCalls = &toolCalls
	aggregate, usageErr := validateEvidenceUsage(ctx, res.Usage)
	result.UsageStatus = aggregate.UsageStatus
	result.UniqueModelCallCount = intPointer(aggregate.UniqueModelCallCount)
	result.RawTotalTokens = cloneInt64Pointer(aggregate.RawTotalTokens)

	var outcomeErr error
	if execErr != nil {
		outcomeErr = errors.Join(outcomeErr, fmt.Errorf("evidence subprocess failed: %w", execErr))
	}
	if res.Status != "success" {
		outcomeErr = errors.Join(outcomeErr, fmt.Errorf("evidence subprocess status is not success"))
	}
	if usageErr != nil {
		outcomeErr = errors.Join(outcomeErr, usageErr)
	}
	if res.ToolCalls > 0 {
		outcomeErr = errors.Join(outcomeErr, fmt.Errorf("evidence tool event detected"))
	}
	if ctx.StrictVerdict {
		outcomeErr = errors.Join(outcomeErr, applyEvidenceVerdict(ctx, res.Output, &result))
	}
	if outcomeErr != nil {
		result.Status = "failed"
	} else {
		result.Status = "success"
	}
	return result, outcomeErr
}

func applyEvidenceVerdict(ctx taskContext, output string, result *taskResult) error {
	if ctx.DiagnosticMode {
		verdict, err := parseDiagnosticVerdict(output)
		if err != nil {
			return err
		}
		result.Verdict, result.FindingCount = verdict.Verdict, intPointer(verdict.FindingCount)
		result.FindingCodes = append([]string(nil), verdict.FindingCodes...)
		result.FindingScopeHashes = append([]string(nil), verdict.FindingScopeHashes...)
		return nil
	}
	verdict, err := parseStrictVerdict(output)
	if err != nil {
		return err
	}
	result.Verdict, result.FindingCount = verdict.Verdict, intPointer(verdict.FindingCount)
	if verdict.Verdict != "PASS" {
		return fmt.Errorf("strict verdict is FAIL")
	}
	return nil
}

func validateEvidenceUsage(ctx taskContext, usage []telemetry.UsageEnvelope) (telemetry.UsageAggregate, error) {
	aggregate := telemetry.AggregateUsage(usage)
	if aggregate.PromotionBlocked || aggregate.UniqueModelCallCount != 1 ||
		aggregate.UsageStatus != telemetry.UsageStatusActual || aggregate.RawTotalTokens == nil ||
		*aggregate.RawTotalTokens <= 0 || *aggregate.RawTotalTokens > ctx.Codex.RawTokenBudget {
		return aggregate, fmt.Errorf("evidence usage is missing, ambiguous, non-actual, or outside budget")
	}
	for _, envelope := range usage {
		if envelope.RunID != ctx.RunID || envelope.CallID != ctx.CallID || envelope.TaskID != ctx.TaskID ||
			envelope.Attempt != ctx.Attempt || envelope.Provider != ctx.Provider || envelope.Model != ctx.Model ||
			envelope.Effort != ctx.Effort || envelope.ProviderVersion != ctx.ProviderVersion ||
			envelope.ModelVersion != ctx.ModelVersion || envelope.RiskPolicy != ctx.RiskPolicy ||
			envelope.CacheStratum != ctx.CacheStratum || envelope.ConfigHash != ctx.ConfigHash ||
			envelope.Phase != ctx.Phase || envelope.Role != ctx.Role {
			return aggregate, fmt.Errorf("evidence usage identity mismatch")
		}
		if envelope.UsageStatus != telemetry.UsageStatusActual || envelope.UsageSource != telemetry.UsageSourceProvider ||
			envelope.RawTotalTokens == nil || *envelope.RawTotalTokens <= 0 {
			return aggregate, fmt.Errorf("evidence usage receipt is not provider actual")
		}
	}
	return aggregate, nil
}

func parseStrictVerdict(output string) (strictVerdictReceipt, error) {
	var wire struct {
		Verdict      *string `json:"verdict"`
		FindingCount *int    `json:"finding_count"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return strictVerdictReceipt{}, fmt.Errorf("strict verdict parse failed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return strictVerdictReceipt{}, fmt.Errorf("strict verdict has trailing JSON")
	}
	if wire.Verdict == nil || (*wire.Verdict != "PASS" && *wire.Verdict != "FAIL") ||
		wire.FindingCount == nil || *wire.FindingCount < 0 || *wire.FindingCount > 1000 {
		return strictVerdictReceipt{}, fmt.Errorf("strict verdict object is outside bounds")
	}
	return strictVerdictReceipt{Verdict: *wire.Verdict, FindingCount: *wire.FindingCount}, nil
}

func baseEvidenceResult(taskID string, ctx taskContext) taskResult {
	return taskResult{
		TaskID: taskID, Timestamp: time.Now().UTC().Format(time.RFC3339), Provider: ctx.Provider,
		Model: ctx.Model, Effort: ctx.Effort, RunID: ctx.RunID, CallID: ctx.CallID,
		Attempt: ctx.Attempt, Phase: ctx.Phase, Role: ctx.Role, OutputSHA256: evidenceOutputSHA256(""),
		UsageStatus: telemetry.UsageStatusUnavailable, UniqueModelCallCount: intPointer(0), ToolCalls: intPointer(0),
	}
}

func evidenceOutputSHA256(output string) string {
	hash := sha256.Sum256([]byte(output))
	return hex.EncodeToString(hash[:])
}

func persistEvidenceOutcome(runsDir string, ctx taskContext, started time.Time, usage []telemetry.UsageEnvelope, result taskResult, outcomeErr error) error {
	ended := time.Now().UTC()
	status := telemetry.StatusPass
	if result.Status != "success" || outcomeErr != nil {
		status = telemetry.StatusFail
		result.Status = "failed"
	}
	run := telemetry.AgentRun{
		AgentName: ctx.Role, SpecID: ctx.SpecID, TaskID: result.TaskID, RunID: ctx.RunID,
		CallID: ctx.CallID, Attempt: ctx.Attempt, Provider: ctx.Provider, Model: ctx.Model,
		Effort: ctx.Effort, Phase: ctx.Phase, Role: ctx.Role, StartTime: started, EndTime: ended,
		Duration: time.Duration(result.DurationMS) * time.Millisecond, Status: status,
		AcceptanceStatus: result.Verdict, ToolCalls: valueOrZero(result.ToolCalls),
		Usage: append([]telemetry.UsageEnvelope(nil), usage...),
	}
	if err := telemetry.AppendAgentRun(".", run); err != nil {
		result.Status = "failed"
		outcomeErr = errors.Join(outcomeErr, fmt.Errorf("persist evidence telemetry: %w", err))
	}
	if err := writeTaskResult(runsDir, result); err != nil {
		outcomeErr = errors.Join(outcomeErr, err)
	}
	return outcomeErr
}

func intPointer(value int) *int { return &value }
func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
