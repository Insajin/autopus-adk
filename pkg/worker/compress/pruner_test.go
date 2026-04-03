package compress

import (
	"strings"
	"testing"
)

func TestPruneToolResults_KeepsRecent(t *testing.T) {
	text := `start
<tool_result>result 1</tool_result>
middle
<tool_result>result 2</tool_result>
end
<tool_result>result 3</tool_result>
final`

	pruned := PruneToolResults(text, 1)

	if !strings.Contains(pruned, "result 3") {
		t.Error("should keep the most recent tool_result")
	}
	if strings.Contains(pruned, "result 1") {
		t.Error("should prune old tool_result 1")
	}
	if strings.Contains(pruned, "result 2") {
		t.Error("should prune old tool_result 2")
	}
	if !strings.Contains(pruned, "[tool_result pruned]") {
		t.Error("should contain placeholder")
	}
}

func TestPruneToolResults_NoopWhenBelowThreshold(t *testing.T) {
	text := `<tool_result>only one</tool_result>`
	pruned := PruneToolResults(text, 2)
	if pruned != text {
		t.Error("should not prune when blocks <= keepRecent")
	}
}

func TestPruneToolResults_PrunesToolCalls(t *testing.T) {
	text := `<tool_call>call 1</tool_call>
<tool_call>call 2</tool_call>
<tool_call>call 3</tool_call>`

	pruned := PruneToolResults(text, 1)

	if !strings.Contains(pruned, "call 3") {
		t.Error("should keep the most recent tool_call")
	}
	if strings.Contains(pruned, "call 1") {
		t.Error("should prune old tool_call 1")
	}
}

func TestCountToolBlocks(t *testing.T) {
	text := `<tool_result>r1</tool_result>
<tool_call>c1</tool_call>
<tool_result>r2</tool_result>`

	results, calls := CountToolBlocks(text)
	if results != 2 {
		t.Errorf("results = %d, want 2", results)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestPruneSummary(t *testing.T) {
	original := strings.Repeat("a", 400) // 100 tokens
	pruned := strings.Repeat("a", 200)   // 50 tokens

	summary := PruneSummary(original, pruned)
	if !strings.Contains(summary, "50") {
		t.Errorf("summary should mention saved tokens: %s", summary)
	}
}

func TestPruneAndReport_NoPruning(t *testing.T) {
	text := "plain text without tool blocks"
	result, report := PruneAndReport(text, 2)
	if result != text {
		t.Error("should return original when nothing to prune")
	}
	if report != "" {
		t.Errorf("report should be empty, got: %s", report)
	}
}

func TestPruneAndReport_WithPruning(t *testing.T) {
	text := `<tool_result>r1</tool_result>
<tool_result>r2</tool_result>
<tool_result>r3</tool_result>`

	result, report := PruneAndReport(text, 1)
	if !strings.Contains(result, "[tool_result pruned]") {
		t.Error("should contain placeholder")
	}
	if report == "" {
		t.Error("report should not be empty")
	}
	if !strings.Contains(report, "pruned tool blocks") {
		t.Error("report should describe pruning")
	}
}
