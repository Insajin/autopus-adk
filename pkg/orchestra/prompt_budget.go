package orchestra

import "strings"

const (
	// Keep Round 2 peer context bounded so three-provider debates do not explode
	// the rebuttal prompt on long brainstorming outputs.
	rebuttalPromptTotalTokens      = 1400
	rebuttalPromptPerParticipant   = 700
	judgePromptTotalTokens         = 2400
	judgePromptPerParticipant      = 800
	promptBudgetOmittedPlaceholder = "[omitted due to prompt budget]"
)

func capPromptSections(sections []string, totalBudget, perSectionBudget int) []string {
	if len(sections) == 0 {
		return nil
	}

	capped := make([]string, len(sections))
	remainingBudget := totalBudget
	remainingSections := countNonEmptySections(sections)

	for i, section := range sections {
		trimmed := strings.TrimSpace(section)
		if trimmed == "" {
			continue
		}

		sectionBudget := nextPromptSectionBudget(remainingBudget, remainingSections, perSectionBudget)
		if sectionBudget <= 0 {
			capped[i] = promptBudgetOmittedPlaceholder
			remainingSections--
			continue
		}

		capped[i] = TruncateToTokens(trimmed, sectionBudget)
		used := EstimateTokens(capped[i])
		if used > remainingBudget {
			used = remainingBudget
		}
		remainingBudget -= used
		remainingSections--
	}

	return capped
}

func capJudgeResults(results []JudgeResult) []JudgeResult {
	if len(results) == 0 {
		return nil
	}

	capped := make([]JudgeResult, len(results))
	remainingBudget := judgePromptTotalTokens
	remainingParticipants := countJudgeParticipants(results)

	for i, result := range results {
		capped[i].Alias = result.Alias
		if !judgeResultHasContent(result) {
			continue
		}

		participantBudget := nextPromptSectionBudget(remainingBudget, remainingParticipants, judgePromptPerParticipant)
		sections := capPromptSections([]string{result.Round1, result.Round2}, participantBudget, participantBudget)
		capped[i].Round1 = sections[0]
		capped[i].Round2 = sections[1]

		used := EstimateTokens(capped[i].Round1) + EstimateTokens(capped[i].Round2)
		if used > remainingBudget {
			used = remainingBudget
		}
		remainingBudget -= used
		remainingParticipants--
	}

	return capped
}

func nextPromptSectionBudget(remainingBudget, remainingSections, perSectionBudget int) int {
	if remainingBudget <= 0 || remainingSections <= 0 {
		return 0
	}
	budget := remainingBudget / remainingSections
	if budget <= 0 {
		budget = remainingBudget
	}
	if perSectionBudget > 0 && budget > perSectionBudget {
		budget = perSectionBudget
	}
	return budget
}

func countNonEmptySections(sections []string) int {
	count := 0
	for _, section := range sections {
		if strings.TrimSpace(section) != "" {
			count++
		}
	}
	return count
}

func countJudgeParticipants(results []JudgeResult) int {
	count := 0
	for _, result := range results {
		if judgeResultHasContent(result) {
			count++
		}
	}
	return count
}

func judgeResultHasContent(result JudgeResult) bool {
	return strings.TrimSpace(result.Round1) != "" || strings.TrimSpace(result.Round2) != ""
}
