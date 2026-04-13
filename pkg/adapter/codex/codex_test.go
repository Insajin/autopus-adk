// Package codex는 Codex 어댑터 테스트이다.
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

func TestCodexAdapter_Name(t *testing.T) {
	t.Parallel()
	a := codex.New()
	assert.Equal(t, "codex", a.Name())
}

func TestCodexAdapter_CLIBinary(t *testing.T) {
	t.Parallel()
	a := codex.New()
	assert.Equal(t, "codex", a.CLIBinary())
}

func TestCodexAdapter_SupportsHooks(t *testing.T) {
	t.Parallel()
	a := codex.New()
	assert.True(t, a.SupportsHooks(), "Codex는 hooks.json을 통해 훅을 지원")
}

func TestCodexAdapter_Detect_NotInstalled(t *testing.T) {
	// t.Setenv는 t.Parallel()과 함께 사용할 수 없음
	t.Setenv("PATH", t.TempDir())
	a := codex.New()
	ok, err := a.Detect(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCodexAdapter_Generate_CreatesAgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	files, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, files)

	// AGENTS.md 생성 확인
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "<!-- AUTOPUS:BEGIN -->")
	assert.Contains(t, content, "<!-- AUTOPUS:END -->")
}

func TestCodexAdapter_Generate_CreatesSkillsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// .codex/skills/auto-* 디렉터리 확인
	skillsDir := filepath.Join(dir, ".codex", "skills")
	info, statErr := os.Stat(skillsDir)
	require.NoError(t, statErr, ".codex/skills 디렉터리가 존재해야 함")
	assert.True(t, info.IsDir())

	repoSkill := filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md")
	_, statErr = os.Stat(repoSkill)
	require.NoError(t, statErr, ".agents/skills/auto/SKILL.md가 존재해야 함")

	marketplace := filepath.Join(dir, ".agents", "plugins", "marketplace.json")
	_, statErr = os.Stat(marketplace)
	require.NoError(t, statErr, ".agents/plugins/marketplace.json이 존재해야 함")

	pluginManifest := filepath.Join(dir, ".autopus", "plugins", "auto", ".codex-plugin", "plugin.json")
	_, statErr = os.Stat(pluginManifest)
	require.NoError(t, statErr, "로컬 codex plugin manifest가 존재해야 함")
}

func TestCodexAdapter_Generate_PreservesUserContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	// 기존 AGENTS.md 생성
	userContent := "# My Agent Rules\n\nCustom agent rules here.\n"
	err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(userContent), 0644)
	require.NoError(t, err)

	_, err = a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	content := string(data)

	// 사용자 컨텐츠 보존 확인
	assert.Contains(t, content, "My Agent Rules")
	assert.Contains(t, content, "Custom agent rules here.")
	assert.Contains(t, content, "<!-- AUTOPUS:BEGIN -->")
}

func TestCodexAdapter_Update(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	files, err := a.Update(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, files)
}

func TestCodexAdapter_InstallHooks_NoOp(t *testing.T) {
	t.Parallel()
	a := codex.New()
	// SupportsHooks()가 false이므로 InstallHooks는 no-op이어야 함
	err := a.InstallHooks(context.Background(), nil, nil)
	require.NoError(t, err)
}

func TestCodexAdapter_Validate_AfterGenerate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	errs, err := a.Validate(context.Background())
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestCodexAdapter_Clean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := codex.NewWithRoot(dir)
	cfg := config.DefaultFullConfig("test-project")

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	err = a.Clean(context.Background())
	require.NoError(t, err)

	// .codex/skills 디렉터리가 제거되어야 함
	_, statErr := os.Stat(filepath.Join(dir, ".codex", "skills"))
	assert.True(t, os.IsNotExist(statErr))

	_, statErr = os.Stat(filepath.Join(dir, ".agents", "skills", "auto"))
	assert.True(t, os.IsNotExist(statErr))
}
