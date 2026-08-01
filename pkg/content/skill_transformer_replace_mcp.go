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
