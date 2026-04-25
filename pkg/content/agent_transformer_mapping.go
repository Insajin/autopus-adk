package content

import (
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

// modelMapping maps source model tiers to platform-specific model names.
var modelMapping = map[string]map[string]string{
	"codex": {
		"sonnet": config.CodexFrontierModel,
		"opus":   config.CodexFrontierModel,
		"haiku":  config.CodexFrontierModel,
	},
	"gemini": {
		"sonnet": "gemini-2.5-pro",
		"opus":   "gemini-2.5-pro",
		"haiku":  "gemini-2.5-flash",
	},
}

// effortMapping maps source effort tiers to platform-specific reasoning controls.
var effortMapping = map[string]map[string]string{
	"codex": {
		"max":    "xhigh",
		"xhigh":  "xhigh",
		"high":   "high",
		"medium": "medium",
		"low":    "low",
	},
}

// agentMappingRe matches Agent(subagent_type="X", task/prompt="Y") patterns for platform mapping.
var agentMappingRe = regexp.MustCompile(
	`Agent\(subagent_type="([^"]+)"(?:,\s*(?:task|prompt)="([^"]*)")?\s*(?:,\s*[^)]*?)?\)`,
)

// todoWriteRe matches TodoWrite tool references.
var todoWriteRe = regexp.MustCompile(`\bTodoWrite\b`)

// worktreeIsolationRe matches isolation: "worktree" references.
var worktreeIsolationRe = regexp.MustCompile(`isolation:\s*"worktree"`)

// MapModel returns the platform-specific model name for a source model tier.
func MapModel(model, platform string) string {
	if pm, ok := modelMapping[platform]; ok {
		if mapped, ok := pm[model]; ok {
			return mapped
		}
	}
	return model
}

// MapEffort returns the platform-specific effort value for a source effort tier.
func MapEffort(effort, platform string) string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	if pm, ok := effortMapping[platform]; ok {
		if normalized == "" {
			return "medium"
		}
		if mapped, ok := pm[normalized]; ok {
			return mapped
		}
	}
	return effort
}

// ReplaceToolReferences applies R3 tool reference mappings to body text.
// Delegates to ReplacePlatformReferences as the single source of truth.
func ReplaceToolReferences(body, platform string) string {
	return ReplacePlatformReferences(body, platform)
}

// normalizePlatform normalizes platform aliases.
func normalizePlatform(platform string) string {
	switch platform {
	case "claude-code":
		return "claude"
	case "gemini-cli":
		return "gemini"
	default:
		return platform
	}
}
