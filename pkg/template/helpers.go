// Package template provides cross-platform template helper functions.
package template

import (
	"unicode/utf8"

	"github.com/insajin/autopus-adk/pkg/config"
)

// SkillMeta holds minimal skill metadata extracted from harness config.
type SkillMeta struct {
	Name        string
	Description string
}

// TruncateToBytes truncates content to maxBytes, respecting UTF-8 boundaries.
// If content fits within maxBytes, it is returned as-is.
// Otherwise, truncation occurs at the last valid UTF-8 boundary before maxBytes.
func TruncateToBytes(content string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(content) <= maxBytes {
		return content
	}
	// Walk backwards from maxBytes to find a valid UTF-8 boundary.
	for maxBytes > 0 && !utf8.RuneStart(content[maxBytes]) {
		maxBytes--
	}
	return content[:maxBytes]
}

// MapPermission maps a Claude permission mode to the equivalent mode on the
// target platform. Returns an empty string for unknown platform/mode combos.
func MapPermission(claudeMode string, targetPlatform string) string {
	m, ok := permissionMap[targetPlatform]
	if !ok {
		return ""
	}
	return m[claudeMode] // returns "" for unknown mode
}

var permissionMap = map[string]map[string]string{
	"codex": {
		"plan":   "on-request",
		"act":    "auto",
		"bypass": "never",
	},
	"antigravity-cli": {
		"plan":   "plan",
		"act":    "auto_edit",
		"bypass": "yolo",
	},
	"gemini-cli": {
		"plan":   "plan",
		"act":    "auto_edit",
		"bypass": "yolo",
	},
}

// SkillList extracts skill metadata from the harness config's category weights.
// Each category key becomes a SkillMeta entry. Returns nil if no categories exist.
func SkillList(cfg *config.HarnessConfig) []SkillMeta {
	if cfg == nil || len(cfg.Skills.CategoryWeights) == 0 {
		return nil
	}
	result := make([]SkillMeta, 0, len(cfg.Skills.CategoryWeights))
	for name := range cfg.Skills.CategoryWeights {
		result = append(result, SkillMeta{Name: name})
	}
	return result
}
