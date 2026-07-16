package orchestra

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

type providerUsageIdentity struct {
	Type    string `json:"type"`
	RunID   string `json:"run_id"`
	CallID  string `json:"call_id"`
	TaskID  string `json:"task_id"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Phase   string `json:"phase"`
	Role    string `json:"role"`
	Attempt int    `json:"attempt"`
}

type codexUsageEvent struct {
	providerUsageIdentity
	Usage *struct {
		InputTokens           *int64 `json:"input_tokens"`
		CachedInputTokens     *int64 `json:"cached_input_tokens"`
		OutputTokens          *int64 `json:"output_tokens"`
		ReasoningOutputTokens *int64 `json:"reasoning_output_tokens"`
		ReasoningTokens       *int64 `json:"reasoning_tokens"`
		ToolTokens            *int64 `json:"tool_tokens"`
		ToolRelation          string `json:"tool_relation"`
	} `json:"usage"`
}

type claudeUsageEvent struct {
	providerUsageIdentity
	CostUSD      *float64 `json:"cost_usd"`
	TotalCostUSD *float64 `json:"total_cost_usd"`
	Usage        *struct {
		InputTokens         *int64 `json:"input_tokens"`
		CacheCreationTokens *int64 `json:"cache_creation_input_tokens"`
		CacheReadTokens     *int64 `json:"cache_read_input_tokens"`
		OutputTokens        *int64 `json:"output_tokens"`
	} `json:"usage"`
}

// parseCodexUsage extracts only allowlisted usage and identity fields. Provider
// prompts, response bodies, and arbitrary event metadata are never retained.
func parseCodexUsage(raw string, binding usageBinding) []telemetry.UsageEnvelope {
	var receipts []telemetry.UsageEnvelope
	forEachJSONLine(raw, func(line []byte) {
		var event codexUsageEvent
		if json.Unmarshal(line, &event) == nil && event.Type == "turn.completed" && event.Usage != nil {
			receipts = append(receipts, normalizeCodexUsage(event, binding))
		}
	})
	return receipts
}

func parseClaudeUsage(raw string, binding usageBinding) []telemetry.UsageEnvelope {
	var receipts []telemetry.UsageEnvelope
	forEachJSONLine(raw, func(line []byte) {
		var event claudeUsageEvent
		if json.Unmarshal(line, &event) != nil || event.Type != "result" ||
			(event.Usage == nil && event.TotalCostUSD == nil && event.CostUSD == nil) {
			return
		}
		resolved := resolveUsageIdentity(event.providerUsageIdentity, binding)
		actualCost := event.TotalCostUSD
		if actualCost == nil {
			actualCost = event.CostUSD
		}
		var input, cacheCreation, cacheRead, output *int64
		if event.Usage != nil {
			input = event.Usage.InputTokens
			cacheCreation = event.Usage.CacheCreationTokens
			cacheRead = event.Usage.CacheReadTokens
			output = event.Usage.OutputTokens
		}
		receipts = append(receipts, telemetry.NormalizeUsage(telemetry.UsageInput{
			RunID: resolved.RunID, CallID: resolved.CallID, TaskID: resolved.TaskID,
			Attempt: resolved.Attempt, Provider: resolved.Provider, Model: resolved.Model,
			Effort: resolved.Effort, Phase: resolved.Phase, Role: resolved.Role,
			Source: telemetry.UsageSourceProvider, SourceSchema: "claude.result.v1",
			UncachedInputTokens: input, CacheCreationInputTokens: cacheCreation,
			CacheReadInputTokens: cacheRead, OutputTokensTotal: output, ActualCostUSD: actualCost,
		}))
	})
	return receipts
}

func normalizeCodexUsage(event codexUsageEvent, binding usageBinding) telemetry.UsageEnvelope {
	resolved := resolveUsageIdentity(event.providerUsageIdentity, binding)
	reasoning := event.Usage.ReasoningOutputTokens
	reasoningRelation := ""
	if reasoning != nil {
		reasoningRelation = telemetry.ComponentSubsetOfOutput
	} else if event.Usage.ReasoningTokens != nil {
		reasoning = event.Usage.ReasoningTokens
	}
	return telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: resolved.RunID, CallID: resolved.CallID, TaskID: resolved.TaskID,
		Attempt: resolved.Attempt, Provider: resolved.Provider, Model: resolved.Model,
		Effort: resolved.Effort, Phase: resolved.Phase, Role: resolved.Role,
		Source: telemetry.UsageSourceProvider, SourceSchema: "codex.turn.completed.v1",
		InputTokensTotal: event.Usage.InputTokens, CachedInputTokens: event.Usage.CachedInputTokens,
		OutputTokensTotal: event.Usage.OutputTokens, ReasoningTokens: reasoning,
		ReasoningRelation: reasoningRelation, ToolTokens: event.Usage.ToolTokens,
		ToolRelation: safeRelation(event.Usage.ToolRelation),
	})
}

func resolveUsageIdentity(event providerUsageIdentity, fallback usageBinding) usageBinding {
	// Provider output is untrusted usage data. Supervisor-bound identity and
	// policy metadata are authoritative and must never be overwritten here.
	_ = event
	return fallback
}

func forEachJSONLine(raw string, visit func([]byte)) {
	reader := bufio.NewReader(strings.NewReader(raw))
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimSpace(line) != "" {
			visit([]byte(line))
		}
		if err != nil {
			if err != io.EOF {
				return
			}
			return
		}
	}
}

func safeRelation(value string) string {
	switch value {
	case telemetry.ComponentSubsetOfInput, telemetry.ComponentSubsetOfOutput, telemetry.ComponentSeparate:
		return value
	default:
		return ""
	}
}
