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

	// Phase 3 (optional): judge verdict when JudgeProvider is set, not skipped, and its binary is installed.
	// Resolve the judge's binary first (may differ from JudgeProvider name).
	if cfg.JudgeProvider != "" && !cfg.NoJudge {
		judgeCfg := findOrBuildJudgeConfig(cfg)
		if detect.IsInstalled(judgeCfg.Binary) {
			judgment := buildJudgmentPrompt(cfg.Prompt, responses)
			judgeResp, judgeErr := runProvider(ctx, judgeCfg, judgment)
			if judgeErr == nil && judgeResp != nil && !judgeResp.TimedOut && !judgeResp.EmptyOutput {
				judgeResp.Provider = cfg.JudgeProvider + " (judge)"
				responses = append(responses, *judgeResp)
			}
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
			rebuttalPrompt := buildRebuttalPrompt(cfg.Prompt, others, 2)
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

// topicIsolationInstruction prevents providers from reading project files during debate.
// @AX:NOTE [AUTO] REQ-2 hardcoded prompt prefix — injected by executeRound caller, not by buildRebuttalPrompt
const topicIsolationInstruction = "IMPORTANT: Discuss ONLY the topic below. Do NOT read, reference, or analyze any existing files in the project directory. Focus exclusively on the given discussion topic.\n\n"

// contextAwareInstruction replaces topicIsolationInstruction when --context is set.
const contextAwareInstruction = "Use the project context below to ground your ideas in the actual codebase. Focus on the given topic.\n\n"

// buildRebuttalPrompt creates a cross-pollination prompt with anonymized participant outputs.
// Uses the Acknowledge/Integrate/Risk 3-step structure for structured revision.
// ICE scores from Round 1 are stripped to prevent confidence cascade.
// For round >= 3, each provider's output is truncated to 500 chars to keep prompt size manageable.
// @AX:NOTE [AUTO] REQ-4 magic constant 500 — truncation limit for round >= 3; increase requires prompt budget review
func buildRebuttalPrompt(original string, otherResponses []ProviderResponse, round int) string {
	var sb strings.Builder
	sb.WriteString(original)
	fmt.Fprintf(&sb, "\n\n---\n\n# Round %d: Cross-Pollination\n\n", round)
	sb.WriteString("Other participants' ideas are shown below (anonymized, ICE scores removed).\n\n")

	for i, r := range otherResponses {
		output := stripICEScores(r.Output)
		if round >= 3 && len(output) > 500 {
			output = output[:500] + "[...truncated]"
		}
		alias := fmt.Sprintf("Participant %c", 'A'+rune(i))
		fmt.Fprintf(&sb, "## %s:\n%s\n\n", alias, output)
	}

	sb.WriteString(`Respond in exactly 3 steps:

### Step 1: Acknowledge
Identify the 2-3 strongest points from other participants.
For each, explain **why** it is strong with specific evidence.
Do NOT blindly praise — only acknowledge points with real merit.

### Step 2: Integrate
Your Round 1 core ideas MUST be preserved. Do not abandon them.
Enhance your ideas by incorporating the strongest elements from others.
Describe the integrated proposal with concrete details.

### Step 3: Risk Assessment
Identify 2-3 remaining weaknesses, risks, or implementation barriers
in the integrated proposal. Be specific about assumptions and dependencies.
`)
	return sb.String()
}

// buildJudgmentPrompt creates the judge's synthesis prompt with anonymized debate results.
// Includes structured ICE scoring instructions for consensus-based evaluation.
func buildJudgmentPrompt(topic string, arguments []ProviderResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Role: Final Judge\n\nYou are the final judge for a multi-analyst debate on the topic below.\nParticipant identities are anonymized. Judge purely on content quality.\n\n## Topic\n\n%s\n\n## Debate Results (Anonymized)\n\n", topic)

	for i, r := range arguments {
		alias := fmt.Sprintf("Participant %c", 'A'+rune(i))
		output := stripICEScores(r.Output)
		fmt.Fprintf(&sb, "### %s:\n%s\n\n", alias, output)
	}

	sb.WriteString(`## Judging Instructions

### 1. Consensus Areas
Extract ideas that 2+ participants converged on. Convergence = high confidence signal.
For each: what is the shared idea, which participants agreed, why it matters.

### 2. Unique Insights
Identify ideas proposed by only 1 participant that others did NOT integrate.
These are potentially innovative but also potentially flawed.

### 3. Cross-Risks
Compile risks that 2+ participants independently flagged.
Shared risk identification = likely a real threat.
For each: describe the risk, severity (high/medium/low).

### 4. Top Ideas Ranking (ICE Score)
Select the top 5 ideas and score each:
- **Impact** (1-10): real-world value to the project
- **Confidence** (1-10): consensus level — more participants agreeing = higher
- **Ease** (1-10): implementation feasibility
- **Score** = Impact × Confidence × Ease / 100

### 5. Recommendation
Write a 2-3 sentence actionable recommendation.
`)
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
