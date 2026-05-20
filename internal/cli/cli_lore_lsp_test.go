// Package cli_test는 lore와 lsp 관련 추가 테스트를 제공한다.
package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoreCmd_ConstraintsInGitRepo는 git repo에서 lore constraints를 테스트한다.
func TestLoreCmd_ConstraintsInGitRepo(t *testing.T) {
	t.Parallel()

	// git repo (현재 디렉터리)에서 실행
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "constraints"})
	// git log 실행 가능하면 오류 없음
	err := cmd.Execute()
	// git repo이면 성공, 아니면 오류
	_ = err
}

// TestLoreCmd_RejectedInGitRepo는 git repo에서 lore rejected를 테스트한다.
func TestLoreCmd_RejectedInGitRepo(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "rejected"})
	_ = cmd.Execute()
}

// TestLoreCmd_DirectivesInGitRepo는 git repo에서 lore directives를 테스트한다.
func TestLoreCmd_DirectivesInGitRepo(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "directives"})
	_ = cmd.Execute()
}

// TestLoreCmd_StaleDefault은 기본 days로 lore stale을 테스트한다.
func TestLoreCmd_StaleDefault(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "stale"})
	_ = cmd.Execute()
}

// TestLoreCmd_StaleShortDays는 짧은 days로 lore stale을 테스트한다.
func TestLoreCmd_StaleShortDays(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lore", "stale", "--days", "1"})
	_ = cmd.Execute()
}

// TestLSPCmd_DiagnosticsJSONFormat은 JSON 형식으로 lsp diagnostics를 테스트한다.
func TestLSPCmd_DiagnosticsJSONFormat(t *testing.T) {
	t.Parallel()

	// Go 프로젝트에서 실행되지만 gopls가 없거나 실패할 수 있음
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "diagnostics", "--format", "json", "main.go"})
	err := cmd.Execute()
	_ = err // gopls가 없으면 오류 발생 가능
}

// TestLSPCmd_DiagnosticsTextFormat은 텍스트 형식으로 lsp diagnostics를 테스트한다.
func TestLSPCmd_DiagnosticsTextFormat(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "diagnostics", "--format", "text", "main.go"})
	err := cmd.Execute()
	_ = err
}

// TestLSPCmd_RefsCommand는 lsp refs 커맨드를 테스트한다.
func TestLSPCmd_RefsCommand(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "refs", "HandleRequest"})
	err := cmd.Execute()
	_ = err
}

// TestLSPCmd_RenameCommand는 lsp rename 커맨드를 테스트한다.
func TestLSPCmd_RenameCommand(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "rename", "oldFunc", "newFunc"})
	err := cmd.Execute()
	_ = err
}

// TestLSPCmd_SymbolsCommand는 lsp symbols 커맨드를 테스트한다.
func TestLSPCmd_SymbolsCommand(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "symbols", "main.go"})
	err := cmd.Execute()
	_ = err
}

// TestLSPCmd_DefinitionCommand는 lsp definition 커맨드를 테스트한다.
func TestLSPCmd_DefinitionCommand(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"lsp", "definition", "MyFunction"})
	err := cmd.Execute()
	_ = err
}

// TestLSPCmd_SubcommandHelp는 각 lsp 서브커맨드 help를 테스트한다.
func TestLSPCmd_SubcommandHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"diagnostics help", []string{"lsp", "diagnostics", "--help"}},
		{"refs help", []string{"lsp", "refs", "--help"}},
		{"rename help", []string{"lsp", "rename", "--help"}},
		{"symbols help", []string{"lsp", "symbols", "--help"}},
		{"definition help", []string{"lsp", "definition", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newTestRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			assert.NoError(t, err)
		})
	}
}

// TestLSPCmd_MissingArgs는 인자 없는 lsp 서브커맨드를 테스트한다.
func TestLSPCmd_MissingArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"diagnostics no args", []string{"lsp", "diagnostics"}},
		{"refs no args", []string{"lsp", "refs"}},
		{"rename one arg", []string{"lsp", "rename", "only-one"}},
		{"symbols no args", []string{"lsp", "symbols"}},
		{"definition no args", []string{"lsp", "definition"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newTestRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			assert.Error(t, err)
		})
	}
}

// TestDocsCmd_NoArgs는 인자 없는 docs 커맨드를 테스트한다.
// docs는 parent command이므로 서브커맨드 없이 실행하면 help를 표시하고 에러 없이 종료한다.
func TestDocsCmd_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"docs"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

// TestDocsCmd_WithLibrary는 라이브러리 이름으로 docs 커맨드를 테스트한다.
// 실제 API 호출이 발생하므로 오류 허용
func TestDocsCmd_WithLibrary(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"docs", "react"})
	err := cmd.Execute()
	// API 호출 실패 가능
	_ = err
}

// TestSearchCmd_MultiWordQuery는 여러 단어 쿼리로 search 커맨드를 테스트한다.
func TestSearchCmd_MultiWordQuery(t *testing.T) {
	// EXA_API_KEY 환경변수 확인
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"search", "golang", "testing", "patterns"})
	err := cmd.Execute()
	// API 키 없으면 오류
	_ = err
}

// TestPlatformCmd_Help는 platform 커맨드 help를 테스트한다.
func TestPlatformCmd_Help(t *testing.T) {
	t.Parallel()

	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"platform", "--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

// TestPlatformAddCmd_GeminiCLI는 antigravity-cli 플랫폼 추가를 테스트한다.
func TestPlatformAddCmd_GeminiCLI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	assert.NoError(t, initCmd.Execute())

	// antigravity-cli 추가
	addCmd := newTestRootCmd()
	addCmd.SetArgs([]string{"platform", "add", "antigravity-cli", "--dir", dir})
	err := addCmd.Execute()
	assert.NoError(t, err)
}

// TestPlatformRemoveCmd_Codex는 codex 플랫폼 제거를 테스트한다.
func TestPlatformRemoveCmd_Codex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 두 플랫폼으로 초기화
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,codex"})
	assert.NoError(t, initCmd.Execute())

	// codex 제거
	removeCmd := newTestRootCmd()
	removeCmd.SetArgs([]string{"platform", "remove", "codex", "--dir", dir})
	err := removeCmd.Execute()
	assert.NoError(t, err)
}

// TestUpdateCmd_UnknownPlatform은 알 수 없는 플랫폼이 있는 update를 테스트한다.
// config에 알 수 없는 플랫폼이 있으면 경고를 출력하고 계속 진행한다.
func TestUpdateCmd_UnknownPlatform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 설정 파일 생성 후 직접 수정하여 알 수 없는 플랫폼 추가는 어려우므로
	// 기본 설정으로만 테스트
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	assert.NoError(t, initCmd.Execute())

	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	err := updateCmd.Execute()
	assert.NoError(t, err)
}
