// Package cli_test는 internal/cli 패키지 커버리지 향상을 위한 추가 테스트이다.
package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateCmd_AllPlatforms는 여러 플랫폼이 있을 때 update를 테스트한다.
func TestUpdateCmd_AllPlatforms(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,codex,gemini-cli"})
	require.NoError(t, initCmd.Execute())

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	assert.NoError(t, updateCmd.Execute())
}

// TestUpdateCmd_MultiplePlatformsOutput은 update 출력 내용을 테스트한다.
func TestUpdateCmd_MultiplePlatformsOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "multi-proj", "--platforms", "claude-code,codex"})
	require.NoError(t, initCmd.Execute())

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute())
}

// TestInitCmd_FullModeGemini는 Full 모드로 gemini-cli를 초기화를 테스트한다.
func TestInitCmd_FullModeGemini(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "full-gemini", "--platforms", "gemini-cli"})
	require.NoError(t, cmd.Execute())

	_, statErr := os.Stat(filepath.Join(dir, "GEMINI.md"))
	assert.NoError(t, statErr)
}

// TestInitCmd_GeminiPlatform는 gemini-cli 플랫폼 초기화를 테스트한다.
func TestInitCmd_GeminiPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "gemini-proj", "--platforms", "gemini-cli"})
	require.NoError(t, cmd.Execute())

	_, statErr := os.Stat(filepath.Join(dir, "GEMINI.md"))
	assert.NoError(t, statErr)
}

// TestInitCmd_CodexPlatform는 codex 플랫폼 초기화를 테스트한다.
func TestInitCmd_CodexPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "codex-proj", "--platforms", "codex"})
	require.NoError(t, cmd.Execute())

	_, statErr := os.Stat(filepath.Join(dir, "AGENTS.md"))
	assert.NoError(t, statErr)
}

// TestInitCmd_UpdatesGitignore는 .gitignore 업데이트를 테스트한다.
func TestInitCmd_UpdatesGitignore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "gitignore-proj", "--platforms", "claude-code"})
	require.NoError(t, cmd.Execute())

	gitignorePath := filepath.Join(dir, ".gitignore")
	_, statErr := os.Stat(gitignorePath)
	assert.NoError(t, statErr)
}

// TestInitCmd_ExistingGitignore는 기존 .gitignore에 패턴 추가를 테스트한다.
func TestInitCmd_ExistingGitignore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	gitignorePath := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("node_modules/\n*.log\n"), 0644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "existing-gi", "--platforms", "claude-code"})
	require.NoError(t, cmd.Execute())

	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "node_modules/")
}

// TestInitCmd_ProjectNameFromDir은 --project 없이 디렉터리 이름을 프로젝트 이름으로 사용하는지 테스트한다.
func TestInitCmd_ProjectNameFromDir(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(dir))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--platforms", "claude-code"})
	require.NoError(t, cmd.Execute())

	data, readErr := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, readErr)
	assert.True(t, strings.Contains(string(data), filepath.Base(dir)))
}

// TestUpdateCmd_GeminiPlatform은 gemini-cli 플랫폼의 update를 테스트한다.
func TestUpdateCmd_GeminiPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "gemini-update", "--platforms", "gemini-cli"})
	require.NoError(t, initCmd.Execute())

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	assert.NoError(t, updateCmd.Execute())
}

// TestUpdateCmd_CodexPlatform은 codex 플랫폼의 update를 테스트한다.
func TestUpdateCmd_CodexPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "codex-update", "--platforms", "codex"})
	require.NoError(t, initCmd.Execute())

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	assert.NoError(t, updateCmd.Execute())
}
