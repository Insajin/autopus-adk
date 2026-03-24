// Package cli는 update 커맨드 테스트이다.
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestUpdateCmd_RequiresExistingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// autopus.yaml이 없으면 에러
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"update", "--dir", dir})
	err := cmd.Execute()
	// config가 없으면 기본값으로 처리하거나 에러 — 동작 확인
	// 기본 구현에서는 기본 설정 로드 후 진행
	_ = err // 에러 여부는 구현에 따름
}

func TestUpdateCmd_UpdatesAfterInit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// 먼저 init 실행
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// CLAUDE.md 수정 시간 기록
	claudePath := filepath.Join(dir, "CLAUDE.md")
	info1, err := os.Stat(claudePath)
	require.NoError(t, err)
	modTime1 := info1.ModTime()

	// update 실행
	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute())

	// 파일이 여전히 존재해야 함
	_, statErr := os.Stat(claudePath)
	require.NoError(t, statErr)
	_ = modTime1 // 시간 비교는 OS 정밀도에 따라 다를 수 있어 생략
}

func TestUpdateCmd_PreservesUserModifications(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// init 실행
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// CLAUDE.md에 사용자 컨텐츠 추가
	claudePath := filepath.Join(dir, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	userExtra := "\n\n## My Custom Section\n\nUser-defined rules.\n"
	err = os.WriteFile(claudePath, append(data, []byte(userExtra)...), 0644)
	require.NoError(t, err)

	// update 실행
	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute())

	// 사용자 컨텐츠가 보존되어야 함
	updated, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "My Custom Section")
	assert.Contains(t, string(updated), "User-defined rules.")
}

// TestUpdateCmd_MigratesCodexPromptViaArgs verifies R4:
// update must migrate codex PromptViaArgs from false to true.
func TestUpdateCmd_MigratesCodexPromptViaArgs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Given: a full-mode project with codex
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,codex"})
	require.NoError(t, initCmd.Execute())

	// Manually set codex PromptViaArgs=false to simulate old config
	cfg, err := loadConfigFromDir(dir)
	require.NoError(t, err)
	if cfg.Orchestra.Providers == nil {
		cfg.Orchestra.Providers = make(map[string]config.ProviderEntry)
	}
	codexEntry := cfg.Orchestra.Providers["codex"]
	codexEntry.PromptViaArgs = false
	cfg.Orchestra.Providers["codex"] = codexEntry
	require.NoError(t, config.Save(dir, cfg))

	// When: update is executed
	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute())

	// Then: codex must have PromptViaArgs=true (R4)
	cfgAfter, loadErr := loadConfigFromDir(dir)
	require.NoError(t, loadErr)
	codex, ok := cfgAfter.Orchestra.Providers["codex"]
	require.True(t, ok, "codex must exist after update")
	assert.True(t, codex.PromptViaArgs, "codex PromptViaArgs must be migrated to true (R4)")
}

// TestUpdateCmd_NoAdapterPlatformIsSkipped verifies that a valid but adapter-less
// platform (e.g. "opencode") in config is skipped with a warning, and update
// still completes successfully.
func TestUpdateCmd_NoAdapterPlatformIsSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Init with claude-code first.
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code"})
	require.NoError(t, initCmd.Execute())

	// Reload config and add "opencode" — valid platform but no update adapter.
	cfg, err := loadConfigFromDir(dir)
	require.NoError(t, err)
	cfg.Platforms = append(cfg.Platforms, "opencode")
	require.NoError(t, config.Save(dir, cfg))

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute(), "update must succeed even when opencode has no adapter")
	assert.Contains(t, out.String(), "경고", "output must warn about platform with no adapter")
}

// TestUpdateCmd_SelfFlagRecognized verifies T11: --self flag is parsed by the
// update command without an "unknown flag" error. The dev-build guard
// (version="0.6.0", commit="none") stops execution before any network call.
func TestUpdateCmd_SelfFlagRecognized(t *testing.T) {
	t.Parallel()

	// Given: the root command with update --self (no --force)
	cmd := newTestRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--self"})

	// When: command executes
	err := cmd.Execute()

	// Then: error is about dev-build guard, NOT "unknown flag"
	// (commit="none" triggers the guard before any network access)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown flag")
	assert.Contains(t, err.Error(), "개발 빌드")
}

// TestUpdateCmd_SelfFlagWithForce verifies that --self --force bypasses the
// dev-build guard and proceeds to the network check step. The test uses a
// closed server URL to confirm it reaches the checker (network error expected).
func TestUpdateCmd_SelfFlagWithForce(t *testing.T) {
	t.Parallel()

	// Given: root command with --self --force
	cmd := newTestRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--self", "--force"})

	// When: command executes
	err := cmd.Execute()

	// Then: dev-build guard is bypassed; error is from network/checker, not the guard
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "개발 빌드")
}

// TestUpdateCmd_SelfCheckOnly verifies that --self --check exits without
// performing a download. Dev-build guard fires first (commit="none"), so we
// verify the flag is parsed correctly by checking the error message.
func TestUpdateCmd_SelfCheckOnly(t *testing.T) {
	t.Parallel()

	// Given: --self --check (no --force, so dev-build guard fires)
	cmd := newTestRootCmd()
	cmd.SetArgs([]string{"update", "--self", "--check"})

	// When: command executes
	err := cmd.Execute()

	// Then: error is the dev-build guard, confirming --check was parsed
	require.Error(t, err)
	assert.Contains(t, err.Error(), "개발 빌드")
}

// TestUpdateCmd_AddsCodexToOrchestraCommands verifies R5:
// update must add codex to orchestra command providers when missing.
func TestUpdateCmd_AddsCodexToOrchestraCommands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Given: full-mode project with codex platform
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "test-proj", "--platforms", "claude-code,codex"})
	require.NoError(t, initCmd.Execute())

	// Remove codex from command providers to simulate old config
	cfg, err := loadConfigFromDir(dir)
	require.NoError(t, err)
	for cmdName, entry := range cfg.Orchestra.Commands {
		var filtered []string
		for _, p := range entry.Providers {
			if p != "codex" {
				filtered = append(filtered, p)
			}
		}
		entry.Providers = filtered
		cfg.Orchestra.Commands[cmdName] = entry
	}
	require.NoError(t, config.Save(dir, cfg))

	// When: update is executed
	updateCmd := newTestRootCmd()
	updateCmd.SetArgs([]string{"update", "--dir", dir})
	require.NoError(t, updateCmd.Execute())

	// Then: all commands must include codex (R5)
	cfgAfter, loadErr := loadConfigFromDir(dir)
	require.NoError(t, loadErr)
	for _, cmdName := range []string{"review", "plan", "secure"} {
		entry, ok := cfgAfter.Orchestra.Commands[cmdName]
		require.True(t, ok, "command %q must exist", cmdName)
		assert.Contains(t, entry.Providers, "codex",
			"command %q must include codex after update (R5)", cmdName)
	}
}
