// Package cli는 init 커맨드 테스트이다.
package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_Default(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	err := cmd.Execute()
	require.NoError(t, err)

	// autopus.yaml 생성 확인
	_, statErr := os.Stat(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, statErr, "autopus.yaml이 생성되어야 함")
}

func TestInitCmd_CreatesAutopusYaml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	err := cmd.Execute()
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, statErr)
}

func TestInitCmd_CreatesGitignore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	err := cmd.Execute()
	require.NoError(t, err)

	// .gitignore 생성 확인
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	content := string(data)

	// autopus 관련 패턴이 있어야 함
	assert.Contains(t, content, ".claude/rules/autopus/")
	assert.Contains(t, content, ".claude/skills/autopus/")
}

func TestInitCmd_MultiplePlatforms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj",
		"--platforms", "claude-code,codex"})
	err := cmd.Execute()
	require.NoError(t, err)

	// autopus.yaml에 플랫폼 목록 확인
	data, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "claude-code")
	assert.Contains(t, content, "codex")
}

func TestInitCmd_ClaudeCodePlatform_CreatesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "my-project", "--platforms", "claude-code"})
	err := cmd.Execute()
	require.NoError(t, err)

	// Claude Code 파일 생성 확인
	_, statErr := os.Stat(filepath.Join(dir, ".claude", "rules", "autopus"))
	require.NoError(t, statErr, ".claude/rules/autopus 디렉터리가 존재해야 함")

	_, statErr = os.Stat(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, statErr, "CLAUDE.md가 존재해야 함")
}

// TestInitCmd_YesFlag verifies --yes flag enables non-interactive mode.
func TestInitCmd_YesFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, statErr, "autopus.yaml must be created with --yes flag")
}

// TestInitCmd_QualityFlag verifies --quality flag sets quality mode preset.
func TestInitCmd_QualityFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes", "--quality", "ultra"})
	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "ultra", "autopus.yaml must contain quality preset 'ultra'")
}

// TestInitCmd_NoReviewGateFlag verifies --no-review-gate disables review gate.
func TestInitCmd_NoReviewGateFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code", "--yes", "--no-review-gate"})
	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	// review_gate section must have enabled: false
	assert.Contains(t, string(data), "enabled: false", "review gate must be disabled")
}
