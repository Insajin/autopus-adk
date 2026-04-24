package orchestra

import (
	"fmt"
	"strings"
	"time"
)

func buildFailedProvider(provider ProviderConfig, resp *ProviderResponse, err error, fallbackSeconds int) FailedProvider {
	failure := FailedProvider{Name: provider.Name}
	if resp != nil {
		if resp.Provider != "" {
			failure.Name = resp.Provider
		}
		failure.StderrPreview = previewFailureText(resp.Error)
		failure.OutputPreview = previewFailureText(resp.Output)
	}

	switch {
	case resp != nil && resp.TimedOut:
		timeoutUsed := providerExecutionTimeout(provider, fallbackSeconds)
		failure.Error = fmt.Sprintf("timeout: provider exceeded %v deadline", timeoutUsed)
	case resp != nil && resp.EmptyOutput:
		failure.Error = "empty output: provider returned no content (check binary args or prompt_via_args setting)"
	case err != nil:
		failure.Error = err.Error()
	default:
		failure.Error = "provider failed without a structured error"
	}

	failure.FailureClass = classifyFailure(failure.Error, resp)
	failure.NextRemediation = remediationForFailureClass(failure.FailureClass)
	return failure
}

func classifyFailure(errMsg string, resp *ProviderResponse) string {
	signals := []string{strings.ToLower(errMsg)}
	if resp != nil {
		signals = append(signals, strings.ToLower(resp.Error), strings.ToLower(resp.Output))
	}
	joined := strings.Join(signals, " ")

	switch {
	case strings.Contains(joined, "capacity exhausted"),
		strings.Contains(joined, "model_capacity_exhausted"),
		strings.Contains(joined, "model capacity unavailable"),
		strings.Contains(joined, "no capacity available for model"):
		return "capacity_exhausted"
	case strings.Contains(joined, "rate limit exceeded"),
		strings.Contains(joined, "ratelimitexceeded"):
		return "rate_limited"
	case strings.Contains(joined, "resource exhausted"),
		strings.Contains(joined, "resource_exhausted"):
		return "resource_exhausted"
	case strings.Contains(joined, "timeout:"),
		strings.Contains(joined, "deadline exceeded"),
		strings.Contains(joined, "context deadline exceeded"):
		return "timeout"
	case strings.Contains(joined, "empty output"):
		return "empty_output"
	case strings.Contains(joined, "not found"),
		strings.Contains(joined, "찾을 수 없습니다"),
		strings.Contains(joined, "sendcommand failed"),
		strings.Contains(joined, "stdin"),
		strings.Contains(joined, "start failed"),
		strings.Contains(joined, "시작 실패"):
		return "binary_or_transport"
	default:
		return "execution_error"
	}
}

func remediationForFailureClass(class string) string {
	switch class {
	case "capacity_exhausted":
		return "retry later or reduce provider set"
	case "rate_limited":
		return "wait for quota reset or lower request rate"
	case "resource_exhausted":
		return "retry later or switch provider/model"
	case "timeout":
		return "increase timeout or simplify strategy"
	case "empty_output":
		return "check provider args or prompt transport"
	case "binary_or_transport":
		return "verify binary availability and transport settings"
	default:
		return "inspect stderr and provider configuration"
	}
}

func previewFailureText(text string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if normalized == "" {
		return ""
	}
	if len(normalized) <= 160 {
		return normalized
	}
	return normalized[:160] + "..."
}

func buildFailureResult(cfg OrchestraConfig, failed []FailedProvider, roundHistory [][]ProviderResponse, start time.Time, runErr error) *OrchestraResult {
	summary := failureSummary(failed)
	if summary == "" && runErr != nil {
		summary = runErr.Error()
	}
	return &OrchestraResult{
		Strategy:        cfg.Strategy,
		Duration:        time.Since(start),
		Summary:         summary,
		FailedProviders: failed,
		RoundHistory:    roundHistory,
		RunID:           cfg.RunID,
	}
}

func failureSummary(failed []FailedProvider) string {
	if len(failed) == 0 {
		return ""
	}

	parts := make([]string, 0, len(failed))
	for _, fp := range failed {
		class := fp.FailureClass
		if class == "" {
			class = classifyFailure(fp.Error, nil)
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", fp.Name, class))
	}
	return "all providers failed: " + strings.Join(parts, ", ")
}
