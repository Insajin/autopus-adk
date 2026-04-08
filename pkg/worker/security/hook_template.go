package security

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// hookEntry represents a single hook matcher and its commands.
type hookEntry struct {
	Matcher string   `json:"matcher"`
	Hooks   []string `json:"hooks"`
}

// GenerateHookConfig returns the hook config map for Claude Code settings.json.
// policyPath is the absolute path to the SecurityPolicy file.
func GenerateHookConfig(policyPath string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []hookEntry{
				{
					Matcher: "Bash|Write|Edit",
					Hooks: []string{
						fmt.Sprintf("auto worker validate --policy '%s' --command \"$TOOL_INPUT\"", policyPath),
					},
				},
			},
		},
	}
}

// WriteHookConfig writes the hook configuration to the given directory's
// .claude/settings.json, merging with existing settings if present.
func WriteHookConfig(dir string, policyPath string) error {
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")
	existing := make(map[string]any)

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse existing settings.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read settings.json: %w", err)
	}

	hookConfig := GenerateHookConfig(policyPath)
	maps.Copy(existing, hookConfig)

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}

	if err := adapter.WriteFileIfChanged(settingsPath, out, 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}
	return nil
}

// RemoveHookConfig removes the worker validate hook from settings.json.
func RemoveHookConfig(dir string) error {
	settingsPath := filepath.Join(dir, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings.json: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings.json: %w", err)
	}

	// Only remove the worker-specific PreToolUse hook, preserving other user hooks.
	if hooks, ok := settings["hooks"].(map[string]any); ok {
		delete(hooks, "PreToolUse")
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}

	if err := adapter.WriteFileIfChanged(settingsPath, out, 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}
	return nil
}
