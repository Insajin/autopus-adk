package orchestra

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
)

var (
	// @AX:NOTE: [AUTO] redaction regexes protect diagnostic output; extend cautiously when new secret/path formats enter failure payloads.
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASS|API[_-]?KEY|ACCESS[_-]?KEY)[A-Z0-9_]*)\s*=\s*([^\s,;]+)`)
	userPathPattern         = regexp.MustCompile(`/Users/[^\s"'<>:]+`)
)

func buildFailedProvider(provider ProviderConfig, resp *ProviderResponse, err error, fallbackSeconds int) FailedProvider {
	return buildFailedProviderWithContext(provider, resp, err, fallbackSeconds, "", false)
}

func buildFailedProviderWithContext(
	provider ProviderConfig,
	resp *ProviderResponse,
	err error,
	fallbackSeconds int,
	role string,
	otherProvidersContinued bool,
) FailedProvider {
	failure := FailedProvider{
		Name:                    provider.Name,
		Role:                    role,
		ModelFamily:             provider.ModelFamily,
		OtherProvidersContinued: otherProvidersContinued,
	}
	if resp != nil {
		failure.ExecutedBackend = resp.ExecutedBackend
		failure.TerminalState = resp.TerminalState
		failure.DegradedReasons = append([]string(nil), resp.DegradedReasons...)
		failure.PreflightFailed = resp.ExecutedBackend == noneBackendMarker &&
			(resp.TerminalState == TerminalSkipped || resp.TerminalState == TerminalBlocked)
		failure.ExitCode = resp.ExitCode
		failure.TimedOut = resp.TimedOut
		failure.Receipt = resp.Receipt
		failure.Usage = append([]telemetry.UsageEnvelope(nil), resp.Usage...)
		failure.UsageCapability = resp.UsageCapability
		if resp.Provider != "" {
			failure.Name = resp.Provider
		}
		if !resp.TimedOut {
			failure.StderrPreview = previewFailureText(resp.Error)
			failure.OutputPreview = previewProviderOutput(resp.Output)
		}
	}

	switch {
	case resp != nil && resp.TimedOut:
		fillTimeoutFailure(&failure, provider, resp, fallbackSeconds)
	case resp != nil && resp.EmptyOutput:
		failure.Error = "empty output: provider returned no content (check binary args or prompt_via_args setting)"
	case err != nil:
		failure.Error = redactFailureText(err.Error())
	default:
		failure.Error = "provider failed without a structured error"
	}

	if failure.FailureClass == "" {
		failure.FailureClass = classifyFailure(failure.Error, resp)
	}
	failure.NextRemediation = remediationForFailureClass(failure.FailureClass)
	return failure
}

func fillTimeoutFailure(failure *FailedProvider, provider ProviderConfig, resp *ProviderResponse, fallbackSeconds int) {
	timeoutUsed := providerExecutionTimeout(provider, fallbackSeconds)
	failure.FailureClass = "timeout"
	failure.TimeoutSource = providerTimeoutSource(provider, fallbackSeconds)
	failure.ConfiguredDuration = timeoutUsed
	failure.ElapsedDuration = resp.Duration
	subject := fmt.Sprintf("provider %s", failure.Name)
	if failure.Role != "" {
		subject = fmt.Sprintf("%s role %s", subject, failure.Role)
	}
	if resp.Duration > 0 {
		failure.Error = fmt.Sprintf("timeout: %s exceeded %v deadline after %v", subject, timeoutUsed, resp.Duration)
		return
	}
	failure.Error = fmt.Sprintf("timeout: %s exceeded %v deadline", subject, timeoutUsed)
}

func providerTimeoutSource(provider ProviderConfig, fallbackSeconds int) string {
	// @AX:NOTE: [AUTO] timeout source strings are diagnostics wire values surfaced through FailedProvider JSON.
	switch {
	case provider.ExecutionTimeout > 0:
		return "provider_execution_timeout"
	case fallbackSeconds > 0:
		return "orchestra_timeout_seconds"
	default:
		return "default_provider_execution_timeout"
	}
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
	normalized = redactFailureText(normalized)
	if len(normalized) <= 160 {
		return normalized
	}
	return normalized[:160] + "..."
}

func previewProviderOutput(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "[redacted_provider_output]"
}

func redactFailureText(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	redacted := secretAssignmentPattern.ReplaceAllString(text, "${1}=[redacted]")
	redacted = userPathPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		return "[redacted_path:" + path.Base(match) + "]"
	})
	return redacted
}

func buildFailureResult(cfg OrchestraConfig, responses []ProviderResponse, failed []FailedProvider, roundHistory [][]ProviderResponse, start time.Time, runErr error) *OrchestraResult {
	summary := failureSummary(failed)
	if summary == "" && runErr != nil {
		summary = runErr.Error()
	}
	return finalizeOrchestraResultForConfig(&OrchestraResult{
		Strategy:        cfg.Strategy,
		Responses:       responses,
		Duration:        time.Since(start),
		Summary:         summary,
		FailedProviders: failed,
		RoundHistory:    roundHistory,
		RunID:           cfg.RunID,
		TerminalState:   TerminalBlocked,
	}, cfg)
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
