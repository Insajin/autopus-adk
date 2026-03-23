// Package gemini는 Gemini CLI 어댑터 테스트이다.
package gemini_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestGeminiAdapter_Name(t *testing.T) {
	t.Parallel()
	a := gemini.New()
	assert.Equal(t, "gemini-cli", a.Name())
}

func TestGeminiAdapter_CLIBinary(t *testing.T) {
	t.Parallel()
	a := gemini.New()
	assert.Equal(t, "gemini", a.CLIBinary())
}

func TestGeminiAdapter_SupportsHooks(t *testing.T) {
	t.Parallel()
	a := gemini.New()
	assert.False(t, a.SupportsHooks())
}

func TestGeminiAdapter_Detect_NotInstalled(t *testing.T) {
	// t.Setenv는 t.Parallel()과 함께 사용할 수 없음
	t.Setenv("PATH", t.TempDir())
	a := gemini.New()
	ok, err := a.Detect(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGeminiAdapter_Generate_CreatesGeminiMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	files, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, files)

	// GEMINI.md 생성 확인
	data, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "<!-- AUTOPUS:BEGIN -->")
	assert.Contains(t, content, "<!-- AUTOPUS:END -->")
}

func TestGeminiAdapter_Generate_CreatesSkillsWithFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// .gemini/skills/autopus 디렉터리 존재 확인
	geminiSkillsDir := filepath.Join(dir, ".gemini", "skills", "autopus")
	info, statErr := os.Stat(geminiSkillsDir)
	require.NoError(t, statErr, ".gemini/skills/autopus 디렉터리가 존재해야 함")
	assert.True(t, info.IsDir())

	// 각 스킬 서브디렉터리의 SKILL.md 확인 (auto-plan, auto-go, auto-fix, auto-sync, auto-review)
	skills := []string{"auto-plan", "auto-go", "auto-fix", "auto-sync", "auto-review"}
	for _, skill := range skills {
		skillPath := filepath.Join(dir, ".gemini", "skills", "autopus", skill, "SKILL.md")
		data, readErr := os.ReadFile(skillPath)
		require.NoError(t, readErr, "SKILL.md가 존재해야 함: %s", skill)
		content := string(data)
		// YAML frontmatter 확인
		assert.Contains(t, content, "---", "YAML frontmatter가 있어야 함: %s", skill)
		assert.Contains(t, content, "name: "+skill, "스킬명이 포함되어야 함: %s", skill)
	}
}

func TestGeminiAdapter_Generate_CreatesAgentsAliases(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// .agents/skills/ 크로스플랫폼 앨리어스 디렉터리 확인
	agentsSkillsDir := filepath.Join(dir, ".agents", "skills")
	info, statErr := os.Stat(agentsSkillsDir)
	require.NoError(t, statErr, ".agents/skills 디렉터리가 존재해야 함")
	assert.True(t, info.IsDir())
}

func TestGeminiAdapter_Generate_PreservesUserContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	// 기존 GEMINI.md 생성
	userContent := "# My Gemini Rules\n\nCustom rules.\n"
	err := os.WriteFile(filepath.Join(dir, "GEMINI.md"), []byte(userContent), 0644)
	require.NoError(t, err)

	_, err = a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "My Gemini Rules")
	assert.Contains(t, content, "Custom rules.")
	assert.Contains(t, content, "<!-- AUTOPUS:BEGIN -->")
}

func TestGeminiAdapter_Update(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	files, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, files)
}

func TestGeminiAdapter_InstallHooks_NoOp(t *testing.T) {
	t.Parallel()
	a := gemini.New()
	err := a.InstallHooks(context.Background(), nil, nil)
	require.NoError(t, err)
}

func TestGeminiAdapter_Validate_AfterGenerate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestGeminiAdapter_Clean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := gemini.NewWithRoot(dir)
	cfg := config.DefaultLiteConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	err = a.Clean(context.Background())
	require.NoError(t, err)

	// .gemini/skills 디렉터리가 제거되어야 함
	_, statErr := os.Stat(filepath.Join(dir, ".gemini", "skills"))
	assert.True(t, os.IsNotExist(statErr))
}
