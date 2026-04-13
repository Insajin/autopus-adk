//go:build integration

// Package cli는 auto init E2E 통합 테스트이다.
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmd는 루트 커맨드를 실행하고 stdout을 반환한다.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newTestRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// TestInit_CreatesCorrectFiles는 init이 올바른 파일을 생성하는지 검증한다.
func TestInit_CreatesCorrectFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := runCmd(t, "init", "--dir", dir, "--project", "test-project", "--platforms", "claude-code")
	require.NoError(t, err)

	// autopus.yaml이 생성되어야 함
	yamlPath := filepath.Join(dir, "autopus.yaml")
	require.FileExists(t, yamlPath)

	// .claude/ 디렉터리 구조 생성 확인
	assert.DirExists(t, filepath.Join(dir, ".claude", "rules", "autopus"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "skills", "autopus"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "commands"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "agents", "autopus"))
	// 라우터 커맨드 파일 존재 확인
	assert.FileExists(t, filepath.Join(dir, ".claude", "commands", "auto.md"))
	// autopus 커맨드 디렉터리는 생성되지 않아야 함
	assert.NoDirExists(t, filepath.Join(dir, ".claude", "commands", "autopus"))

	// .gitignore 패턴 추가 확인
	gitignorePath := filepath.Join(dir, ".gitignore")
	require.FileExists(t, gitignorePath)
	gitignoreData, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	gitignoreContent := string(gitignoreData)
	assert.Contains(t, gitignoreContent, ".claude/rules/autopus/")
	assert.Contains(t, gitignoreContent, ".claude/skills/autopus/")
	assert.Contains(t, gitignoreContent, ".codex/skills/")
	assert.Contains(t, gitignoreContent, ".gemini/skills/autopus/")
	assert.Contains(t, gitignoreContent, ".agents/plugins/")
}

// TestInit_CreatesAllContent는 init이 전체 콘텐츠를 생성하는지 검증한다.
func TestInit_CreatesAllContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := runCmd(t, "init", "--dir", dir, "--project", "full-project", "--platforms", "claude-code")
	require.NoError(t, err)

	// autopus.yaml 생성 확인
	yamlPath := filepath.Join(dir, "autopus.yaml")
	require.FileExists(t, yamlPath)

	// .claude/ 디렉터리 구조 생성 확인
	assert.DirExists(t, filepath.Join(dir, ".claude", "rules", "autopus"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "skills", "autopus"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "commands"))
	assert.DirExists(t, filepath.Join(dir, ".claude", "agents", "autopus"))
	// 라우터 커맨드 파일 존재 확인
	assert.FileExists(t, filepath.Join(dir, ".claude", "commands", "auto.md"))

	// CLAUDE.md 생성 확인
	claudePath := filepath.Join(dir, "CLAUDE.md")
	require.FileExists(t, claudePath)
	claudeData, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	assert.Contains(t, string(claudeData), "full-project")
	assert.Contains(t, string(claudeData), "<!-- AUTOPUS:BEGIN -->")
	assert.Contains(t, string(claudeData), "<!-- AUTOPUS:END -->")
}

// TestUpdate_PreservesUserModifications는 update가 마커 외부 사용자 수정을 보존하는지 검증한다.
func TestUpdate_PreservesUserModifications(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// init 먼저 실행
	_, err := runCmd(t, "init", "--dir", dir, "--project", "update-proj", "--platforms", "claude-code")
	require.NoError(t, err)

	claudePath := filepath.Join(dir, "CLAUDE.md")
	require.FileExists(t, claudePath)

	// 마커 외부에 사용자 콘텐츠 추가
	data, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	userSection := "\n\n## My Custom Rules\n\nThese are user-defined rules that must be preserved.\n"
	err = os.WriteFile(claudePath, append(data, []byte(userSection)...), 0o644)
	require.NoError(t, err)

	// update 실행
	_, err = runCmd(t, "update", "--dir", dir)
	require.NoError(t, err)

	// 사용자 수정 사항이 보존되어야 함
	updated, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	updatedStr := string(updated)

	assert.Contains(t, updatedStr, "My Custom Rules")
	assert.Contains(t, updatedStr, "These are user-defined rules that must be preserved.")

	// autopus 마커 섹션도 여전히 존재해야 함
	assert.Contains(t, updatedStr, "<!-- AUTOPUS:BEGIN -->")
	assert.Contains(t, updatedStr, "<!-- AUTOPUS:END -->")
}

