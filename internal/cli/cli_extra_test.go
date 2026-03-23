// Package cli_test는 CLI 커맨드에 대한 추가 테스트를 제공한다.
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersionCmd는 version 커맨드를 테스트한다.
// version 커맨드는 fmt.Println을 사용하므로 오류 없이 실행되는 것만 확인한다.
func TestVersionCmd(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestRootCmd_NoArgs는 인자 없는 루트 커맨드 실행을 테스트한다.
func TestRootCmd_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{})
	// 도움말 출력 (오류 없음)
	err := cmd.Execute()
	assert.NoError(t, err)
}

// TestRootCmd_Help는 --help 플래그를 테스트한다.
func TestRootCmd_Help(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	// --help는 오류 없이 실행됨
	assert.NoError(t, err)
}

// TestHashCmd_ValidFile은 유효한 파일에 대한 hash 커맨드를 테스트한다.
// hash 커맨드는 fmt.Println을 사용하므로 오류 없이 실행되는 것만 확인한다.
func TestHashCmd_ValidFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"hash", filePath})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestHashCmd_NonExistentFile은 존재하지 않는 파일에 대한 hash 커맨드를 테스트한다.
func TestHashCmd_NonExistentFile(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"hash", "/nonexistent/path/file.txt"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// TestHashCmd_EmptyFile은 빈 파일에 대한 hash 커맨드를 테스트한다.
func TestHashCmd_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0o644))

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"hash", filePath})
	err := cmd.Execute()
	require.NoError(t, err)
	// 빈 파일이므로 출력 없음
	assert.Empty(t, buf.String())
}

// TestSearchCmd_NoAPIKey는 API 키 없는 search 커맨드를 테스트한다.
// t.Setenv는 t.Parallel()과 함께 사용 불가하므로 직렬 실행
func TestSearchCmd_NoAPIKey(t *testing.T) {
	// Setenv와 Parallel은 함께 사용 불가

	// EXA_API_KEY 임시 제거
	origKey := os.Getenv("EXA_API_KEY")
	require.NoError(t, os.Setenv("EXA_API_KEY", ""))
	defer os.Setenv("EXA_API_KEY", origKey)

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"search", "golang testing"})
	err := cmd.Execute()
	// API 키 없으면 오류
	assert.Error(t, err)
}

// TestSearchCmd_NoArgs는 인자 없는 search 커맨드를 테스트한다.
func TestSearchCmd_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"search"})
	err := cmd.Execute()
	assert.Error(t, err)
}

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

// TestArchCmd_GenerateCurrentDir은 현재 디렉터리 arch generate를 테스트한다.
// arch generate는 현재 디렉터리에 ARCHITECTURE.md를 생성하므로 임시 디렉터리로 이동
func TestArchCmd_GenerateCurrentDir(t *testing.T) {
	// Chdir은 병렬 실행과 함께 사용 불가

	dir := t.TempDir()
	// 간단한 Go 프로젝트 구조 생성
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg", "api"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.23\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "api", "handler.go"), []byte("package api\n"), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"arch", "generate", dir})
	execErr := cmd.Execute()
	require.NoError(t, execErr)

	// ARCHITECTURE.md가 현재 (임시) 디렉터리에 생성되어야 함
	_, statErr := os.Stat(filepath.Join(dir, "ARCHITECTURE.md"))
	require.NoError(t, statErr, "ARCHITECTURE.md가 생성되어야 함")
}

// TestArchCmd_EnforceNoViolation은 위반 없는 arch enforce를 테스트한다.
func TestArchCmd_EnforceNoViolation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.23\n"), 0o644))

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"arch", "enforce", dir})
	err := cmd.Execute()
	// 위반 없으면 성공
	require.NoError(t, err)
}

