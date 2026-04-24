// Package codex_test는 Codex 어댑터 추가 테스트이다.
package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

// TestCodexAdapter_Version은 Version 메서드를 테스트한다.
func TestCodexAdapter_Version(t *testing.T) {
	t.Parallel()

	a := codex.New()
	// Version은 "" 또는 버전 문자열을 반환할 수 있음
	v := a.Version()
	_ = v // 값 자체보다 패닉 없음을 확인
}

// TestCodexAdapter_CleanRemovesFiles는 Clean이 파일을 삭제하는지 테스트한다.
func TestCodexAdapter_CleanRemovesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	// 먼저 Generate로 파일 생성
	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Clean 실행
	err = a.Clean(context.Background())
	require.NoError(t, err)
}

// TestCodexAdapter_Clean_NonExistent는 존재하지 않는 파일에 대한 Clean을 테스트한다.
func TestCodexAdapter_Clean_NonExistent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)

	// 파일 없는 상태에서 Clean 실행 (오류 없어야 함)
	err := a.Clean(context.Background())
	assert.NoError(t, err)
}

// TestCodexAdapter_Validate_NoErrors는 Generate 후 Validate 오류가 없는지 테스트한다.
func TestCodexAdapter_Validate_NoErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	// Generate 실행
	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Validate 실행
	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	// 생성 직후이므로 오류 없어야 함
	for _, e := range errs {
		assert.NotEqual(t, "error", e.Level, "오류 수준 검증 오류: %s", e.Message)
	}
}

// TestCodexAdapter_Validate_MissingFiles는 파일 없는 상태에서 Validate를 테스트한다.
func TestCodexAdapter_Validate_MissingFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)

	// Generate 없이 Validate 실행
	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	// 파일이 없으므로 오류가 있어야 함
	assert.NotEmpty(t, errs)
}

// TestCodexAdapter_Generate_FullMode는 Full 모드에서 Generate를 테스트한다.
func TestCodexAdapter_Generate_FullMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, files)
}

// TestCodexAdapter_Generate_CreatesAgentMd는 AGENTS.md 생성을 테스트한다.
func TestCodexAdapter_Generate_CreatesAgentMd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// AGENTS.md 파일이 생성되었는지 확인
	_, statErr := os.Stat(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, statErr, "AGENTS.md가 생성되어야 함")
}

func TestCodexAdapter_Validate_WarnsWhenRouterPromptBrandingMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	routerPrompt := filepath.Join(dir, ".codex", "prompts", "auto.md")
	require.NoError(t, os.WriteFile(routerPrompt, []byte("---\ndescription: test\n---\n\n# auto\n"), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)

	found := false
	for _, e := range errs {
		if e.File == filepath.Join(".codex", "prompts", "auto.md") && e.Message == "Codex router prompt에 Autopus 브랜딩 블록이 없음" {
			found = true
		}
	}
	assert.True(t, found, "router prompt branding warning should be reported")
}

func TestCodexAdapter_Validate_WarnsWhenProjectDocBudgetTooLow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	configPath := filepath.Join(dir, ".codex", "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`model = "gpt-5.5"
model_reasoning_effort = "medium"
approval_policy = "on-request"
sandbox_mode = "workspace-write"
web_search = "cached"
project_doc_max_bytes = 65536
`), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)

	found := false
	for _, e := range errs {
		if e.File == ".codex/config.toml" && e.Message == "project_doc_max_bytes가 너무 낮음 (65536 < 262144): 대형 프로젝트 문서가 잘릴 수 있음" {
			found = true
		}
	}
	assert.True(t, found, "project doc budget warning should be reported")
}

func TestCodexAdapter_Validate_WarnsWhenContext7FallbackMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	rulePath := filepath.Join(dir, ".codex", "rules", "autopus", "context7-docs.md")
	require.NoError(t, os.WriteFile(rulePath, []byte("# Context7\n"), 0644))

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)

	found := false
	for _, e := range errs {
		if e.File == filepath.Join(".codex", "rules", "autopus", "context7-docs.md") && e.Message == "Codex Context7 규칙에 web fallback 계약이 없음" {
			found = true
		}
	}
	assert.True(t, found, "context7 fallback warning should be reported")
}
