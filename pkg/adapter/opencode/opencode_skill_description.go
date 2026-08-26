package opencode

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

const openCodeOnlyDescriptionPrefix = "[OpenCode-only] "

func openCodeSkillDescription(cfg *config.HarnessConfig, description string) string {
	if cfg == nil || !containsString(cfg.Platforms, "codex") || !containsString(cfg.Platforms, "opencode") {
		return description
	}
	if strings.HasPrefix(description, openCodeOnlyDescriptionPrefix) {
		return description
	}
	return openCodeOnlyDescriptionPrefix + description
}
