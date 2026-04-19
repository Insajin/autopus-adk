package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlatformListCmd_WithDetected는 감지된 플랫폼이 포함된 platform list를 테스트한다.
func TestPlatformListCmd_WithDetected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
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
