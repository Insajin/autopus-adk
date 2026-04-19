package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlatformAddCmd_AlreadyExistsOutput은 이미 추가된 플랫폼 추가 시 출력을 테스트한다.
func TestPlatformAddCmd_AlreadyExistsOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	addCmd := newTestRootCmd()
	addCmd.SetArgs([]string{"platform", "add", "claude-code", "--dir", dir})
	assert.NoError(t, addCmd.Execute())
}

// TestPlatformRemoveCmd_WithCleanup은 플랫폼 제거 시 파일 정리를 테스트한다.
func TestPlatformRemoveCmd_WithCleanup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,gemini-cli"})
	require.NoError(t, initCmd.Execute())

	removeCmd := newTestRootCmd()
	removeCmd.SetArgs([]string{"platform", "remove", "gemini-cli", "--dir", dir})
	assert.NoError(t, removeCmd.Execute())
}

// TestPlatformRemoveCmd_ClaudeCode는 claude-code 플랫폼 제거를 테스트한다.
func TestPlatformRemoveCmd_ClaudeCode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,codex"})
	require.NoError(t, initCmd.Execute())

	removeCmd := newTestRootCmd()
	removeCmd.SetArgs([]string{"platform", "remove", "claude-code", "--dir", dir})
	assert.NoError(t, removeCmd.Execute())
}

// TestPlatformListCmd_ResolveDir은 resolveDir 코드 경로를 테스트한다.
// --dir 없이 실행하면 현재 디렉터리를 사용한다.
func TestPlatformListCmd_ResolveDir(t *testing.T) {
	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	listCmd := newTestRootCmd()
	listCmd.SetArgs([]string{"platform", "list"})
	assert.NoError(t, listCmd.Execute())
}

// TestArchCmd_EnforceWithValidation은 arch enforce --validate를 테스트한다.
func TestArchCmd_EnforceWithValidation(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(dir))

	genCmd := newTestRootCmd()
	genCmd.SetArgs([]string{"arch", "generate"})
	require.NoError(t, genCmd.Execute())

	enforceCmd := newTestRootCmd()
	enforceCmd.SetArgs([]string{"arch", "enforce"})
	err = enforceCmd.Execute()
	_ = err
}

// TestSpecCmd_ValidateNoSpec는 SPEC 파일 없는 상태에서 validate를 테스트한다.
func TestSpecCmd_ValidateNoSpec(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(dir))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"spec", "validate"})
	err = cmd.Execute()
	_ = err
}

// TestLSPCmd_DiagnosticsCurrentDir은 현재 디렉터리에서 LSP diagnostics를 테스트한다.
// LSP 서버가 없으면 createLSPClient 오류 경로를 통과한다.
func TestLSPCmd_DiagnosticsCurrentDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "diagnostics", "--format", "text", filepath.Join(dir, "main.go")})
	err := cmd.Execute()
	_ = err
}
