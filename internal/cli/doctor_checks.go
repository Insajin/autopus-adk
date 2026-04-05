// Package cli provides helper functions for doctor command diagnostics.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

// checkQualityGate validates quality preset, review gate providers, and methodology config.
// Returns true if all checks passed, false if any issue was found.
func checkQualityGate(out io.Writer, cfg *config.HarnessConfig) bool {
	allOK := true

	// Check quality preset
	if cfg.Quality.Default != "" {
		if _, ok := cfg.Quality.Presets[cfg.Quality.Default]; ok {
			tui.OK(out, fmt.Sprintf("quality preset: %s", cfg.Quality.Default))
		} else {
			tui.FAIL(out, fmt.Sprintf("quality preset %q not found in presets", cfg.Quality.Default))
			allOK = false
		}
	} else {
		tui.SKIP(out, "quality preset: not configured")
	}

	// Check review gate
	if cfg.Spec.ReviewGate.Enabled {
		tui.OK(out, "review gate: enabled")

		// Check each configured provider
		installedCount := 0
		for _, provName := range cfg.Spec.ReviewGate.Providers {
			if detect.IsInstalled(provName) {
				tui.OK(out, fmt.Sprintf("  provider: %s", provName))
				installedCount++
			} else {
				tui.FAIL(out, fmt.Sprintf("  provider: %s not installed", provName))
				allOK = false
			}
		}
		if installedCount < 2 {
			tui.SKIP(out, "review gate: fewer than 2 providers available")
		}
	} else {
		tui.SKIP(out, "review gate: disabled")
	}

	// Show methodology
	tui.OK(out, fmt.Sprintf("methodology: %s (enforce: %v)", cfg.Methodology.Mode, cfg.Methodology.Enforce))

	return allOK
}

// checkHooksPermissions parses .claude/settings.json and validates hooks and permissions config.
// Returns true if all checks passed, false if any issue was found.
func checkHooksPermissions(out io.Writer, dir string) bool {
	allOK := true

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		tui.SKIP(out, ".claude/settings.json not found (run 'auto init' to generate)")
		return allOK
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		tui.FAIL(out, "settings.json 파싱 실패")
		return false
	}

	// Check hooks
	if hooksVal, ok := settings["hooks"]; ok {
		if hooksMap, ok := hooksVal.(map[string]interface{}); ok && len(hooksMap) > 0 {
			tui.OK(out, fmt.Sprintf("hooks: %d event(s) configured", len(hooksMap)))
		} else {
			tui.SKIP(out, "hooks: empty or invalid format")
		}
	} else {
		tui.SKIP(out, "hooks: not configured (run 'auto update' to install)")
	}

	// Check permissions
	if permsVal, ok := settings["permissions"]; ok {
		if permsMap, ok := permsVal.(map[string]interface{}); ok {
			if allowList, ok := permsMap["allow"].([]interface{}); ok && len(allowList) > 0 {
				tui.OK(out, fmt.Sprintf("permissions: %d allow rule(s)", len(allowList)))
			} else {
				tui.SKIP(out, "permissions.allow: empty")
			}
		}
	} else {
		tui.SKIP(out, "permissions: not configured (run 'auto update' to install)")
	}

	return allOK
}
