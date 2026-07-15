package memindex

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func renderContext(query string, results []SearchResult, budgetTokens int) ([]SearchResult, int, string) {
	prefix := fmt.Sprintf("## Quality Recall\n\nQuery: %s\n\n", safeText(query))
	emptyPrompt := renderSelectedContext(prefix, nil, len(results))
	if budgetTokens <= 0 || approxTokens(emptyPrompt) > budgetTokens {
		return nil, len(results), ""
	}
	selected := make([]SearchResult, 0, len(results))
	for _, result := range results {
		candidate := append(append([]SearchResult(nil), selected...), result)
		if approxTokens(renderSelectedContext(prefix, candidate, len(results)-len(candidate))) > budgetTokens {
			break
		}
		selected = append(selected, result)
	}
	omitted := len(results) - len(selected)
	return selected, omitted, renderSelectedContext(prefix, selected, omitted)
}

func renderSelectedContext(prefix string, results []SearchResult, omitted int) string {
	var b strings.Builder
	b.WriteString(prefix)
	for _, result := range results {
		b.WriteString(contextLine(result))
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "\nomitted_results: %d\n", omitted)
	}
	return b.String()
}

func contextLine(result SearchResult) string {
	parts := []string{
		fmt.Sprintf("- [%d] %s", result.Rank, safeText(result.Title)),
		fmt.Sprintf("  source_ref: %s", safeText(result.SourceRef)),
		fmt.Sprintf("  source_hash: %s", safeText(result.SourceHash)),
		fmt.Sprintf("  source_type: %s", result.SourceType),
		fmt.Sprintf("  freshness: %s", result.FreshnessState),
		fmt.Sprintf("  failure_pattern: %s", compact(safeText(result.Title), 180)),
		fmt.Sprintf("  summary: %s", compact(safeText(result.Summary), 260)),
	}
	if result.Severity != "" {
		parts = append(parts, fmt.Sprintf("  severity: %s", result.Severity))
	}
	if len(result.AcceptanceIDs) > 0 {
		parts = append(parts, fmt.Sprintf("  acceptance_refs: %s", safeText(strings.Join(result.AcceptanceIDs, ", "))))
	}
	parts = append(parts, "  next_action: verify source refs before reusing this pattern")
	return strings.Join(parts, "\n") + "\n"
}

func approxTokens(value string) int {
	return promptlayer.EstimateTokens(value)
}
