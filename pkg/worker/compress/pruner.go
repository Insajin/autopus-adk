package compress

import (
	"fmt"
	"regexp"
	"strings"
)

// toolResultPattern matches tool call result blocks in LLM output.
// Supports common formats: <tool_result>...</tool_result>, ```tool_result...```,
// and JSON-style {"type":"tool_result",...}.
var toolResultPattern = regexp.MustCompile(
	`(?s)(<tool_result>.*?</tool_result>|` +
		"```tool_result\\b.*?```" +
		`|\\{"type"\\s*:\\s*"tool_result"[^}]*\\})`,
)

// toolCallPattern matches tool call blocks.
var toolCallPattern = regexp.MustCompile(
	`(?s)(<tool_call>.*?</tool_call>|` +
		"```tool_call\\b.*?```" +
		`|\\{"type"\\s*:\\s*"tool_call"[^}]*\\})`,
)

// PruneToolResults replaces old tool call/result blocks with placeholders.
// It keeps the most recent keepRecent blocks intact and replaces older ones.
func PruneToolResults(text string, keepRecent int) string {
	text = pruneBlocks(text, toolResultPattern, "[tool_result pruned]", keepRecent)
	text = pruneBlocks(text, toolCallPattern, "[tool_call pruned]", keepRecent)
	return text
}

// pruneBlocks replaces all but the last keepRecent matches of pattern
// with a placeholder string.
func pruneBlocks(text string, pattern *regexp.Regexp, placeholder string, keepRecent int) string {
	matches := pattern.FindAllStringIndex(text, -1)
	if len(matches) <= keepRecent {
		return text
	}

	toPrune := matches[:len(matches)-keepRecent]

	// Build result by replacing pruned matches from end to start
	// to preserve indices.
	result := text
	for i := len(toPrune) - 1; i >= 0; i-- {
		start, end := toPrune[i][0], toPrune[i][1]
		result = result[:start] + placeholder + result[end:]
	}
	return result
}

// PruneSummary returns a short description of what was pruned.
func PruneSummary(original, pruned string) string {
	origTokens := EstimateTokens(original)
	prunedTokens := EstimateTokens(pruned)
	saved := origTokens - prunedTokens
	if saved <= 0 {
		return "no tokens saved by pruning"
	}
	return fmt.Sprintf("pruned %d tokens (%d → %d)", saved, origTokens, prunedTokens)
}

// CountToolBlocks returns the number of tool_result and tool_call blocks.
func CountToolBlocks(text string) (results int, calls int) {
	results = len(toolResultPattern.FindAllStringIndex(text, -1))
	calls = len(toolCallPattern.FindAllStringIndex(text, -1))
	return
}

// PruneAndReport prunes tool blocks and returns the pruned text plus a report line.
func PruneAndReport(text string, keepRecent int) (string, string) {
	pruned := PruneToolResults(text, keepRecent)
	if pruned == text {
		return text, ""
	}

	origResults, origCalls := CountToolBlocks(text)
	newResults, newCalls := CountToolBlocks(pruned)

	report := fmt.Sprintf(
		"pruned tool blocks: results %d→%d, calls %d→%d | %s",
		origResults, newResults, origCalls, newCalls,
		PruneSummary(text, pruned),
	)
	return pruned, strings.TrimSpace(report)
}
