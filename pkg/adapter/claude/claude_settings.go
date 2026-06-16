package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

// applyHooksAndPermissions는 hooks와 permissions를 .claude/settings.json에 설치한다.
// Always writes settings.json — DetectPermissions always returns non-nil with common defaults.
func (a *Adapter) applyHooksAndPermissions(_ context.Context, cfg *config.HarnessConfig) error {
	files, err := a.prepareHooksAndPermissionsFiles(cfg)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := writeClaudeMapping(a.root, file); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) prepareHooksAndPermissionsFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	a.statusLineMode = resolveStatusLineMode(cfg, InspectStatusLine(a.root))
	hookConfigs, gitHooks, _ := content.GenerateProjectHookConfigs(cfg, "claude-code", a.SupportsHooks())
	perms := content.DetectPermissions(a.root, cfg.Hooks.Permissions)
	settings, err := a.prepareSettingsMapping(hookConfigs, perms)
	if err != nil {
		return nil, fmt.Errorf("hooks/permissions 준비 실패: %w", err)
	}
	files := []adapter.FileMapping{settings}
	for _, gh := range gitHooks {
		files = append(files, adapter.FileMapping{
			TargetPath:      gh.Path,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(gh.Content),
			Content:         []byte(gh.Content),
		})
	}
	return files, nil
}

// InstallHooks는 .claude/settings.json에 훅과 권한을 Claude Code 중첩 스키마로 설치한다.
func (a *Adapter) InstallHooks(_ context.Context, hooks []adapter.HookConfig, perms *adapter.PermissionSet) error {
	mapping, err := a.prepareSettingsMapping(hooks, perms)
	if err != nil {
		return err
	}
	return writeClaudeMapping(a.root, mapping)
}

func (a *Adapter) prepareSettingsMapping(hooks []adapter.HookConfig, perms *adapter.PermissionSet) (adapter.FileMapping, error) {
	var settings map[string]interface{}
	data, err := os.ReadFile(filepath.Join(a.root, ".claude", "settings.json"))
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
		managedEvents["TaskCreated"] = true

		// Preserve user-defined event keys that autopus does not manage
		for k, v := range existingHooks {
			if !managedEvents[k] {
				hooksMap[k] = v
			}
		}

		// Set autopus-managed events fresh (no append to existing)
		for _, h := range hooks {
			hookEntry := map[string]any{
				"type":    h.Type,
				"command": h.Command,
				"timeout": h.Timeout,
			}
			if len(h.Env) > 0 {
				hookEntry["env"] = h.Env
			}
			entry := map[string]any{
				"matcher": h.Matcher,
				"hooks":   []map[string]any{hookEntry},
			}
			entries, _ := hooksMap[h.Event].([]any)
			entries = append(entries, entry)
			hooksMap[h.Event] = entries
		}
		settings["hooks"] = hooksMap
	} else if existingHooks, ok := settings["hooks"].(map[string]any); ok {
		delete(existingHooks, "TaskCreated")
		settings["hooks"] = existingHooks
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

	mode := a.statusLineMode
	if !mode.IsValid() {
		mode = resolveStatusLineMode(nil, statusLineStateFromValue(settings["statusLine"]))
	}
	switch mode {
	case config.StatusLineModeMerge:
		settings["statusLine"] = defaultClaudeCombinedStatusLine()
	case config.StatusLineModeReplace:
		settings["statusLine"] = defaultClaudeStatusLine()
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return adapter.FileMapping{}, fmt.Errorf("settings.json 직렬화 실패: %w", err)
	}
	content := append(out, '\n')
	return adapter.FileMapping{
		TargetPath:      filepath.Join(".claude", "settings.json"),
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(content)),
		Content:         content,
	}, nil
}

func writeClaudeMapping(root string, file adapter.FileMapping) error {
	targetPath := filepath.Join(root, file.TargetPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("디렉터리 생성 실패 %s: %w", filepath.Dir(targetPath), err)
	}
	if err := adapter.WriteFileIfChanged(targetPath, file.Content, claudeFileMode(file.TargetPath)); err != nil {
		return fmt.Errorf("파일 쓰기 실패 %s: %w", file.TargetPath, err)
	}
	return nil
}

func claudeFileMode(path string) os.FileMode {
	clean := filepath.ToSlash(path)
	if strings.HasPrefix(clean, ".git/hooks/") || strings.HasSuffix(clean, ".sh") {
		return 0755
	}
	return 0644
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

func defaultClaudeStatusLine() map[string]any {
	return map[string]any{
		"type":    "command",
		"command": autopusClaudeStatusLineCommand,
		"padding": 1,
	}
}

func defaultClaudeCombinedStatusLine() map[string]any {
	return map[string]any{
		"type":    "command",
		"command": autopusClaudeCombinedStatusLineCommand,
		"padding": 1,
	}
}
