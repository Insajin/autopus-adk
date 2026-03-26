package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

// applyHooksAndPermissions는 hooks와 permissions를 .claude/settings.json에 설치한다.
// Always writes settings.json — DetectPermissions always returns non-nil with common defaults.
func (a *Adapter) applyHooksAndPermissions(ctx context.Context, cfg *config.HarnessConfig) error {
	hookConfigs, gitHooks, _ := content.GenerateHookConfigs(cfg.Hooks, "claude-code", a.SupportsHooks())
	perms := content.DetectPermissions(a.root, cfg.Hooks.Permissions)
	if err := a.InstallHooks(ctx, hookConfigs, perms); err != nil {
		return fmt.Errorf("hooks/permissions 설치 실패: %w", err)
	}
	// Write git hooks as fallback when CLI hooks not supported
	for _, gh := range gitHooks {
		ghPath := filepath.Join(a.root, gh.Path)
		if err := os.MkdirAll(filepath.Dir(ghPath), 0755); err != nil {
			return fmt.Errorf("git hook 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(ghPath, []byte(gh.Content), 0755); err != nil {
			return fmt.Errorf("git hook 쓰기 실패: %w", err)
		}
	}
	return nil
}

// InstallHooks는 .claude/settings.json에 훅과 권한을 Claude Code 중첩 스키마로 설치한다.
func (a *Adapter) InstallHooks(_ context.Context, hooks []adapter.HookConfig, perms *adapter.PermissionSet) error {
	settingsDir := filepath.Join(a.root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("설정 디렉터리 생성 실패: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Load existing settings.json or create new
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]interface{})
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Build hooks in Claude Code nested schema, merging with existing user hooks.
	// Autopus-managed event keys are replaced entirely to prevent duplication;
	// other event keys set by the user are preserved.
	if len(hooks) > 0 {
		existingHooks, _ := settings["hooks"].(map[string]any)
		hooksMap := make(map[string]any)

		// Collect which event keys autopus manages
		managedEvents := make(map[string]bool)
		for _, h := range hooks {
			managedEvents[h.Event] = true
		}

		// Preserve user-defined event keys that autopus does not manage
		for k, v := range existingHooks {
			if !managedEvents[k] {
				hooksMap[k] = v
			}
		}

		// Set autopus-managed events fresh (no append to existing)
		for _, h := range hooks {
			entry := map[string]any{
				"matcher": h.Matcher,
				"hooks": []map[string]any{
					{
						"type":    h.Type,
						"command": h.Command,
						"timeout": h.Timeout,
					},
				},
			}
			entries, _ := hooksMap[h.Event].([]any)
			entries = append(entries, entry)
			hooksMap[h.Event] = entries
		}
		settings["hooks"] = hooksMap
	}

	// Merge permissions: append autopus defaults to existing user permissions.
	if perms != nil && (len(perms.Allow) > 0 || len(perms.Deny) > 0) {
		existingPerms, _ := settings["permissions"].(map[string]any)
		permMap := make(map[string]any)
		for k, v := range existingPerms {
			permMap[k] = v
		}
		if len(perms.Allow) > 0 {
			existing := toStringSlice(permMap["allow"])
			permMap["allow"] = mergeUnique(existing, perms.Allow)
		}
		if len(perms.Deny) > 0 {
			existing := toStringSlice(permMap["deny"])
			permMap["deny"] = mergeUnique(existing, perms.Deny)
		}
		settings["permissions"] = permMap
	}

	// Statusline configuration — always set to autopus statusline
	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": ".claude/statusline.sh",
		"padding": 1,
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("settings.json 직렬화 실패: %w", err)
	}
	return os.WriteFile(settingsPath, append(out, '\n'), 0644)
}

// toStringSlice converts an any (typically []any from JSON) to []string.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// mergeUnique appends items from add to base, skipping duplicates.
func mergeUnique(base, add []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	result := append([]string{}, base...)
	for _, s := range add {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}
