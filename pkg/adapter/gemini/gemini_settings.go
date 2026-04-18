// Package gemini provides settings.json management for Gemini CLI.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

// generateSettings renders settings.json.tmpl and returns a file mapping.
func (a *Adapter) generateSettings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("gemini/settings/settings.json.tmpl")
	if err != nil {
		return nil, fmt.Errorf("gemini settings 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("gemini settings 템플릿 렌더링 실패: %w", err)
	}

	// Parse rendered JSON and merge with existing settings
	var newSettings map[string]any
	if err := json.Unmarshal([]byte(rendered), &newSettings); err != nil {
		return nil, fmt.Errorf("gemini settings JSON 파싱 실패: %w", err)
	}

	settingsPath := filepath.Join(a.root, ".gemini", "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var existing map[string]any
		if err := json.Unmarshal(data, &existing); err == nil {
			newSettings = mergeSettingsMaps(existing, newSettings)
		}
	}

	out, err := json.MarshalIndent(newSettings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gemini settings JSON 직렬화 실패: %w", err)
	}
	outStr := string(out) + "\n"

	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".gemini", "settings.json"),
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(outStr),
		Content:         []byte(outStr),
	}}, nil
}

// applyHooksAndPermissions installs hooks and permissions to .gemini/settings.json.
func (a *Adapter) applyHooksAndPermissions(ctx context.Context, cfg *config.HarnessConfig) error {
	hookConfigs, _, _ := content.GenerateHookConfigs(cfg.Hooks, "gemini-cli", a.SupportsHooks())
	perms := content.DetectPermissions(a.root, cfg.Hooks.Permissions)
	return a.InstallHooks(ctx, hookConfigs, perms)
}

// InstallHooks merges hooks and permissions into .gemini/settings.json.
func (a *Adapter) InstallHooks(_ context.Context, hooks []adapter.HookConfig, perms *adapter.PermissionSet) error {
	settingsDir := filepath.Join(a.root, ".gemini")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("gemini 설정 디렉터리 생성 실패: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]any)
		}
	} else {
		settings = make(map[string]any)
	}

	if len(hooks) > 0 {
		existingHooks, _ := settings["hooks"].(map[string]any)
		hooksMap := make(map[string]any)

		// Purge both the new (gemini-native) and legacy (Claude Code) event
		// names so stale PreToolUse/PostToolUse entries from prior installs
		// are removed when regenerating with gemini-translated names.
		managedEvents := map[string]bool{
			"PreToolUse":  true,
			"PostToolUse": true,
			"BeforeTool":  true,
			"AfterTool":   true,
		}
		for _, h := range hooks {
			managedEvents[h.Event] = true
		}

		for k, v := range existingHooks {
			if !managedEvents[k] {
				hooksMap[k] = v
			}
		}

		for _, h := range hooks {
			entry := map[string]any{
				"matcher": h.Matcher,
				"hooks": []map[string]any{{
					"type":    h.Type,
					"command": h.Command,
					"timeout": h.Timeout,
				}},
			}
			entries, _ := hooksMap[h.Event].([]any)
			hooksMap[h.Event] = append(entries, entry)
		}
		settings["hooks"] = hooksMap
	}

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

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("gemini settings.json 직렬화 실패: %w", err)
	}
	return adapter.WriteFileIfChanged(settingsPath, append(out, '\n'), 0644)
}

// mergeSettingsMaps merges new settings into existing, preserving user keys.
func mergeSettingsMaps(existing, newSettings map[string]any) map[string]any {
	for k, v := range newSettings {
		if existingSub, ok := existing[k].(map[string]any); ok {
			if newSub, ok := v.(map[string]any); ok {
				existing[k] = mergeSettingsMaps(existingSub, newSub)
				continue
			}
		}
		existing[k] = v
	}
	return existing
}

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
