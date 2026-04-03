package compress

import (
	"strings"
	"testing"
)

func TestDefaultCompressor_BelowThreshold(t *testing.T) {
	c := NewDefaultCompressor(2)
	output := "short output that should not be compressed"
	result := c.Compress("executor", output, "claude")
	if result != output {
		t.Error("should return original when below threshold")
	}
}

func TestDefaultCompressor_AboveThreshold(t *testing.T) {
	c := NewDefaultCompressor(1)

	// claude threshold = 100000 tokens = 400000 chars
	largeOutput := "## Goal\nImplement feature X\n\n" +
		"## Progress\nDid many things\n\n" +
		strings.Repeat("<tool_result>some tool output here</tool_result>\n", 50) +
		strings.Repeat("detailed implementation notes ", 15000)

	result := c.Compress("executor", largeOutput, "claude")

	if result == largeOutput {
		t.Error("should compress when above threshold")
	}
	if !strings.Contains(result, "## Phase Summary: executor") {
		t.Error("compressed output should contain structured summary")
	}
	if EstimateTokens(result) > SummaryBudget("claude")+20 {
		t.Errorf("compressed output too large: %d tokens", EstimateTokens(result))
	}
}

func TestDefaultCompressor_KeepRecentTools(t *testing.T) {
	c := NewDefaultCompressor(2)

	// codex threshold = 64000 tokens = 256000 chars
	output := strings.Repeat("x", 260000) +
		"<tool_result>old1</tool_result>\n" +
		"<tool_result>old2</tool_result>\n" +
		"<tool_result>recent1</tool_result>\n" +
		"<tool_result>recent2</tool_result>\n"

	result := c.Compress("tester", output, "codex")
	if result == output {
		t.Error("should compress")
	}
}

func TestNopCompressor(t *testing.T) {
	c := NopCompressor{}
	output := strings.Repeat("a", 500000) // very large
	result := c.Compress("executor", output, "claude")
	if result != output {
		t.Error("NopCompressor should always return original")
	}
}

func TestNewDefaultCompressor_NegativeKeep(t *testing.T) {
	c := NewDefaultCompressor(-1)
	if c.KeepRecentTools != 0 {
		t.Errorf("negative keepRecent should default to 0, got %d", c.KeepRecentTools)
	}
}

func TestContextCompressorInterface(t *testing.T) {
	// Verify both types implement the interface.
	var _ ContextCompressor = &DefaultCompressor{}
	var _ ContextCompressor = NopCompressor{}
}
