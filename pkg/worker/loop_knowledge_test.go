package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMemoryAgentID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  LoopConfig
		want string
	}{
		{
			name: "prefers explicit memory agent id",
			cfg: LoopConfig{
				WorkerName:    "adk-worker-codex",
				MemoryAgentID: "11111111-2222-4333-8444-555555555555",
			},
			want: "11111111-2222-4333-8444-555555555555",
		},
		{
			name: "falls back to worker name when uuid",
			cfg: LoopConfig{
				WorkerName: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
			},
			want: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		},
		{
			name: "returns empty when no valid uuid exists",
			cfg: LoopConfig{
				WorkerName: "adk-worker-claude",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveMemoryAgentID(tt.cfg))
		})
	}
}

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

func TestBuildKnowledgeQuery_ShortDescription(t *testing.T) {
	t.Parallel()

	description := "runtime canary"
	assert.Equal(t, "runtime canary", buildKnowledgeQuery(description))
}

func TestBuildKnowledgeQuery_LongDescription_CompressesToKeywords(t *testing.T) {
	t.Parallel()

	description := "In the current workspace, create a file named runtime-postfix-canary-20260413-2303.md containing exactly three lines: Post-fix canary, 20260413-2303, ok. Do not modify any other files."
	got := buildKnowledgeQuery(description)

	assert.NotContains(t, got, "exactly three lines")
	assert.LessOrEqual(t, len(got), 120)
	assert.Contains(t, got, "runtime-postfix-canary-20260413-2303.md")
	assert.Contains(t, got, "workspace")
}
