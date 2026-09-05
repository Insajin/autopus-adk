package orchestra

import (
	"context"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// @AX:ANCHOR: [AUTO] direct judge execution boundary that emits both verdict outcome and fresh-session evidence
// @AX:REASON: [AUTO] debate finalization and run receipts depend on consistent failure, attempt, backend, and freshness projection
func executeDebateJudge(
	ctx context.Context,
	cfg OrchestraConfig,
	responses []ProviderResponse,
) (*ProviderResponse, *FailedProvider, *FreshJudgeSessionEvidence) {
	judgeCfg := findOrBuildJudgeConfig(cfg)
	judgeAttempt := debateJudgeAttempt(cfg)
	freshJudgeSession := newFreshSubprocessJudgeSessionEvidence()
	if err := freshJudgeConfigError(judgeCfg); err != nil {
		freshJudgeSession.Reason = err.Error()
		failure := buildFailedProviderWithContext(
			judgeCfg, nil, err, cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = noneBackendMarker
		return nil, &failure, freshJudgeSession
	}
	if judgeCfg.Backend == "" && !detect.IsInstalled(judgeCfg.Binary) {
		err := fmt.Errorf("judge binary not found: %s", judgeCfg.Binary)
		freshJudgeSession.Reason = err.Error()
		failure := buildFailedProviderWithContext(
			judgeCfg, nil, err,
			cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = "subprocess"
		return nil, &failure, freshJudgeSession
	}

	progress := NewProgressTracker([]string{judgeCfg.Name})
	stopProgress := progress.StartHeartbeat(ctx, progressHeartbeatInterval)
	defer stopProgress()
	judgment := buildTypedJudgmentPrompt(cfg.Prompt, responses)
	response, err := runConfiguredProvider(
		ctx, cfg, judgeCfg, judgment, "judge", judgeAttempt, progress,
	)
	verifyFreshConfiguredJudgeSession(freshJudgeSession, response, cfg, judgeCfg)
	if response != nil {
		response.freshJudgeSession = freshJudgeSession
	}
	if err != nil || response == nil || response.TimedOut || response.EmptyOutput {
		failure := buildFailedProviderWithContext(
			judgeCfg, response, err, cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = "subprocess"
		if response != nil && response.ExecutedBackend != "" {
			failure.ExecutedBackend = response.ExecutedBackend
		}
		return nil, &failure, freshJudgeSession
	}
	if _, parseErr := (&OutputParser{}).ParseJudge(response.Output); parseErr != nil {
		response.Error = parseErr.Error()
		failure := buildFailedProviderWithContext(
			judgeCfg, response, parseErr, cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = response.ExecutedBackend
		return nil, &failure, freshJudgeSession
	}
	response.Provider = cfg.JudgeProvider + " (judge)"
	return response, nil, freshJudgeSession
}

func verifyFreshConfiguredJudgeSession(
	evidence *FreshJudgeSessionEvidence,
	response *ProviderResponse,
	cfg OrchestraConfig,
	provider ProviderConfig,
) {
	if provider.Backend == "" {
		verifyFreshSubprocessJudgeSession(evidence, response)
		return
	}
	if evidence == nil {
		return
	}
	if response == nil {
		evidence.Reason = "fresh configured judge execution returned no response"
		return
	}
	backend := cfg.ProviderBackends[provider.Backend]
	fresh, ok := backend.(FreshExecutionBackend)
	if !ok || !fresh.FreshExecutionPerRequest() {
		evidence.Reason = fmt.Sprintf(
			"configured judge backend %q does not guarantee fresh execution",
			provider.Backend,
		)
		return
	}
	evidence.Isolated = true
	evidence.Verified = true
	evidence.Reason = "fresh configured backend execution verified"
}

func buildTypedJudgmentPrompt(topic string, responses []ProviderResponse) string {
	prompt := buildJudgmentPrompt(topic, responses)
	schema, err := (&SchemaBuilder{}).EmbedInPrompt("judge")
	if err != nil {
		return prompt + "\n\nReturn only one valid JSON object matching the requested judge fields."
	}
	return prompt + "\n\nReturn only one valid JSON object matching this schema:\n" + schema
}

func debateJudgeAttempt(cfg OrchestraConfig) int {
	rounds := cfg.DebateRounds
	if rounds <= 0 {
		rounds = 1
	}
	return rounds + 1
}
