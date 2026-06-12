package worker

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
)

func TestResolveTaskTraceID_Precedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		taskID  string
		pending a2a.ApprovalRequestParams
		meta    taskRunMeta
		want    string
	}{
		{
			name:    "prefers pending trace id",
			taskID:  "task-1",
			pending: a2a.ApprovalRequestParams{TraceID: " trace-pending "},
			meta:    taskRunMeta{TraceID: "trace-meta"},
			want:    "trace-pending",
		},
		{
			name:    "falls back to meta trace id",
			taskID:  "task-1",
			pending: a2a.ApprovalRequestParams{},
			meta:    taskRunMeta{TraceID: "trace-meta"},
			want:    "trace-meta",
		},
		{
			name:    "falls back to task id when nothing else",
			taskID:  "task-1",
			pending: a2a.ApprovalRequestParams{},
			meta:    taskRunMeta{},
			want:    "task-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveTaskTraceID(tt.taskID, tt.pending, tt.meta))
		})
	}
}

func TestBuildExecutionContextSnapshot_WorkspaceMode(t *testing.T) {
	t.Parallel()

	ctx := buildExecutionContextSnapshot(
		LoopConfig{WorkspaceID: " ws-1 "},
		"/root/work",
		"", // empty active falls back to root
		"",
	)
	assert.Equal(t, "ws-1", ctx.WorkspaceID)
	assert.Equal(t, "/root/work", ctx.RootWorkDir)
	assert.Equal(t, "/root/work", ctx.ActiveWorkDir)
	assert.Equal(t, "workspace", ctx.Mode)
	assert.Empty(t, ctx.WorktreePath)
}

func TestBuildExecutionContextSnapshot_TemporaryWorktreeMode(t *testing.T) {
	t.Parallel()

	ctx := buildExecutionContextSnapshot(
		LoopConfig{WorkspaceID: "ws-2"},
		"/root/work",
		"/root/work/active",
		"/tmp/worktree/../worktree/task-1",
	)
	assert.Equal(t, "temporary_worktree", ctx.Mode)
	assert.Equal(t, "/tmp/worktree/task-1", ctx.WorktreePath)
	assert.Equal(t, "/root/work/active", ctx.ActiveWorkDir)
	assert.Contains(t, ctx.BoundaryHint, "temporary git worktree")
}

func TestDescribeExecutionContext_Branches(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"task execution context is not available yet",
		describeExecutionContext(nil),
	)
	assert.Equal(t,
		"task running inside temporary worktree /tmp/wt",
		describeExecutionContext(&HostExecutionContext{Mode: "temporary_worktree", WorktreePath: "/tmp/wt"}),
	)
	assert.Equal(t,
		"task running in retained workspace /root/active",
		describeExecutionContext(&HostExecutionContext{Mode: "workspace", ActiveWorkDir: "/root/active"}),
	)
	assert.Equal(t,
		"task running in the retained workspace",
		describeExecutionContext(&HostExecutionContext{Mode: "workspace"}),
	)
}

func TestCleanPath_EmptyAndNormalized(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", cleanPath("   "))
	assert.Equal(t, "/a/b", cleanPath("/a/./b/"))
}

func TestWorktreePath_DistinguishesActiveFromRoot(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", worktreePath("/root", "/root"))
	assert.Equal(t, "", worktreePath("", "/root"))
	assert.Equal(t, "/root/wt", worktreePath("/root/wt/", "/root"))
}

func TestPreviewArtifact_TextualVsBinary(t *testing.T) {
	t.Parallel()

	// Binary mime, non-output name -> not previewed.
	assert.Equal(t, "", previewArtifact(adapter.Artifact{
		Name:     "img.png",
		MimeType: "image/png",
		Data:     "binary",
	}))
	// Textual mime -> previewed and whitespace collapsed.
	assert.Equal(t, "hello world", previewArtifact(adapter.Artifact{
		Name:     "log.txt",
		MimeType: "text/plain",
		Data:     "  hello   world  ",
	}))
	// Name "output" forces textual treatment even with empty mime.
	assert.Equal(t, "summary", previewArtifact(adapter.Artifact{
		Name: "output",
		Data: "summary",
	}))
}

func TestSummarizeArtifacts_EmptyAndPopulated(t *testing.T) {
	t.Parallel()

	assert.Nil(t, summarizeArtifacts(nil))

	out := summarizeArtifacts([]adapter.Artifact{
		{Name: " notes.md ", MimeType: " text/markdown ", Data: "content"},
	})
	if assert.Len(t, out, 1) {
		assert.Equal(t, "notes.md", out[0].Name)
		assert.Equal(t, "text/markdown", out[0].MimeType)
		assert.Equal(t, "content", out[0].Preview)
		assert.Equal(t, "worker_result", out[0].Source)
	}
}

func TestBuildHostResult_SummaryFallbacks(t *testing.T) {
	t.Parallel()

	// Empty summary, failed status, no artifacts -> failure default summary.
	failed := buildHostResult("failed", "  ", "boom", adapter.TaskResult{})
	assert.Equal(t, "failed", failed.Status)
	assert.Equal(t, "Task failed before returning a retained result.", failed.Summary)
	assert.Equal(t, "boom", failed.ErrorMessage)

	// Empty summary, completed status, no artifacts -> completion default summary.
	done := buildHostResult("completed", "", "", adapter.TaskResult{CostUSD: 0.5, DurationMS: 1200, SessionID: " sess "})
	assert.Equal(t, "Task completed and retained a desktop-visible result summary.", done.Summary)
	assert.Equal(t, "$0.5000", done.CostLabel)
	assert.Equal(t, int64(1200), done.DurationMS)
	assert.Equal(t, "sess", done.SessionID)

	// Output drives the summary when no explicit summary provided.
	withOutput := buildHostResult("completed", "", "", adapter.TaskResult{Output: "did the thing"})
	assert.Equal(t, "did the thing", withOutput.Summary)
}
