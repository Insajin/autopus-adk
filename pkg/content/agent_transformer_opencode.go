package content

import (
	"fmt"
	"sort"
	"strings"
)

var openCodeToolMap = map[string]string{
	"Bash":       "bash",
	"Edit":       "edit",
	"Glob":       "glob",
	"Grep":       "grep",
	"Read":       "read",
	"TodoWrite":  "todowrite",
	"WebFetch":   "webfetch",
	"WebSearch":  "websearch",
	"Write":      "edit",
	"WriteFile":  "edit",
	"MultiEdit":  "edit",
	"ApplyPatch": "edit",
}

// TransformAgentForOpenCode produces an OpenCode markdown agent definition.
func TransformAgentForOpenCode(src AgentSource) string {
	body := NormalizeAgentReferences(src.Body, "opencode")
	permissions := buildOpenCodePermissions(src)

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "description: %q\n", src.Meta.Description)
	sb.WriteString("mode: subagent\n")
	if src.Meta.MaxTurns > 0 {
		fmt.Fprintf(&sb, "steps: %d\n", src.Meta.MaxTurns)
	}
	sb.WriteString("permission:\n")
	sb.WriteString("  \"*\": deny\n")
	for _, key := range permissions {
		fmt.Fprintf(&sb, "  %q: allow\n", key)
	}
	sb.WriteString("---\n\n")

	if len(src.Meta.Skills) > 0 {
		sb.WriteString("Use the following Autopus skills when they fit the task: ")
		for i, skill := range src.Meta.Skills {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "`%s`", skill)
		}
		sb.WriteString(".\n\n")
	}

	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}

	return sb.String()
}

func buildOpenCodePermissions(src AgentSource) []string {
	seen := map[string]bool{
		"question": true,
		"skill":    true,
		"task":     true,
	}
	for _, item := range strings.Split(src.Meta.Tools, ",") {
		tool := strings.TrimSpace(item)
		if tool == "" {
			continue
		}
		mapped, ok := openCodeToolMap[tool]
		if ok {
			seen[mapped] = true
			continue
		}
		seen[tool] = true
	}

	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
