package content

import (
	"strings"
)

// platformPatterns defines line-level patterns to filter per platform.
// Lines containing these patterns are removed for non-Claude platforms.
var platformPatterns = []string{
	"mcp__",
	"Agent(subagent_type=",
	".claude/",
}

// FilterPlatformReferences removes platform-specific references from content.
// For Claude platform, content is returned unchanged.
// For other platforms, lines containing MCP calls, Agent() syntax,
// and .claude/ paths are removed.
func FilterPlatformReferences(body string, platform string) string {
	if platform == "claude" || platform == "claude-code" {
		return body
	}

	lines := strings.Split(body, "\n")
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		if containsPlatformRef(line) {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}

// containsPlatformRef checks if a line contains any platform-specific pattern.
func containsPlatformRef(line string) bool {
	for _, pattern := range platformPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}
