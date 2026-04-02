package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InjectOrchestraAfterAgentHook adds the autopus orchestra result collector
// AfterAgent hook to .gemini/settings.json, preserving existing user hooks.
// This is session-specific and injected separately from harness-managed hooks.
// Duplicate entries with the same command are skipped.
func (a *Adapter) InjectOrchestraAfterAgentHook(scriptPath string) error {
	settingsDir := filepath.Join(a.root, ".gemini")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("create .gemini dir: %w", err)
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

	// Ensure hooks map exists
	hooksMap, _ := settings["hooks"].(map[string]any)
	if hooksMap == nil {
		hooksMap = make(map[string]any)
	}

	entry := map[string]any{
		"command": scriptPath,
	}

	// Check for duplicate command before appending
	existing, _ := hooksMap["AfterAgent"].([]any)
	for _, e := range existing {
		if m, ok := e.(map[string]any); ok && m["command"] == scriptPath {
			// Already present — skip to avoid duplicate
			return nil
		}
	}
	hooksMap["AfterAgent"] = append(existing, entry)
	settings["hooks"] = hooksMap

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(settingsPath, append(out, '\n'), 0644)
}
