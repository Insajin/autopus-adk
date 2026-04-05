// Package content provides platform-specific reference replacement for skill content.
package content

import (
	"regexp"
	"strings"
)

// mcpResolveRe matches mcp__context7__resolve-library-id(...) calls with arguments.
var mcpResolveRe = regexp.MustCompile(
	`mcp__context7__resolve-library-id\(([^)]*)\)`,
)

// mcpQueryRe matches mcp__context7__query-docs(...) calls with arguments.
var mcpQueryRe = regexp.MustCompile(
	`mcp__context7__query-docs\(([^)]*)\)`,
)

// mcpGenericRe matches any remaining mcp__ references not caught by specific patterns.
var mcpGenericRe = regexp.MustCompile(`mcp__\w+`)

// pathReplacements maps Claude-specific directory prefixes to platform equivalents.
var pathReplacements = map[string]map[string]string{
	"codex": {
		".claude/skills/": ".codex/skills/",
		".claude/agents/": ".codex/agents/",
		".claude/rules/":  ".codex/rules/",
		".claude/":        ".codex/",
	},
	"gemini": {
		".claude/skills/": ".gemini/skills/",
		".claude/agents/": ".gemini/agents/",
		".claude/rules/":  ".gemini/rules/",
		".claude/":        ".gemini/",
	},
}

// pathOrder ensures specific paths are replaced before the general .claude/ prefix.
var pathOrder = []string{
	".claude/skills/",
	".claude/agents/",
	".claude/rules/",
	".claude/",
}

// ReplacePlatformReferences replaces Claude-specific references with platform equivalents.
// For Claude platforms, content is returned unchanged (backward compat — S8).
func ReplacePlatformReferences(body string, platform string) string {
	if platform == "claude" || platform == "claude-code" {
		return body
	}

	lines := strings.Split(body, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = replaceAgentCalls(line, platform)
		line = replaceMCPCalls(line, platform)
		line = replacePaths(line, platform)
		line = replaceWorktreeIsolation(line)
		line = replaceTodoWrite(line)
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// replaceAgentCalls converts Agent(subagent_type="X", task="Y") to platform syntax.
// Reuses agentMappingRe from agent_transformer_mapping.go.
func replaceAgentCalls(line string, platform string) string {
	return agentMappingRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := agentMappingRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := sub[1]
		task := ""
		if len(sub) >= 3 {
			task = sub[2]
		}

		switch platform {
		case "codex":
			if task != "" {
				return `spawn_agent ` + name + ` --task "` + task + `"`
			}
			return `spawn_agent ` + name
		case "gemini", "gemini-cli":
			if task != "" {
				return `@` + name + ` ` + task
			}
			return `@` + name
		default:
			return match
		}
	})
}

// replaceMCPCalls converts mcp__context7__ calls to WebSearch equivalents.
// Uses detailed regexes to extract library/topic arguments for richer replacements.
func replaceMCPCalls(line string, _ string) string {
	line = mcpResolveRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := mcpResolveRe.FindStringSubmatch(match)
		lib := "library"
		if len(sub) >= 2 && sub[1] != "" {
			lib = cleanArg(sub[1])
		}
		return `WebSearch "` + lib + ` docs"`
	})

	line = mcpQueryRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := mcpQueryRe.FindStringSubmatch(match)
		args := "library"
		if len(sub) >= 2 && sub[1] != "" {
			args = cleanArg(sub[1])
		}
		return `WebSearch "` + args + ` docs"`
	})

	// Replace any remaining generic mcp__ references
	line = mcpGenericRe.ReplaceAllString(line, "WebSearch")

	return line
}

// replacePaths converts .claude/ directory references to platform-specific paths.
func replacePaths(line string, platform string) string {
	p := normalizePlatform(platform)
	paths, ok := pathReplacements[p]
	if !ok {
		return line
	}

	for _, key := range pathOrder {
		if repl, exists := paths[key]; exists {
			line = strings.ReplaceAll(line, key, repl)
		}
	}
	return line
}

// replaceWorktreeIsolation converts isolation: "worktree" references.
// Reuses worktreeIsolationRe from agent_transformer_mapping.go.
func replaceWorktreeIsolation(line string) string {
	return worktreeIsolationRe.ReplaceAllString(line, "auto pipeline worktree")
}

// replaceTodoWrite removes or comments out TodoWrite tool references.
func replaceTodoWrite(line string) string {
	if todoWriteRe.MatchString(line) {
		return "// TodoWrite is not available on this platform"
	}
	return line
}

// cleanArg strips quotes and whitespace from a function argument string.
func cleanArg(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return s
}
