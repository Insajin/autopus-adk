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

var mcpResolveNameRe = regexp.MustCompile(`mcp__context7__resolve-library-id`)
var mcpQueryNameRe = regexp.MustCompile(`mcp__context7__query-docs`)

// mcpGenericRe matches any remaining mcp__ references not caught by specific patterns.
var mcpGenericRe = regexp.MustCompile(`mcp__[\w-]+(?:__[\w-]+)*`)

var openCodeSkillPathRe = regexp.MustCompile(`\.agents/skills/([a-z0-9-]+)\.md`)

// pathReplacements maps Claude-specific directory prefixes to platform equivalents.
var pathReplacements = map[string]map[string]string{
	"codex": {
		".claude/commands/": ".codex/prompts/",
		".claude/skills/":   ".codex/skills/",
		".claude/agents/":   ".codex/agents/",
		".claude/rules/":    ".codex/rules/",
		".claude/":          ".codex/",
	},
	"gemini": {
		".claude/commands/": ".gemini/commands/",
		".claude/skills/":   ".gemini/skills/",
		".claude/agents/":   ".gemini/agents/",
		".claude/rules/":    ".gemini/rules/",
		".claude/":          ".gemini/",
	},
	"opencode": {
		".claude/skills/autopus/": ".agents/skills/",
		".claude/commands/":       ".opencode/commands/",
		".claude/skills/":         ".agents/skills/",
		".claude/agents/":         ".opencode/agents/",
		".claude/rules/":          ".opencode/rules/",
		".claude/hooks/":          ".opencode/plugins/",
		".claude/":                ".opencode/",
	},
}

// pathOrder ensures specific paths are replaced before the general .claude/ prefix.
var pathOrder = []string{
	".claude/commands/",
	".claude/skills/autopus/",
	".claude/skills/",
	".claude/agents/",
	".claude/rules/",
	".claude/hooks/",
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
		line = replaceTodoWrite(line, platform)
		line = replaceWorkflowTools(line, platform)
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// NormalizeAgentReferences applies platform-specific path fixes that should be
// preserved across generated agent surfaces.
func NormalizeAgentReferences(body, platform string) string {
	p := normalizePlatform(platform)
	normalized := body
	if p != "claude" {
		normalized = ReplacePlatformReferences(normalized, p)
	}

	brandingRule := map[string]string{
		"claude":   "`.claude/rules/autopus/branding.md`",
		"codex":    "`.codex/rules/autopus/branding.md`",
		"gemini":   "`.gemini/rules/autopus/branding.md`",
		"opencode": "`.opencode/rules/autopus/branding.md`",
	}[p]
	if brandingRule == "" {
		brandingRule = "`content/rules/branding.md`"
	}

	replacer := strings.NewReplacer(
		"`content/rules/branding.md`", brandingRule,
		"`branding-formats.md.tmpl`", "`templates/shared/branding-formats.md.tmpl`",
	)
	return replacer.Replace(normalized)
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
		case "opencode":
			if task != "" {
				return `task tool → subagent_type="` + name + `", prompt="` + task + `"`
			}
			return `task tool → subagent_type="` + name + `"`
		default:
			return match
		}
	})
}

// replaceMCPCalls converts mcp__context7__ calls into platform-neutral guidance
// that preserves the intended Context7-first, WebSearch-fallback behavior.
func replaceMCPCalls(line string, _ string) string {
	line = mcpResolveRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := mcpResolveRe.FindStringSubmatch(match)
		lib := "library"
		if len(sub) >= 2 && sub[1] != "" {
			lib = cleanArg(sub[1])
		}
		return buildContext7FallbackText(lib, "")
	})

	line = mcpQueryRe.ReplaceAllStringFunc(line, func(match string) string {
		sub := mcpQueryRe.FindStringSubmatch(match)
		lib := "library"
		topic := ""
		if len(sub) >= 2 && sub[1] != "" {
			lib, topic = parseContext7QueryArgs(sub[1])
		}
		return buildContext7FallbackText(lib, topic)
	})

	line = mcpResolveNameRe.ReplaceAllString(line, "Context7 MCP resolve-library-id tool (fallback: web search)")
	line = mcpQueryNameRe.ReplaceAllString(line, "Context7 MCP query-docs tool (fallback: web search)")

	// Replace any remaining generic mcp__ references
	line = mcpGenericRe.ReplaceAllString(line, "Context7 MCP with WebSearch fallback")

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
	if p == "opencode" {
		line = openCodeSkillPathRe.ReplaceAllString(line, ".agents/skills/$1/SKILL.md")
	}
	return line
}

// replaceWorktreeIsolation converts isolation: "worktree" references.
// Reuses worktreeIsolationRe from agent_transformer_mapping.go.
func replaceWorktreeIsolation(line string) string {
	return worktreeIsolationRe.ReplaceAllString(line, "auto pipeline worktree")
}

// replaceTodoWrite removes or comments out TodoWrite tool references.
func replaceTodoWrite(line string, platform string) string {
	if normalizePlatform(platform) == "opencode" && strings.Contains(line, "TodoWrite") {
		line = todoWriteRe.ReplaceAllString(line, "todowrite")
	}
	if strings.Contains(line, "todowrite") {
		return line
	}
	if todoWriteRe.MatchString(line) {
		return "// TodoWrite is not available on this platform"
	}
	return line
}

func replaceWorkflowTools(line string, platform string) string {
	if normalizePlatform(platform) != "opencode" {
		return line
	}

	replacer := strings.NewReplacer(
		"AskUserQuestion", "question",
		"TaskCreate", "todowrite",
		"TaskUpdate", "todowrite",
		"TaskList", "todowrite",
		"TaskGet", "todowrite",
		"TeamCreate", "task",
		"SendMessage", "task result handoff",
	)
	return replacer.Replace(line)
}

// cleanArg strips quotes and whitespace from a function argument string.
func cleanArg(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return s
}

func parseContext7QueryArgs(argString string) (string, string) {
	parts := strings.Split(argString, ",")
	if len(parts) == 0 {
		return "library", ""
	}

	lib := cleanArg(parts[0])
	if lib == "" {
		lib = "library"
	}

	topic := ""
	for _, raw := range parts[1:] {
		part := strings.TrimSpace(raw)
		if !strings.HasPrefix(part, "topic=") {
			continue
		}
		topic = cleanArg(strings.TrimPrefix(part, "topic="))
		break
	}

	return lib, topic
}

func buildWebSearchQuery(lib, topic string) string {
	query := lib
	if topic != "" {
		query += " " + topic
	}
	query += " docs"
	return strings.TrimSpace(query)
}

func buildContext7FallbackText(lib, topic string) string {
	query := buildWebSearchQuery(lib, topic)
	text := `Context7 MCP first; fallback: WebSearch "` + query + `"`
	if topic != "" {
		text += ` (topic: ` + topic + `)`
	}
	return text
}
