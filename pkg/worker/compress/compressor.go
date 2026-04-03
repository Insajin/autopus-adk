package compress

import (
	"log"
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
	if !ShouldCompress(output, provider) {
		return output
	}

	log.Printf("[compress] phase %s exceeds threshold for %s, compressing", phaseName, provider)

	// Step 1: Prune old tool results (REQ-COMP-003).
	pruned, report := PruneAndReport(output, c.KeepRecentTools)
	if report != "" {
		log.Printf("[compress] %s", report)
	}

	// Step 2: Generate structured summary (REQ-COMP-001).
	budget := SummaryBudget(provider)
	summary := Summarize(phaseName, pruned, budget)

	log.Printf("[compress] compressed %d → %d tokens",
		EstimateTokens(output), EstimateTokens(summary))

	return summary
}

// NopCompressor is a no-op compressor that always returns the original output.
type NopCompressor struct{}

// Compress returns output unchanged.
func (NopCompressor) Compress(_, output, _ string) string {
	return output
}
