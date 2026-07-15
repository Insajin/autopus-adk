package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

// execResult holds the outcome of running a provider subprocess.
type execResult struct {
	Status                        string
	CostUSD                       float64
	DurationMS                    int64
	SessionID                     string
	Output                        string
	Usage                         []telemetry.UsageEnvelope
	ToolCalls                     int
	OperationalErrorClass         string
	OperationalErrorFingerprint   string
	OperationalErrorStage         string
	OperationalErrorSignals       []string
	OperationalProviderEventKind  string
	OperationalProviderEventShape []string
	OperationalProviderEvents     []providerFailureEventReceipt
}

// buildDefaultRegistry creates a registry with all known provider adapters.
func buildDefaultRegistry() *adapter.Registry {
	reg := adapter.NewRegistry()
	reg.Register(adapter.NewClaudeAdapter())
	reg.Register(adapter.NewCodexAdapter())
	reg.Register(adapter.NewGeminiAdapter())
	return reg
}

// executeAgentTask resolves the adapter, spawns subprocess, parses stream, returns result.
func executeAgentTask(ctx context.Context, reg *adapter.Registry, providerName string, taskCfg adapter.TaskConfig) (execResult, error) {
	prov, err := reg.Get(providerName)
	if err != nil {
		return execResult{}, fmt.Errorf("resolve provider %q: %w", providerName, err)
	}
	taskCfg = ensureAgentTaskUsageIdentity(taskCfg)
	strictEvidenceStream := providerName == "codex" && taskCfg.EvidenceMode

	cmd := prov.BuildCommand(ctx, taskCfg)
	var stderr boundedOperationalErrorBuffer
	cmd.Stderr = &stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return execResult{}, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return execResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		class, fingerprint := classifyOperationalError(stderr.String(), err)
		return execResult{Status: "failed", OperationalErrorClass: class,
				OperationalErrorFingerprint: fingerprint, OperationalErrorStage: "subprocess_start",
				OperationalErrorSignals: operationalErrorSignals(stderr.HasData(), false, false)},
			fmt.Errorf("start subprocess: %w", err)
	}

	// Write prompt via stdin then close to signal EOF to the subprocess.
	_, _ = io.Copy(stdinPipe, strings.NewReader(taskCfg.Prompt))
	stdinPipe.Close()

	// Parse stream output and capture the last result event.
	scanner := bufio.NewScanner(stdout)
	var lastResult adapter.TaskResult
	hasResult := false
	var usage []telemetry.UsageEnvelope
	toolCalls := 0
	sawToolEvent := false
	streamParseFailed := false
	var providerFailure providerFailureObservation
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		evt, err := prov.ParseEvent([]byte(line))
		if err != nil {
			if strictEvidenceStream {
				streamParseFailed = true
				log.Printf("[agent-run] evidence stream parse failure")
			} else {
				log.Printf("[agent-run] stream parse error: %v", err)
			}
			continue
		}
		usage = mergeAgentUsage(usage, bindAgentUsage(evt.Usage, taskCfg, providerName))
		if strictEvidenceStream && (evt.Type == "error" || (evt.Type == "turn" && evt.Subtype == "failed")) {
			providerFailure.Observe(evt.Type, evt.Subtype, evt.Data)
		}
		if evt.Type == "tool_call" || evt.Type == "tool_use" {
			sawToolEvent = true
			count := evt.ToolCalls
			if count == 0 {
				count = 1
			}
			toolCalls += count
		}
		if evt.Type == "result" {
			result := prov.ExtractResult(evt)
			result.Usage = bindAgentUsage(result.Usage, taskCfg, providerName)
			reportedToolCalls := result.ToolCalls
			if evt.ToolCalls > reportedToolCalls {
				reportedToolCalls = evt.ToolCalls
			}
			if !sawToolEvent && reportedToolCalls > toolCalls {
				toolCalls = reportedToolCalls
			}
			result.ToolCalls = 0
			lastResult = adapter.MergeSequentialResult(prov.Name(), lastResult, hasResult, result)
			hasResult = true
		}
	}
	if err := scanner.Err(); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		providerEvents := providerFailure.Events()
		class, fingerprint := classifyOperationalErrorWithProvider(stderr.String(), err, providerEvents)
		return execResult{Status: "failed", DurationMS: time.Since(startTime).Milliseconds(), Usage: mergeAgentUsage(usage, lastResult.Usage), ToolCalls: toolCalls, Output: lastResult.Output,
				OperationalErrorClass: class, OperationalErrorFingerprint: fingerprint, OperationalErrorStage: "stream_scan",
				OperationalErrorSignals:      operationalErrorSignals(stderr.HasData(), providerFailure.Observed(), streamParseFailed),
				OperationalProviderEventKind: providerFailure.Kind(), OperationalProviderEventShape: providerFailure.Shape(),
				OperationalProviderEvents: providerEvents},
			fmt.Errorf("stream scan: %w", err)
	}

	waitErr := cmd.Wait()
	durationMS := time.Since(startTime).Milliseconds()

	if waitErr != nil && !hasResult {
		providerEvents := providerFailure.Events()
		class, fingerprint := classifyOperationalErrorWithProvider(stderr.String(), waitErr, providerEvents)
		return execResult{Status: "failed", DurationMS: durationMS, Usage: usage, ToolCalls: toolCalls,
				OperationalErrorClass: class, OperationalErrorFingerprint: fingerprint, OperationalErrorStage: "process_wait",
				OperationalErrorSignals:      operationalErrorSignals(stderr.HasData(), providerFailure.Observed(), streamParseFailed),
				OperationalProviderEventKind: providerFailure.Kind(), OperationalProviderEventShape: providerFailure.Shape(),
				OperationalProviderEvents: providerEvents},
			fmt.Errorf("subprocess exited with error: %w", waitErr)
	}

	if !hasResult {
		cause := fmt.Errorf("no result event from subprocess")
		providerEvents := providerFailure.Events()
		class, fingerprint := classifyOperationalErrorWithProvider(stderr.String(), cause, providerEvents)
		return execResult{Status: "failed", DurationMS: durationMS, Usage: usage, ToolCalls: toolCalls,
				OperationalErrorClass: class, OperationalErrorFingerprint: fingerprint, OperationalErrorStage: "missing_result",
				OperationalErrorSignals:      operationalErrorSignals(stderr.HasData(), providerFailure.Observed(), streamParseFailed),
				OperationalProviderEventKind: providerFailure.Kind(), OperationalProviderEventShape: providerFailure.Shape(),
				OperationalProviderEvents: providerEvents},
			cause
	}

	result := execResult{
		Status:     "success",
		CostUSD:    lastResult.CostUSD,
		DurationMS: durationMS,
		SessionID:  lastResult.SessionID,
		Output:     lastResult.Output,
		Usage:      mergeAgentUsage(usage, lastResult.Usage),
		ToolCalls:  toolCalls,
	}
	var evidenceErr error
	if strictEvidenceStream && waitErr != nil {
		evidenceErr = errors.Join(evidenceErr, fmt.Errorf("subprocess exited with error: %w", waitErr))
	}
	if strictEvidenceStream && streamParseFailed {
		evidenceErr = errors.Join(evidenceErr, fmt.Errorf("provider stream parse failure"))
	}
	if strictEvidenceStream && providerFailure.Observed() {
		evidenceErr = errors.Join(evidenceErr, fmt.Errorf("provider failure event"))
	}
	if evidenceErr != nil {
		providerEvents := providerFailure.Events()
		result.OperationalErrorClass, result.OperationalErrorFingerprint =
			classifyOperationalErrorWithProvider(stderr.String(), evidenceErr, providerEvents)
		result.OperationalErrorStage = "evidence_postcondition"
		result.OperationalErrorSignals = operationalErrorSignals(stderr.HasData(), providerFailure.Observed(), streamParseFailed)
		result.OperationalProviderEventKind = providerFailure.Kind()
		result.OperationalProviderEventShape = providerFailure.Shape()
		result.OperationalProviderEvents = providerEvents
	}
	return result, evidenceErr
}

