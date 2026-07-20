// Package gemini provides settings.json management for Antigravity CLI.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

func (a *Adapter) generateSettingsWithHooks(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	files, err := a.generateSettings(cfg)
	if err != nil || len(files) == 0 {
		return files, err
	}

	settings := make(map[string]any)
	if err := json.Unmarshal(files[0].Content, &settings); err != nil {
		return nil, fmt.Errorf("gemini settings JSON 파싱 실패: %w", err)
	}
	applyGeminiHooksAndPermissions(settings, a.configuredHooks(cfg), content.DetectPermissions(a.root, cfg.Hooks.Permissions))
	return buildGeminiSettingsMapping(settings)
}

func (a *Adapter) installAntigravityPluginIfAvailable(ctx context.Context) {
	if a.skipPluginInstall {
		return
	}
	if _, lookErr := exec.LookPath(cliBinary); lookErr == nil {
		pluginPath := filepath.Join(a.root, antigravityPluginDir)
		cmd := exec.CommandContext(ctx, cliBinary, "plugin", "install", pluginPath)
		_ = cmd.Run()
	}
}

// InstallHooks merges hooks and permissions into .gemini/settings.json.
func (a *Adapter) InstallHooks(_ context.Context, hooks []adapter.HookConfig, perms *adapter.PermissionSet) error {
	settingsDir := filepath.Join(a.root, ".gemini")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("gemini 설정 디렉터리 생성 실패: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings := readGeminiSettings(settingsPath)
	applyGeminiHooksAndPermissions(settings, hooks, perms)
	files, err := buildGeminiSettingsMapping(settings)
	if err != nil {
		return err
	}
	files = sanitizeUnsupportedClaudeTeamMappings(files)
	return adapter.WriteFileIfChanged(settingsPath, files[0].Content, 0644)
}

func applyGeminiHooksAndPermissions(settings map[string]any, hooks []adapter.HookConfig, perms *adapter.PermissionSet) {
	if len(hooks) > 0 {
		existingHooks, _ := settings["hooks"].(map[string]any)
		hooksMap := make(map[string]any)

		// Purge both Antigravity and legacy Gemini event names so stale entries
		// from prior installs are removed when regenerating hook settings.
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
}

func buildGeminiSettingsMapping(settings map[string]any) ([]adapter.FileMapping, error) {
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("gemini settings.json 직렬화 실패: %w", err)
	}
	content := append(out, '\n')
	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".gemini", "settings.json"),
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(content)),
		Content:         content,
	}}, nil
}

func readGeminiSettings(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]any)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return make(map[string]any)
	}
	return settings
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
