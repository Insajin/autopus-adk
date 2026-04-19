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
	t.Setenv("EXA_API_KEY", "")

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
