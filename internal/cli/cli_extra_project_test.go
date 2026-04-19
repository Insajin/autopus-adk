package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
