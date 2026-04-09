package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPopulateMemory_NilSearcher(t *testing.T) {
	t.Parallel()
	result := populateMemory(context.Background(), nil, "agent-1", "deploy the service")
	assert.Equal(t, "", result)
}

func TestPopulateMemory_EmptyDescription(t *testing.T) {
	t.Parallel()
	result := populateMemory(context.Background(), nil, "agent-1", "")
	assert.Equal(t, "", result)
}

func TestTruncateForMemory_ShortContent(t *testing.T) {
	t.Parallel()
	description := "fix the bug"
	output := "applied nil check"
	got := truncateForMemory(description, output)
	assert.Contains(t, got, description)
	assert.Contains(t, got, output)
	assert.LessOrEqual(t, len(got), 500)
}

func TestTruncateForMemory_LongContentTruncated(t *testing.T) {
	t.Parallel()
	description := "short"
	output := strings.Repeat("x", 600)
	got := truncateForMemory(description, output)
	assert.Len(t, got, 500)
}

func TestTruncateForMemory_ExactLimit(t *testing.T) {
	t.Parallel()
	// Build a string that lands exactly at 500 chars after formatting.
	// "Task: d\nResult summary: " = 24 chars, so output needs 500-24=476 chars.
	description := "d"
	output := strings.Repeat("y", 476)
	got := truncateForMemory(description, output)
	assert.Len(t, got, 500)
}
