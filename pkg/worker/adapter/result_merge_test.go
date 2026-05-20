package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeSequentialResult_GeminiPlainText(t *testing.T) {
	first := TaskResult{Output: "line one"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("gemini", first, true, second)

	assert.Equal(t, "line one\nline two", got.Output)
}

func TestMergeSequentialResult_NonGeminiUsesLatestResult(t *testing.T) {
	first := TaskResult{Output: "line one"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("claude", first, true, second)

	assert.Equal(t, "line two", got.Output)
}

func TestMergeSequentialResult_GeminiStructuredUsesLatestResult(t *testing.T) {
	first := TaskResult{Output: "line one", SessionID: "s1"}
	second := TaskResult{Output: "line two"}

	got := MergeSequentialResult("gemini", first, true, second)

	assert.Equal(t, "line two", got.Output)
}