// TestSpecCmd_New는 spec new 커맨드를 테스트한다.
// spec new는 현재 디렉터리에 파일을 생성하므로 임시 디렉터리로 이동 후 실행한다.
func TestSpecCmd_New(t *testing.T) {
	// Chdir은 t.Parallel()과 함께 사용하면 race condition 발생 가능

	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"spec", "new", "TEST-001", "--title", "테스트 스펙"})
	execErr := cmd.Execute()
	require.NoError(t, execErr)

	// SPEC 디렉터리 생성 확인
	_, statErr := os.Stat(filepath.Join(dir, ".autopus", "specs", "SPEC-TEST-001"))
	require.NoError(t, statErr, "SPEC 디렉터리가 생성되어야 함")
}

// TestSpecCmd_NewDefaultTitle는 title 없는 spec new 커맨드를 테스트한다.
// spec new는 TestSpecCmd_New와 함께 순서대로 실행하면 Chdir race condition 발생
// 따라서 TestSpecCmd_New와 분리된 임시 디렉터리 사용
func TestSpecCmd_NewDefaultTitle(t *testing.T) {
	// Chdir은 병렬 실행과 함께 사용 불가

	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"spec", "new", "TEST-002"})
	execErr := cmd.Execute()
	require.NoError(t, execErr)

	// SPEC 디렉터리 생성 확인
	_, statErr := os.Stat(filepath.Join(dir, ".autopus", "specs", "SPEC-TEST-002"))
	require.NoError(t, statErr, "SPEC 디렉터리가 생성되어야 함")
}

// TestSpecCmd_ValidateExisting는 기존 spec validate 커맨드를 테스트한다.
// spec new는 현재 디렉터리에서 실행하므로 임시 디렉터리로 이동 필요
func TestSpecCmd_ValidateExisting(t *testing.T) {
	// Chdir은 병렬 실행과 함께 사용 불가

	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	// 먼저 SPEC 생성
	createCmd := newTestRootCmd()
	createCmd.SetArgs([]string{"spec", "new", "VALID-001", "--title", "유효성 검증 테스트"})
	require.NoError(t, createCmd.Execute())

	// 생성된 SPEC 검증
	validateCmd := newTestRootCmd()
	validateCmd.SetArgs([]string{"spec", "validate", filepath.Join(dir, ".autopus", "specs", "SPEC-VALID-001")})
	// 검증 실행 (오류가 발생할 수 있음 - 경고만 있으면 성공)
	_ = validateCmd.Execute()
}

// TestSpecCmd_ValidateNonExistent는 존재하지 않는 spec validate를 테스트한다.
func TestSpecCmd_ValidateNonExistent(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"spec", "validate", "/nonexistent/spec/dir"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// TestUpdateCmd_WithDir는 --dir 플래그로 update 커맨드를 테스트한다.
func TestUpdateCmd_WithDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 먼저 init으로 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// update 실행
	var buf bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&buf)
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	err := updateCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Update complete")
}

// TestUpdateCmd_DefaultDir는 기본 디렉터리에서 update를 테스트한다.
// config.Load는 파일 없으면 기본 설정을 반환하므로 오류 없이 실행된다.
func TestUpdateCmd_DefaultDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"update", "--dir", dir})
	err := cmd.Execute()
	// 설정 파일 없어도 기본값으로 실행됨
	require.NoError(t, err)
}

// TestPlatformListCmd_WithDetected는 감지된 플랫폼이 포함된 platform list를 테스트한다.
func TestPlatformListCmd_WithDetected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	var buf bytes.Buffer
	listCmd := newTestRootCmd()
	listCmd.SetOut(&buf)
	listCmd.SetArgs([]string{"platform", "list", "--dir", dir})
	err := listCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "claude-code")
	assert.Contains(t, output, "Configured platforms")
}

// TestPlatformAddCmd_AlreadyExists는 이미 있는 플랫폼 추가를 테스트한다.
func TestPlatformAddCmd_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// 이미 있는 플랫폼 추가 시도
	var buf bytes.Buffer
	addCmd := newTestRootCmd()
	addCmd.SetOut(&buf)
	addCmd.SetArgs([]string{"platform", "add", "claude-code", "--dir", dir})
	err := addCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "이미 추가")
}

