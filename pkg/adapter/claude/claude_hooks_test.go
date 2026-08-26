// Package claude_test는 Claude 어댑터 훅 관련 추가 테스트이다.
package claude_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	data, readErr := os.ReadFile(settingsPath)
	require.NoError(t, readErr)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))

	statusLine, ok := settings["statusLine"].(map[string]interface{})
	require.True(t, ok, "statusLine 필드가 있어야 함")
	assert.Equal(t, ".claude/statusline.sh", statusLine["command"])
}

// TestClaudeAdapter_InstallHooks_WithHooks는 훅 설정을 포함한 InstallHooks를 테스트한다.
// New schema: hooks are nested by event name.
func TestClaudeAdapter_InstallHooks_WithHooks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := claude.NewWithRoot(dir)

	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
		{Event: "PostToolUse", Matcher: "Bash", Type: "command", Command: "auto react check --quiet", Timeout: 60},
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

func TestClaudeAdapter_InstallHooks_PreservesExistingStatusLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))

	existing := map[string]interface{}{
		"statusLine": map[string]interface{}{
			"type":    "command",
			"command": "node ~/.claude/hud/omc-hud.mjs",
			"padding": 2,
		},
	}
	data, err := json.Marshal(existing)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644))

	a := claude.NewWithRoot(dir)
	err = a.InstallHooks(context.Background(), []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
	}, nil)
	require.NoError(t, err)

	updated, readErr := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, readErr)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(updated, &result))

	statusLine, ok := result["statusLine"].(map[string]interface{})
	require.True(t, ok, "기존 statusLine이 유지되어야 함")
	assert.Equal(t, "node ~/.claude/hud/omc-hud.mjs", statusLine["command"])
	assert.EqualValues(t, 2, statusLine["padding"])
}

// TestClaudeAdapter_InstallHooks_InvalidJSON은 잘못된 JSON settings.json이 있을 때를 테스트한다.
func TestClaudeAdapter_InstallHooks_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// 잘못된 JSON 파일 생성
	settingsDir := filepath.Join(dir, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")
	original := []byte("{invalid json}")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	require.NoError(t, os.WriteFile(settingsPath, original, 0644))

	a := claude.NewWithRoot(dir)
	hooks := []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
	}

	// Malformed user settings fail closed instead of being replaced.
	err := a.InstallHooks(context.Background(), hooks, nil)
	assert.Error(t, err)
	data, readErr := os.ReadFile(settingsPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, data)
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

// TestClaudeAdapter_InstallHooks_ConditionalDispatcherStaysSingle is the S10
// oracle at the settings layer (REQ-CONDRULE-COMPILE-03). prepareSettingsMapping
// writes the deduplicated dispatcher as one PreToolUse entry holding one hook
// object, leaves the autopus entries that use other matchers and events intact,
// and preserves the user event keys it does not manage.
func TestClaudeAdapter_InstallHooks_ConditionalDispatcherStaysSingle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))

	// A user-managed event key plus a stale autopus PreToolUse entry from an
	// earlier install: the first survives, the second must not be duplicated.
	seeded := `{"hooks":{"Notification":[{"matcher":"","hooks":[{"type":"command","command":"user-notify"}]}],` +
		`"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"auto rules fire --event PreToolUse"}]}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(seeded), 0o644))

	err := claude.NewWithRoot(dir).InstallHooks(context.Background(), []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto check --arch --quiet", Timeout: 30},
		{Event: "PreToolUse", Matcher: "Bash", Type: "command", Command: "auto rules fire --event PreToolUse", Timeout: 10},
		{Event: "PostToolUse", Matcher: "Bash", Type: "command", Command: "auto react check --quiet", Timeout: 60},
	}, nil)
	require.NoError(t, err)

	raw, readErr := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	require.NoError(t, readErr)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(raw, &settings))

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "settings.json must hold a hooks object")
	preToolUse, ok := hooks["PreToolUse"].([]any)
	require.True(t, ok)
	require.Len(t, preToolUse, 2, "the arch check keeps its own entry beside the dispatcher")

	var dispatchers []map[string]any
	for _, item := range preToolUse {
		entry, entryOK := item.(map[string]any)
		require.True(t, entryOK)
		objects, objectsOK := entry["hooks"].([]any)
		require.True(t, objectsOK, "a hook entry nests its hook objects")
		for _, object := range objects {
			hook, hookOK := object.(map[string]any)
			require.True(t, hookOK)
			if command, _ := hook["command"].(string); strings.Contains(command, "auto rules fire") {
				dispatchers = append(dispatchers, entry)
			}
		}
	}

	require.Len(t, dispatchers, 1, "regeneration must not duplicate the dispatcher entry")
	assert.Equal(t, "Bash", dispatchers[0]["matcher"])
	assert.Len(t, dispatchers[0]["hooks"], 1, "the dispatcher entry holds exactly one hook object")
	assert.NotEmpty(t, hooks["PostToolUse"], "autopus entries for other events survive")
	assert.NotEmpty(t, hooks["Notification"], "unmanaged user event keys survive")
}
