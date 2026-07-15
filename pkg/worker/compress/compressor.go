package compress

import (
	"fmt"
	"log"
	"strings"
)

// ContextCompressor compresses phase output before passing to the next phase.
type ContextCompressor interface {
	// Compress takes the phase name, raw output, and provider name.
	// Returns compressed output if threshold is exceeded, or original output otherwise.
	Compress(phaseName, output, provider string) string
}

// DefaultCompressor implements ContextCompressor using rule-based summarization
// and tool result pruning.
type DefaultCompressor struct {
	// KeepRecentTools is the number of recent tool blocks to preserve.
	KeepRecentTools int
}

// NewDefaultCompressor creates a compressor that keeps the specified number
// of recent tool blocks when pruning.
func NewDefaultCompressor(keepRecentTools int) *DefaultCompressor {
	if keepRecentTools < 0 {
		keepRecentTools = 0
	}
	return &DefaultCompressor{KeepRecentTools: keepRecentTools}
}

// Compress checks if the output exceeds the compression threshold.
// If so, it prunes tool blocks and generates a structured summary.
// If not, it returns the output unchanged (REQ-COMP-005).
func (c *DefaultCompressor) Compress(phaseName, output, provider string) string {
	return c.CompressDetailed(phaseName, output, provider).Output
}

// @AX:ANCHOR: [AUTO] @AX:SPEC: SPEC-CONTEXT-COMPRESS-001: public compaction contract for phase handoff metadata
// @AX:REASON: Pipeline, worker, and orchestra callers use this method to receive output, event metadata, and fail-closed blocker state together.
// CompressDetailed returns compressed output plus compaction event metadata.
func (c *DefaultCompressor) CompressDetailed(phaseName, output, provider string) CompactionResult {
	if !ShouldCompress(output, provider) {
		prune := softPruneToolPairs(output, findToolBlocks(output), c.KeepRecentTools)
		if prune.IncompletePairCount > 0 {
			return CompactionResult{
				Output:  output,
				Event:   buildSoftPruneEvent(phaseName, provider, output, output, prune),
				Blocker: ReasonIncompleteToolPair,
			}
		}
		if prune.PrunedPairCount > 0 {
			return CompactionResult{
				Output: prune.Text,
				Event:  buildSoftPruneEvent(phaseName, provider, output, prune.Text, prune),
			}
		}
		return CompactionResult{
			Output: output,
			Event:  newCompactionEvent(phaseName, provider, output, false),
		}
	}

	log.Printf("[compress] phase %s exceeds threshold for %s, compressing", phaseName, provider)

	// Step 1: Prune old tool results (REQ-COMP-003).
	prune := PruneToolResultsDetailed(output, c.KeepRecentTools)
	report := pruneReport(output, prune.Text)
	if report != "" {
		log.Printf("[compress] %s", report)
	}

	// Step 2: Generate structured summary (REQ-COMP-001).
	budget := SummaryBudget(provider)
	summary := Summarize(phaseName, prune.Text, budget)
	event := buildCompactionEvent(phaseName, provider, output, summary, prune)

	log.Printf("[compress] compressed %d → %d tokens",
		EstimateTokens(output), EstimateTokens(summary))

	result := CompactionResult{Output: summary, Event: event}
	if prune.IncompletePairCount > 0 {
		result.Blocker = ReasonIncompleteToolPair
	} else if strings.Contains(summary, "context-budget") {
		result.Blocker = "context-budget"
	}
	return result
}

func pruneReport(original, pruned string) string {
	if pruned == original {
		return ""
	}
	origResults, origCalls := CountToolBlocks(original)
	newResults, newCalls := CountToolBlocks(pruned)
	return strings.TrimSpace(fmt.Sprintf(
		"pruned tool blocks: results %d→%d, calls %d→%d | %s",
		origResults, newResults, origCalls, newCalls,
		PruneSummary(original, pruned),
	))
}

// NopCompressor is a no-op compressor that always returns the original output.
type NopCompressor struct{}

// Compress returns output unchanged.
func (NopCompressor) Compress(_, output, _ string) string {
	return output
}