func ensureAgentTaskUsageIdentity(task adapter.TaskConfig) adapter.TaskConfig {
	if task.Attempt <= 0 {
		task.Attempt = 1
	}
	if task.Phase == "" {
		task.Phase = "execute"
	}
	if task.Role == "" {
		task.Role = "agent"
	}
	if task.RunID == "" {
		task.RunID = fmt.Sprintf("%s-%d", task.TaskID, time.Now().UnixNano())
	}
	if task.CallID == "" {
		task.CallID = fmt.Sprintf("%s:%s:%d", task.RunID, task.Phase, task.Attempt)
	}
	return task
}

func bindAgentUsage(inputs []telemetry.UsageEnvelope, task adapter.TaskConfig, provider string) []telemetry.UsageEnvelope {
	bound := make([]telemetry.UsageEnvelope, len(inputs))
	for i, envelope := range inputs {
		envelope.RunID, envelope.CallID, envelope.TaskID = task.RunID, task.CallID, task.TaskID
		envelope.Attempt, envelope.Provider = task.Attempt, provider
		envelope.Model, envelope.Effort = task.Model, task.Effort
		envelope.Phase, envelope.Role = task.Phase, task.Role
		envelope.ProviderVersion = task.ProviderVersion
		envelope.ModelVersion = task.ModelVersion
		envelope.RiskPolicy = task.RiskPolicy
		envelope.CacheStratum = task.CacheStratum
		envelope.ConfigHash = task.ConfigHash
		bound[i] = envelope
	}
	return bound
}

func mergeAgentUsage(left, right []telemetry.UsageEnvelope) []telemetry.UsageEnvelope {
	merged := append([]telemetry.UsageEnvelope(nil), left...)
	for _, candidate := range right {
		duplicate := false
		for _, prior := range merged {
			if prior.RunID == candidate.RunID && prior.CallID == candidate.CallID &&
				!telemetry.AggregateUsage([]telemetry.UsageEnvelope{prior, candidate}).PromotionBlocked {
				duplicate = true
				break
			}
		}
		if !duplicate {
			merged = append(merged, candidate)
		}
	}
	return merged
}
