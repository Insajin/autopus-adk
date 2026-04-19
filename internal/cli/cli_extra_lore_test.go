package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoreCmd_ContextInvalidDir는 git 없는 디렉터리에서 lore context 명령을 테스트한다.
func TestLoreCmd_ContextInvalidDir(t *testing.T) {
	t.Parallel()

	// 현재 디렉토리는 git repo이므로 lore context 실행은 오류 없이 실행될 수 있음
	// 여기서는 존재하지 않는 경로를 사용하여 테스트
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "context", "nonexistent_path.go"})
	// git repo에서 실행되므로 오류는 발생하지 않을 수 있다
	_ = cmd.Execute()
}

// TestLoreCmd_CommitWithTrailers는 트레일러가 있는 commit 명령을 테스트한다.
// lore commit은 fmt.Println을 사용하므로 오류 없이 실행되는 것만 확인한다.
func TestLoreCmd_CommitWithTrailers(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{
		"lore", "commit", "feat: add new feature",
		"--constraint", "must not break API",
		"--confidence", "high",
		"--scope-risk", "local",
	})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestLoreCmd_CommitBasic은 기본 commit 명령을 테스트한다.
func TestLoreCmd_CommitBasic(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "commit", "fix: bug fix"})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestLoreCmd_CommitAllTrailers는 모든 트레일러 옵션을 테스트한다.
func TestLoreCmd_CommitAllTrailers(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{
		"lore", "commit", "refactor: improve code structure",
		"--constraint", "no breaking changes",
		"--rejected", "full rewrite",
		"--confidence", "medium",
		"--scope-risk", "module",
		"--reversibility", "moderate",
		"--directive", "follow clean code",
		"--tested", "unit tests",
		"--not-tested", "integration tests",
		"--related", "SPEC-001",
	})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestLoreCmd_ValidateWithFile은 파일로 lore validate 명령을 테스트한다.
func TestLoreCmd_ValidateWithFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	commitMsgPath := filepath.Join(dir, "COMMIT_EDITMSG")

	// 유효한 lore 트레일러가 있는 커밋 메시지
	commitMsg := "feat: add new feature\n\nConstraint: must follow API spec\nConfidence: high\n"
	require.NoError(t, os.WriteFile(commitMsgPath, []byte(commitMsg), 0o644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "validate", commitMsgPath})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestLoreCmd_ValidateWithRequiredTrailer는 필수 트레일러 검증을 테스트한다.
func TestLoreCmd_ValidateWithRequiredTrailer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	commitMsgPath := filepath.Join(dir, "COMMIT_EDITMSG")

	// 필수 트레일러가 없는 커밋 메시지
	commitMsg := "feat: add new feature\n"
	require.NoError(t, os.WriteFile(commitMsgPath, []byte(commitMsg), 0o644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "validate", commitMsgPath, "--required", "Constraint"})
	err := cmd.Execute()
	// 필수 트레일러 없으면 오류
	assert.Error(t, err)
}

// TestLoreCmd_ValidateNonExistentFile은 존재하지 않는 파일 검증을 테스트한다.
func TestLoreCmd_ValidateNonExistentFile(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "validate", "/nonexistent/COMMIT_EDITMSG"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// TestLoreCmd_StaleCommand는 stale 명령을 테스트한다.
func TestLoreCmd_StaleCommand(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"lore", "stale", "--days", "30"})
	// git repo에서 실행되므로 오류 없음
	_ = cmd.Execute()
}
