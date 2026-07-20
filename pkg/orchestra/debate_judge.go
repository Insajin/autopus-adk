package orchestra

import (
	"context"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/detect"
)

func executeDebateJudge(ctx context.Context, cfg OrchestraConfig, responses []ProviderResponse) (*ProviderResponse, *FailedProvider) {
	judgeCfg := findOrBuildJudgeConfig(cfg)
	judgeAttempt := debateJudgeAttempt(cfg)
	if !detect.IsInstalled(judgeCfg.Binary) {
		failure := buildFailedProviderWithContext(
			judgeCfg, nil, fmt.Errorf("judge binary not found: %s", judgeCfg.Binary),
			cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = "subprocess"
		return nil, &failure
	}

	progress := NewProgressTracker([]string{judgeCfg.Name})
	stopProgress := progress.StartHeartbeat(ctx, progressHeartbeatInterval)
	defer stopProgress()
	judgment := buildTypedJudgmentPrompt(cfg.Prompt, responses)
	response, err := runProviderWithProgress(ctx, judgeCfg, judgment, progress)
	applyProviderRequestEvidence(response, ProviderRequest{
		Provider: judgeCfg.Name, Config: judgeCfg, Role: "judge", Round: judgeAttempt,
	}, "subprocess")
	if err != nil || response == nil || response.TimedOut || response.EmptyOutput {
		failure := buildFailedProviderWithContext(
			judgeCfg, response, err, cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = "subprocess"
		if response != nil && response.ExecutedBackend != "" {
			failure.ExecutedBackend = response.ExecutedBackend
		}
		return nil, &failure
	}
	if _, parseErr := (&OutputParser{}).ParseJudge(response.Output); parseErr != nil {
		response.Error = parseErr.Error()
		failure := buildFailedProviderWithContext(
			judgeCfg, response, parseErr, cfg.TimeoutSeconds, "judge", len(responses) > 0,
		)
		failure.Attempt = judgeAttempt
		failure.ExecutedBackend = response.ExecutedBackend
		return nil, &failure
	}
	response.Provider = cfg.JudgeProvider + " (judge)"
	return response, nil
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
