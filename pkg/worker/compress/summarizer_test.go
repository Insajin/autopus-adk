package compress

import (
	"strings"
	"testing"
)

func TestSummarize_StructuredOutput(t *testing.T) {
	input := `## Goal
Implement the compression feature

## Progress
- Added budget.go
- Added compressor.go

## Decision
Use rule-based summarization instead of LLM calls

## Files Modified
- pkg/worker/compress/budget.go
- pkg/worker/compress/compressor.go

## Next Steps
Add tests and integrate into pipeline
`

	result := Summarize("executor", input, 5000)

	required := []string{
		"## Phase Summary: executor",
		"### Goal",
		"Implement the compression feature",
		"### Progress",
		"Added budget.go",
		"### Decisions",
		"rule-based summarization",
		"### Files Modified",
		"pkg/worker/compress/budget.go",
		"### Next Steps",
		"Add tests",
	}
	for _, s := range required {
		if !strings.Contains(result, s) {
			t.Errorf("summary missing %q", s)
		}
	}
}

func TestSummarize_FallbacksWhenEmpty(t *testing.T) {
	result := Summarize("planner", "just some plain text without sections", 5000)

	if !strings.Contains(result, "## Phase Summary: planner") {
		t.Error("missing phase summary header")
	}
	if !strings.Contains(result, "Phase output analysis") {
		t.Error("missing goal fallback")
	}
	if !strings.Contains(result, "No explicit decisions recorded") {
		t.Error("missing decision fallback")
	}
}

func TestSummarize_Truncation(t *testing.T) {
	// Create input that generates a large summary
	longText := "## Goal\n" + strings.Repeat("This is a very detailed goal. ", 500)
	result := Summarize("executor", longText, 50)

	tokens := EstimateTokens(result)
	// Allow some slack for the truncation message itself
	if tokens > 70 {
		t.Errorf("summary too large: %d tokens, want <=70", tokens)
	}
	if !strings.Contains(result, "[...truncated") {
		t.Error("expected truncation marker")
	}
}

func TestExtractFiles(t *testing.T) {
	text := `Modified files:
- pkg/worker/compress/budget.go
- pkg/worker/compress/compressor.go
Also touched internal/config.yaml
And some bare file.go should be skipped
`
	files := extractFiles(text)

	want := map[string]bool{
		"pkg/worker/compress/budget.go":     true,
		"pkg/worker/compress/compressor.go": true,
		"internal/config.yaml":              true,
	}

	for _, f := range files {
		if !want[f] {
			t.Errorf("unexpected file: %s", f)
		}
		delete(want, f)
	}
	for f := range want {
		t.Errorf("missing file: %s", f)
	}
}

func TestExtractSections(t *testing.T) {
	text := `## Goal
Do the thing

## Progress
Did step 1
Did step 2
`
	sections := extractSections(text)

	if sections["goal"] != "Do the thing" {
		t.Errorf("goal = %q, want 'Do the thing'", sections["goal"])
	}
	if !strings.Contains(sections["progress"], "Did step 1") {
		t.Errorf("progress missing 'Did step 1': %q", sections["progress"])
	}
}
