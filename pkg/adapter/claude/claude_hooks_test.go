// Package claude_test는 Claude 어댑터 훅 관련 추가 테스트이다.
package claude_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
)

// TestClaudeAdapter_InstallHooks_Empty는 훅이 없는 경우 InstallHooks를 테스트한다.
func TestClaudeAdapter_InstallHooks_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)

	err := a.InstallHooks(context.Background(), nil, nil)
	assert.NoError(t, err)

	// settings.json이 생성되어야 함
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	_, statErr := os.Stat(settingsPath)
	assert.NoError(t, statErr)
}

// TestClaudeAdapter_InstallHooks_WithHooks는 훅 설정을 포함한 InstallHooks를 테스트한다.
// New schema: hooks are nested by event name.
func TestClaudeAdapter_InstallHooks_WithHooks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)

	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
		{Event: "PostToolUse", Matcher: "Bash", Type: "command", Command: "auto react --ci-failure --quiet", Timeout: 60},
	}

	err := a.InstallHooks(context.Background(), hooks, nil)
	require.NoError(t, err)

	// settings.json 내용 확인
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, readErr := os.ReadFile(settingsPath)
	require.NoError(t, readErr)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))

	// hooks 필드가 맵(nested schema)으로 있어야 함
	hooksVal, ok := settings["hooks"]
	assert.True(t, ok, "hooks 필드가 있어야 함")
	hooksMap, ok := hooksVal.(map[string]interface{})
	assert.True(t, ok, "hooks는 event별 맵이어야 함")
	// PreToolUse 이벤트 항목 확인
	preToolUse, ok := hooksMap["PreToolUse"]
	assert.True(t, ok, "PreToolUse 이벤트가 있어야 함")
	entries, ok := preToolUse.([]interface{})
	assert.True(t, ok)
	assert.Len(t, entries, 1)
}

// TestClaudeAdapter_InstallHooks_WithPermissions는 권한 설정을 포함한 InstallHooks를 테스트한다.
func TestClaudeAdapter_InstallHooks_WithPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)

	perms := &adapter.PermissionSet{
		Allow: []string{"Bash(go test:*)", "Bash(git *)", "WebSearch"},
		Deny:  []string{"Bash(rm -rf:*)"},
	}

	err := a.InstallHooks(context.Background(), nil, perms)
	require.NoError(t, err)

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	data, readErr := os.ReadFile(settingsPath)
	require.NoError(t, readErr)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))

	permVal, ok := settings["permissions"]
	assert.True(t, ok, "permissions 필드가 있어야 함")
	permMap, ok := permVal.(map[string]interface{})
	assert.True(t, ok)
	allowList, ok := permMap["allow"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, allowList, 3)
	denyList, ok := permMap["deny"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, denyList, 1)
}

// TestClaudeAdapter_InstallHooks_MergesExisting는 기존 settings.json과 병합을 테스트한다.
func TestClaudeAdapter_InstallHooks_MergesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 기존 settings.json 생성
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	existing := map[string]interface{}{
		"theme": "dark",
	}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644))

	a := claude.NewWithRoot(dir)
	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
	}

	err := a.InstallHooks(context.Background(), hooks, nil)
	require.NoError(t, err)

	// 결과 확인: 기존 필드와 새 hooks 모두 있어야 함
	updated, readErr := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, readErr)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(updated, &result))

	// hooks 추가됨, 기존 theme 보존됨
	_, hasHooks := result["hooks"]
	assert.True(t, hasHooks)
	theme, _ := result["theme"].(string)
	assert.Equal(t, "dark", theme)
}

// TestClaudeAdapter_InstallHooks_InvalidJSON은 잘못된 JSON settings.json이 있을 때를 테스트한다.
func TestClaudeAdapter_InstallHooks_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 잘못된 JSON 파일 생성
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{invalid json}"), 0644))

	a := claude.NewWithRoot(dir)
	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
	}

	// 잘못된 JSON이어도 오류 없이 처리되어야 함 (기본 맵으로 초기화)
	err := a.InstallHooks(context.Background(), hooks, nil)
	assert.NoError(t, err)
}

// TestClaudeAdapter_Clean_RemovesMarker는 Clean이 CLAUDE.md 마커 섹션을 제거하는지 테스트한다.
func TestClaudeAdapter_Clean_RemovesMarker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// CLAUDE.md에 마커 섹션 포함 콘텐츠 생성
	claudePath := filepath.Join(dir, "CLAUDE.md")
	content := "# 내 프로젝트\n\n<!-- AUTOPUS:BEGIN -->\n자동 생성 섹션\n<!-- AUTOPUS:END -->\n\n## 사용자 섹션\n"
	require.NoError(t, os.WriteFile(claudePath, []byte(content), 0644))

	a := claude.NewWithRoot(dir)
	err := a.Clean(context.Background())
	require.NoError(t, err)

	// 마커 섹션이 제거되고 사용자 섹션은 보존되어야 함
	data, readErr := os.ReadFile(claudePath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "AUTOPUS:BEGIN")
	assert.Contains(t, string(data), "사용자 섹션")
}
