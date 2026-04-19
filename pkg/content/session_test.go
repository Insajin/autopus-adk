// Package content_test는 세션 상태 패키지의 테스트이다.
package content_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestSaveAndLoadState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	state := &content.SessionState{
		WorkflowPhase:    "implementation",
		CompletedTasks:   []string{"task-1", "task-2"},
		PendingDecisions: []string{"결제 모듈 선택"},
		ContextSummary:   "현재 인증 모듈 구현 중. JWT 토큰 방식 결정됨.",
	}

	err := content.SaveState(path, state)
	require.NoError(t, err)

	loaded, err := content.LoadState(path)
	require.NoError(t, err)
	assert.Equal(t, state.WorkflowPhase, loaded.WorkflowPhase)
	assert.Equal(t, state.CompletedTasks, loaded.CompletedTasks)
	assert.Equal(t, state.PendingDecisions, loaded.PendingDecisions)
	assert.Equal(t, state.ContextSummary, loaded.ContextSummary)
}

func TestLoadState_NotFound(t *testing.T) {
	t.Parallel()

	_, err := content.LoadState("/nonexistent/path/.auto-continue.md")
	assert.Error(t, err)
}

func TestSaveState_ContextSummaryTruncation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	// 2000 토큰 초과 컨텍스트 (대략 8000자 이상)
	longSummary := make([]byte, 10000)
	for i := range longSummary {
		longSummary[i] = 'a'
	}

	state := &content.SessionState{
		WorkflowPhase:  "test",
		ContextSummary: string(longSummary),
	}

	err := content.SaveState(path, state)
	require.NoError(t, err)

	loaded, err := content.LoadState(path)
	require.NoError(t, err)
	// 2000 토큰 제한으로 잘려야 함 (대략 8000자)
	assert.LessOrEqual(t, len(loaded.ContextSummary), 8001)
}

func TestSessionState_EmptyFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".auto-continue.md")

	state := &content.SessionState{
		WorkflowPhase: "planning",
	}

	err := content.SaveState(path, state)
	require.NoError(t, err)

	loaded, err := content.LoadState(path)
	require.NoError(t, err)
	assert.Equal(t, "planning", loaded.WorkflowPhase)
	assert.Empty(t, loaded.CompletedTasks)
	assert.Empty(t, loaded.PendingDecisions)
}
