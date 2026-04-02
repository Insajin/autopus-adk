package orchestra

import (
	"fmt"
	"strings"
)

// JudgeBuilder creates the blind judge synthesis prompt.
type JudgeBuilder struct {
	promptBuilder *PromptBuilder
}

// NewJudgeBuilder creates a JudgeBuilder with the given PromptBuilder.
func NewJudgeBuilder(pb *PromptBuilder) *JudgeBuilder {
	return &JudgeBuilder{promptBuilder: pb}
}

// Build creates a judge ProviderRequest from anonymized debate results.
func (jb *JudgeBuilder) Build(baseData PromptData, allResults []JudgeResult) (ProviderRequest, error) {
	data := baseData
	data.AllResults = allResults

	prompt, err := jb.promptBuilder.BuildJudge(data)
	if err != nil {
		return ProviderRequest{}, fmt.Errorf("judge_builder: %w", err)
	}

	return ProviderRequest{
		Prompt: prompt,
		Role:   "judge",
	}, nil
}

// MergeSubprocessResults produces a final markdown document from the judge verdict,
// identity mapping, and individual provider results.
func MergeSubprocessResults(
	judgeOutput *JudgeOutput,
	identityMap map[string]string,
	round1 []ProviderResult,
	round2 []ProviderResult,
) string {
	var sb strings.Builder

	// Judge synthesis
	sb.WriteString("# Orchestra Result\n\n")
	sb.WriteString("## Judge Synthesis\n\n")

	if len(judgeOutput.ConsensusAreas) > 0 {
		sb.WriteString("### Consensus Areas\n\n")
		for _, c := range judgeOutput.ConsensusAreas {
			sb.WriteString(fmt.Sprintf("- **%s** (by %s): %s\n",
				c.Idea, strings.Join(c.Participants, ", "), c.Significance))
		}
		sb.WriteByte('\n')
	}

	if len(judgeOutput.UniqueInsights) > 0 {
		sb.WriteString("### Unique Insights\n\n")
		for _, u := range judgeOutput.UniqueInsights {
			sb.WriteString(fmt.Sprintf("- **%s** (by %s): %s\n",
				u.Idea, u.Proposer, u.WhyMissed))
		}
		sb.WriteByte('\n')
	}

	if len(judgeOutput.CrossRisks) > 0 {
		sb.WriteString("### Cross-Identified Risks\n\n")
		for _, r := range judgeOutput.CrossRisks {
			sb.WriteString(fmt.Sprintf("- [%s] **%s** (flagged by %s)\n",
				r.Severity, r.Risk, strings.Join(r.Flaggers, ", ")))
		}
		sb.WriteByte('\n')
	}

	// ICE-ranked ideas
	if len(judgeOutput.TopIdeas) > 0 {
		sb.WriteString("### Top Ideas (ICE Scored)\n\n")
		sb.WriteString("| Rank | Idea | Impact | Confidence | Ease | Score |\n")
		sb.WriteString("|------|------|--------|------------|------|-------|\n")
		for _, idea := range judgeOutput.TopIdeas {
			sb.WriteString(fmt.Sprintf("| %d | %s | %d | %d | %d | %.2f |\n",
				idea.Rank, idea.Title, idea.Impact, idea.Confidence, idea.Ease, idea.Score))
		}
		sb.WriteByte('\n')
	}

	// Recommendation
	if judgeOutput.Recommendation != "" {
		sb.WriteString("### Recommendation\n\n")
		sb.WriteString(judgeOutput.Recommendation)
		sb.WriteString("\n\n")
	}

	// Per-provider summaries with de-anonymized attribution
	sb.WriteString("## Provider Summaries\n\n")
	for alias, realName := range identityMap {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", realName, alias))
		for _, r := range round1 {
			if r.Provider == realName {
				sb.WriteString("**Round 1:**\n")
				sb.WriteString(truncate(r.Output, 500))
				sb.WriteString("\n\n")
			}
		}
		for _, r := range round2 {
			if r.Provider == realName {
				sb.WriteString("**Round 2:**\n")
				sb.WriteString(truncate(r.Output, 500))
				sb.WriteString("\n\n")
			}
		}
	}

	return sb.String()
}

// truncate shortens text to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
