package content

import "strings"

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
		case "gemini", "gemini-cli", "antigravity-cli":
			if task != "" {
				return `@` + name + ` ` + task
			}
			return `@` + name
		case "opencode", "omp":
			if task != "" {
				return `task tool → subagent_type="` + name + `", prompt="` + task + `"`
			}
			return `task tool → subagent_type="` + name + `"`
		default:
			return match
		}
	})
}

// replaceTodoWrite rewrites TodoWrite to the platform's own todo tool name, and
// comments the reference out on platforms that have no equivalent.
// The per-platform rewrite returns immediately so that a platform tool name is
// never used as a skip condition for another platform.
func replaceTodoWrite(line string, platform string) string {
	if strings.Contains(line, "TodoWrite") {
		switch normalizePlatform(platform) {
		case "opencode":
			return todoWriteRe.ReplaceAllString(line, "todowrite")
		case "omp":
			return todoWriteRe.ReplaceAllString(line, "todo")
		}
	}
	if strings.Contains(line, "todowrite") {
		return line
	}
	if todoWriteRe.MatchString(line) {
		return "// TodoWrite is not available on this platform"
	}
	return line
}

// ompWorkflowToolReplacer rewrites Claude-only workflow tool names for omp.
// Only `todo` is an omp canonical tool name (REQ-006 mapping table); the omp
// tool surface measured for this SPEC has no counterpart for the interactive
// question and team-messaging tools, so those become neutral prose instead of
// an invented tool name.
var ompWorkflowToolReplacer = strings.NewReplacer(
	"AskUserQuestion", "user prompt",
	"request_user_input", "user prompt",
	"TaskCreate", "todo",
	"TaskUpdate", "todo",
	"TaskList", "todo",
	"TaskGet", "todo",
	"TeamCreate", "parallel agent run",
	"SendMessage", "agent result handoff",
)

var openCodeWorkflowToolReplacer = strings.NewReplacer(
	"AskUserQuestion", "question",
	"request_user_input", "question",
	"TaskCreate", "todowrite",
	"TaskUpdate", "todowrite",
	"TaskList", "todowrite",
	"TaskGet", "todowrite",
	"TeamCreate", "task",
	"SendMessage", "task result handoff",
)

func replaceWorkflowTools(line string, platform string) string {
	switch normalizePlatform(platform) {
	case "opencode":
		return openCodeWorkflowToolReplacer.Replace(line)
	case "omp":
		return ompWorkflowToolReplacer.Replace(line)
	default:
		return line
	}
}