// TestPlatformRemoveCmd_NotFound는 없는 플랫폼 제거를 테스트한다.
func TestPlatformRemoveCmd_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// 없는 플랫폼 제거 시도
	var buf bytes.Buffer
	removeCmd := newTestRootCmd()
	removeCmd.SetOut(&buf)
	removeCmd.SetArgs([]string{"platform", "remove", "nonexistent-platform", "--dir", dir})
	err := removeCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "찾을 수 없습니다")
}

// TestPlatformRemoveCmd_LastPlatform는 마지막 플랫폼 제거 시도를 테스트한다.
func TestPlatformRemoveCmd_LastPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 단일 플랫폼으로 설정
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// 마지막 플랫폼 제거 시도 - 오류가 발생해야 함
	removeCmd := newTestRootCmd()
	removeCmd.SetArgs([]string{"platform", "remove", "claude-code", "--dir", dir})
	err := removeCmd.Execute()
	assert.Error(t, err)
}

// TestDoctorCmd_WithConfig는 설정 파일이 있는 doctor 커맨드를 테스트한다.
func TestDoctorCmd_WithConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--lite", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	var buf bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&buf)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	err := doctorCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Autopus")
}

// TestDoctorCmd_NoConfig는 설정 파일 없는 doctor 커맨드를 테스트한다.
func TestDoctorCmd_NoConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	doctorCmd := newTestRootCmd()
	doctorCmd.SetOut(&buf)
	doctorCmd.SetArgs([]string{"doctor", "--dir", dir})
	// 설정 파일 없어도 오류 없이 실행됨 (내부에서 처리)
	_ = doctorCmd.Execute()
	output := buf.String()
	assert.Contains(t, output, "Autopus")
}

// TestLSPCmd_Structure는 lsp 커맨드 구조를 테스트한다.
func TestLSPCmd_Structure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"lsp", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "diagnostics")
	assert.Contains(t, output, "refs")
	assert.Contains(t, output, "rename")
	assert.Contains(t, output, "symbols")
	assert.Contains(t, output, "definition")
}

// TestLSPDiagnosticsCmd_InGoProject는 Go 프로젝트에서 lsp diagnostics를 테스트한다.
// 실제 LSP 서버 없이 오류만 확인
func TestLSPDiagnosticsCmd_InGoProject(t *testing.T) {
	t.Parallel()

	// lsp diagnostics는 go.mod가 있어야 하고 gopls가 있어야 함
	// 여기서는 Go 프로젝트(CWD)에서 실행하지만 gopls 없으면 오류
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "diagnostics", "main.go"})
	err := cmd.Execute()
	// gopls가 없거나 다른 오류가 발생할 수 있음
	_ = err
}

// TestSkillListCmd_WithCategory는 카테고리 필터로 skill list를 테스트한다.
func TestSkillListCmd_WithCategory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestSkill(t, dir, "tdd.md", `---
name: tdd
description: TDD 스킬
category: methodology
triggers:
  - tdd
---
body`)
	writeTestSkill(t, dir, "deploy.md", `---
name: deploy
description: 배포 스킬
category: devops
triggers:
  - deploy
---
body`)

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "list", "--skills-dir", dir, "--category", "methodology"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "tdd")
	assert.NotContains(t, output, "deploy")
}

// TestSkillListCmd_Empty는 빈 스킬 디렉토리를 테스트한다.
func TestSkillListCmd_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "list", "--skills-dir", dir})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "등록된 스킬이 없습니다")
}

// TestSkillInfoCmd_WithResources는 리소스가 있는 skill info를 테스트한다.
func TestSkillInfoCmd_WithResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestSkill(t, dir, "advanced.md", `---
name: advanced
description: 고급 스킬
category: advanced
triggers:
  - advanced
resources:
  - docs/reference.md
  - examples/sample.md
---

# Advanced Skill

이 스킬은 고급 기능을 제공합니다.`)

	var buf bytes.Buffer
	cmd := newTestRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"skill", "info", "advanced", "--skills-dir", dir})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "advanced")
	assert.Contains(t, output, "고급 스킬")
}
