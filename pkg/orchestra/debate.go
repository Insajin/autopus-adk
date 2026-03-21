package orchestra

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// runDebate executes the full debate flow:
// Phase 1 (parallel arguments) → optional Phase 2 (rebuttal) → optional judgment.
func runDebate(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, error) {
	// Phase 1: all debaters respond to original prompt in parallel
	responses, _, err := runParallel(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Phase 2 (optional): rebuttal round when DebateRounds >= 2
	rounds := cfg.DebateRounds
	if rounds <= 0 {
		rounds = 1
	}
	if rounds >= 2 && len(responses) >= 2 {
		rebuttalResps, rebuttalErr := runRebuttalRound(ctx, cfg, responses)
		if rebuttalErr == nil && len(rebuttalResps) > 0 {
			responses = rebuttalResps
		}
	}

	// Phase 3 (optional): judge verdict when JudgeProvider binary is installed
	if cfg.JudgeProvider != "" && detect.IsInstalled(cfg.JudgeProvider) {
		judgeCfg := findOrBuildJudgeConfig(cfg)
		judgment := buildJudgmentPrompt(cfg.Prompt, responses)
		judgeResp, judgeErr := runProvider(ctx, judgeCfg, judgment)
		if judgeErr == nil && judgeResp != nil {
			judgeResp.Provider = cfg.JudgeProvider + " (judge)"
			responses = append(responses, *judgeResp)
		}
	}

	return responses, nil
}

// runRebuttalRound executes one rebuttal round for each debater.
// Each debater receives the original prompt plus all other debaters' responses.
func runRebuttalRound(ctx context.Context, cfg OrchestraConfig, prevResponses []ProviderResponse) ([]ProviderResponse, error) {
	rebuttalResults := make([]providerResult, len(cfg.Providers))
	var wg sync.WaitGroup

	for i, p := range cfg.Providers {
		wg.Add(1)
		go func(idx int, provider ProviderConfig) {
			defer wg.Done()
			// Collect other debaters' responses (exclude current provider)
			var others []ProviderResponse
			for _, r := range prevResponses {
				if r.Provider != provider.Name {
					others = append(others, r)
				}
			}
			rebuttalPrompt := buildRebuttalPrompt(cfg.Prompt, others)
			resp, err := runProvider(ctx, provider, rebuttalPrompt)
			if err != nil {
				rebuttalResults[idx] = providerResult{err: err, idx: idx}
				return
			}
			rebuttalResults[idx] = providerResult{resp: *resp, idx: idx}
		}(i, p)
	}
	wg.Wait()

	var responses []ProviderResponse
	for _, r := range rebuttalResults {
		if r.err == nil {
			responses = append(responses, r.resp)
		}
	}
	if len(responses) == 0 {
		if len(rebuttalResults) > 0 {
			return nil, rebuttalResults[0].err
		}
		return nil, fmt.Errorf("rebuttal round: no providers configured")
	}
	return responses, nil
}

// buildRebuttalPrompt creates a rebuttal prompt including other debaters' arguments.
func buildRebuttalPrompt(original string, otherResponses []ProviderResponse) string {
	var sb strings.Builder
	sb.WriteString(original)
	sb.WriteString("\n\n## Other debaters' arguments:\n")
	for _, r := range otherResponses {
		sb.WriteString(fmt.Sprintf("\n### %s:\n%s\n", r.Provider, r.Output))
	}
	sb.WriteString("\nPlease provide your rebuttal:")
	return sb.String()
}

// buildJudgmentPrompt creates the judge's prompt with all arguments.
func buildJudgmentPrompt(topic string, arguments []ProviderResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Topic: %s\n\n## Arguments:\n", topic))
	for _, r := range arguments {
		sb.WriteString(fmt.Sprintf("\n### %s:\n%s\n", r.Provider, r.Output))
	}
	sb.WriteString("\nAs the judge, render a final verdict:")
	return sb.String()
}

// buildDebateMerged formats the debate result and builds the summary.
// If the last response is from the judge, it is noted in the summary.
func buildDebateMerged(responses []ProviderResponse, cfg OrchestraConfig) (string, string) {
	if len(responses) == 0 {
		return "", "토론 결과 없음"
	}

	judgeVerdict := ""
	judgePresent := false

	// Check if the last response is from the judge
	last := responses[len(responses)-1]
	if cfg.JudgeProvider != "" && strings.HasPrefix(last.Provider, cfg.JudgeProvider) {
		judgeVerdict = last.Output
		judgePresent = true
	}

	merged := FormatDebate(responses)

	judgeLabel := cfg.JudgeProvider
	if judgeLabel == "" {
		judgeLabel = "없음"
	}

	var summary string
	if judgePresent {
		preview := judgeVerdict
		if len(preview) > 50 {
			preview = preview[:50]
		}
		summary = fmt.Sprintf("토론 완료, 판정: %s (verdict: %s)", judgeLabel, preview)
	} else {
		summary = fmt.Sprintf("토론 완료, 판정: %s", judgeLabel)
	}

	return merged, summary
}

// findOrBuildJudgeConfig finds the judge's ProviderConfig from cfg.Providers,
// or creates a default one with Name and Binary both set to JudgeProvider.
func findOrBuildJudgeConfig(cfg OrchestraConfig) ProviderConfig {
	for _, p := range cfg.Providers {
		if p.Name == cfg.JudgeProvider {
			return p
		}
	}
	return ProviderConfig{
		Name:   cfg.JudgeProvider,
		Binary: cfg.JudgeProvider,
	}
}