// TestDoctor_ReportsHealth는 doctor 커맨드가 유효/무효 설정에 대해 올바른 상태를 보고하는지 검증한다.
func TestDoctor_ReportsHealth(t *testing.T) {
	t.Parallel()

	t.Run("valid_setup", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// 먼저 init으로 유효한 상태 구성
		_, err := runCmd(t, "init", "--dir", dir, "--project", "doctor-proj", "--platforms", "claude-code")
		require.NoError(t, err)

		// doctor 실행
		out, err := runCmd(t, "doctor", "--dir", dir)
		require.NoError(t, err)

		// 정상 상태 보고 확인
		assert.Contains(t, out, "Autopus")
		assert.Contains(t, out, "[OK] autopus.yaml")
		// 플랫폼 검증 OK
		assert.Contains(t, out, "[OK] claude-code")
	})

	t.Run("invalid_setup_no_config", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// autopus.yaml 없이 doctor 실행

		out, err := runCmd(t, "doctor", "--dir", dir)
		// doctor는 오류를 반환하지 않고 출력으로 보고함
		require.NoError(t, err)

		// 설정 로드 실패 메시지 확인
		assert.Contains(t, out, "Autopus")
		assert.Contains(t, out, "ERROR")
	})
}

// TestMultiPlatform_Init는 여러 플랫폼으로 init 시 각 플랫폼 파일이 생성되는지 검증한다.
func TestMultiPlatform_Init(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := runCmd(t, "init", "--dir", dir, "--project", "multi-proj",
		"--platforms", "claude-code,codex,gemini-cli")
	require.NoError(t, err)

	// autopus.yaml에 모든 플랫폼 포함 확인
	yamlData, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	yamlStr := string(yamlData)
	assert.Contains(t, yamlStr, "claude-code")
	assert.Contains(t, yamlStr, "codex")
	assert.Contains(t, yamlStr, "gemini-cli")

	// Claude Code 파일 생성 확인
	assert.DirExists(t, filepath.Join(dir, ".claude", "rules", "autopus"))
	assert.FileExists(t, filepath.Join(dir, "CLAUDE.md"))

	// Codex 파일 생성 확인
	assert.DirExists(t, filepath.Join(dir, ".codex"))

	// Gemini CLI 파일 생성 확인
	assert.DirExists(t, filepath.Join(dir, ".gemini"))

	// .gitignore에 모든 플랫폼 패턴 포함 확인
	gitignoreData, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	gitignoreStr := string(gitignoreData)
	assert.Contains(t, gitignoreStr, ".claude/rules/autopus/")
	assert.Contains(t, gitignoreStr, ".codex/skills/")
	assert.Contains(t, gitignoreStr, ".gemini/skills/autopus/")
	assert.Contains(t, gitignoreStr, ".agents/plugins/")

	// 컨텍스트 격리 검증: 각 플랫폼 파일은 다른 플랫폼 고유 내용을 포함하지 않아야 함
	claudeMD, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	claudeStr := string(claudeMD)
	// CLAUDE.md는 Claude Code 전용 마커 포함
	assert.Contains(t, claudeStr, "<!-- AUTOPUS:BEGIN -->")
	// CLAUDE.md는 codex 전용 섹션을 포함하지 않아야 함
	assert.False(t, strings.Contains(claudeStr, "CODEX:BEGIN"),
		"CLAUDE.md는 Codex 전용 마커를 포함하면 안 됩니다")
}
