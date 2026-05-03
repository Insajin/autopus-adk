package codex

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

func (a *Adapter) renderRouterSkill(cfg *config.HarnessConfig) (string, error) {
	tmplContent, err := templates.FS.ReadFile("codex/prompts/auto.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("codex router skill 템플릿 읽기 실패: %w", err)
	}
	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("codex router skill 템플릿 렌더링 실패: %w", err)
	}
	_, body := splitSkillFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		body = rendered
	}
	body = strings.TrimSpace(body)
	body = rewriteCodexRouterBody(body)
	body = normalizeCodexInvocationBody(body)
	body = normalizeCodexHelperPaths(body)
	invoNote := strings.TrimSpace(fmt.Sprintf(`
## Codex Invocation

Use this skill through either of these surfaces:

- %s — preferred when the local Autopus plugin is installed from %s
- %s — direct repository skill invocation

Direct skill loads should treat the user's latest %s request as the arguments.
This skill is a thin router. After resolving the subcommand, load the matching detailed skill (%s) before executing the workflow.
`,
		"`@auto <subcommand> ...`",
		"`/plugins`",
		"`$auto <subcommand> ...`",
		"`auto ...`",
		routerDetailSkills(),
	))
	body = injectAfterFirstHeading(body, invoNote)
	body = injectAfterFirstHeading(body, codexRouterExecutionContract())
	body = injectCodexBrandingBlock(body, true)
	frontmatter := strings.TrimSpace(fmt.Sprintf(`---
name: auto
description: >
  Autopus Codex router skill. Use when the user wants %s or %s workflows such as setup, status, plan, go, fix, review, sync, idea, map, why, verify, secure, test, qa, dev, canary, and doctor.
---`, "`@auto ...`", "`$auto ...`"))
	return frontmatter + "\n\n" + strings.TrimSpace(body) + "\n", nil
}

func (a *Adapter) renderWorkflowSkill(cfg *config.HarnessConfig, spec workflowSpec) (string, error) {
	if rendered, ok := renderCustomWorkflowSkill(spec); ok {
		return rendered, nil
	}
	if spec.SkillPath == "" {
		return "", fmt.Errorf("codex workflow skill 경로 누락: %s", spec.Name)
	}

	tmplContent, err := templates.FS.ReadFile(spec.SkillPath)
	if err != nil {
		return "", fmt.Errorf("codex skill 템플릿 읽기 실패 %s: %w", spec.SkillPath, err)
	}
	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("codex skill 템플릿 렌더링 실패 %s: %w", spec.Name, err)
	}
	_, body := splitSkillFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		body = rendered
	}
	body = strings.TrimSpace(body)
	body = pkgcontent.ReplacePlatformReferences(body, "codex")
	body = normalizeCodexSkillBody(body, strings.TrimPrefix(spec.Name, "auto-"))
	body = injectCodexBrandingBlock(body, false)
	if !strings.Contains(body, "## Codex Invocation") {
		invocationNote := strings.TrimSpace(fmt.Sprintf(`
## Codex Invocation

You can invoke this workflow through any of these compatible surfaces:

- %s — preferred when the local Autopus plugin is installed
- %s — direct repository skill invocation
- %s — via the router skill

Load and follow any helper documents referenced from this file under %s and %s.
`,
			fmt.Sprintf("`@auto %s ...`", strings.TrimPrefix(spec.Name, "auto-")),
			fmt.Sprintf("`$%s ...`", spec.Name),
			fmt.Sprintf("`$auto %s ...`", strings.TrimPrefix(spec.Name, "auto-")),
			"`.codex/skills/`",
			"`.codex/rules/autopus/`",
		))
		body = injectAfterFirstHeading(body, invocationNote)
	}

	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\n---", spec.Name, spec.Description)
	return frontmatter + "\n\n" + body + "\n", nil
}

func normalizeCodexSkillBody(body, subcommand string) string {
	body = normalizeCodexInvocationBody(body)
	body = normalizeCodexHelperPaths(body)
	body = normalizeCodexToolingBody(body)
	if subcommand == "" {
		return body
	}
	replacer := strings.NewReplacer(
		fmt.Sprintf("@auto-%s", subcommand), fmt.Sprintf("@auto %s", subcommand),
		fmt.Sprintf("$auto-%s", subcommand), fmt.Sprintf("$auto %s", subcommand),
	)
	return replacer.Replace(body)
}

func injectCodexBrandingBlock(body string, router bool) string {
	if strings.Contains(body, "## Autopus Branding") {
		return body
	}
	title := "this workflow"
	if router {
		title = "`@auto` router responses"
	}
	block := strings.TrimSpace(fmt.Sprintf(
		"## Autopus Branding\n\n"+
			"When handling %s, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:\n\n"+
			"```text\n"+
			"🐙 Autopus ─────────────────────────\n"+
			"```\n\n"+
			"End the completed response with `🐙`.\n",
		title,
	))
	return injectAfterFirstHeading(body, block)
}

func decorateCodexWorkflowPrompt(rendered string, router bool) string {
	frontmatter, body := splitSkillFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		frontmatter = ""
		body = rendered
	}

	body = strings.TrimSpace(body)
	if router {
		body = rewriteCodexRouterBody(body)
		body = injectAfterFirstHeading(body, codexRouterExecutionContract())
	}
	body = injectCodexBrandingBlock(body, router)

	if strings.TrimSpace(frontmatter) == "" {
		return strings.TrimSpace(body) + "\n"
	}
	return frontmatter + "\n\n" + strings.TrimSpace(body) + "\n"
}

func codexRouterExecutionContract() string {
	return fmt.Sprintf("## Router Execution Contract\n\n"+
		"- Treat this file as a thin entrypoint only.\n"+
		"- After resolving the subcommand, immediately load the matching detailed workflow surface (%s) before answering or acting.\n"+
		"- Do not stay at the router layer when a detailed workflow exists for the request.\n"+
		"- Always load the project context documents before routing or executing the workflow.\n\n"+
		"## Context Load\n\n"+
		"Before processing any `@auto` subcommand, read these files if they exist:\n\n"+
		codexProjectContextLoadList()+"\n"+
		"- If none of these files exist, explicitly note that project context is missing and recommend `@auto setup`.\n"+
		"- Do not skip this load step just because the subcommand looks obvious.\n\n"+
		"## SPEC Path Resolution\n\n"+
		"When any workflow receives a SPEC-ID, resolve the actual file path before opening files, spawning workers, or running build/test commands:\n\n"+
		"1. Check `.autopus/specs/{SPEC-ID}/spec.md` (top-level, cross-module or legacy SPECs).\n"+
		"2. Recursively search `**/.autopus/specs/{SPEC-ID}/spec.md`, skipping `.git`, `node_modules`, `vendor`, `.cache`, and `dist`.\n\n"+
		"From the resolved path, extract:\n\n"+
		"- `SPEC_PATH`: full path to `spec.md`\n"+
		"- `SPEC_DIR`: parent SPEC directory\n"+
		"- `TARGET_MODULE`: submodule path, or `.` for top-level SPECs\n"+
		"- `WORKING_DIR`: the directory where build/test commands run (`TARGET_MODULE` or `.`)\n\n"+
		"Error handling:\n\n"+
		"- 0 matches: report the SPEC is missing and list available SPEC IDs.\n"+
		"- 2+ matches: report the duplicate paths and stop for clarification.\n"+
		"- All detailed workflows must use the resolved values instead of assuming `.autopus/specs/{SPEC-ID}` is rooted at the current directory.\n",
		routerDetailSkills(),
	)
}

func rewriteCodexRouterBody(body string) string {
	body = strings.TrimSpace(body)
	body = injectRouterSupportedFlows(body)
	body = strings.ReplaceAll(body, "위 7개", fmt.Sprintf("위 %d개", routerSubcommandCount()))
	body = strings.ReplaceAll(body, "위 8개", fmt.Sprintf("위 %d개", routerSubcommandCount()))
	body = strings.ReplaceAll(body, "같은 이름의 상세 스킬/프롬프트(`auto-setup`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-canary`, `auto-idea`)", "같은 이름의 상세 스킬/프롬프트("+routerDetailSkills()+")")
	return body
}

func injectRouterSupportedFlows(body string) string {
	start := strings.Index(body, "지원 서브커맨드:")
	rules := strings.Index(body, "\n\n규칙:")
	section := "지원 서브커맨드:\n" + routerSupportedFlows()
	if start < 0 || rules < 0 || rules <= start {
		return strings.TrimSpace(body) + "\n\n" + section
	}
	return body[:start] + section + body[rules:]
}

func normalizeCodexInvocationBody(body string) string {
	replacer := strings.NewReplacer(
		"`/auto ", "`@auto ",
		"/auto ", "@auto ",
		"`/auto`", "`@auto`",
	)
	return replacer.Replace(body)
}

func normalizeCodexHelperPaths(body string) string {
	replacer := strings.NewReplacer(
		"@.codex/skills/autopus/", "@.codex/skills/",
		".codex/skills/autopus/", ".codex/skills/",
		"@.claude/skills/autopus/", "@.codex/skills/",
		".claude/skills/autopus/", ".codex/skills/",
		".codex/agents/autopus/", ".codex/agents/",
		".claude/agents/autopus/", ".codex/agents/",
		".claude/rules/autopus/", ".codex/rules/autopus/",
		"`content/rules/branding.md`", "`.codex/rules/autopus/branding.md`",
		"`branding-formats.md.tmpl`", "`templates/shared/branding-formats.md.tmpl`",
	)
	return replacer.Replace(body)
}

func normalizeCodexToolingBody(body string) string {
	replacer := strings.NewReplacer(
		"Load the `mcp__sequential-thinking__sequentialthinking` tool via ToolSearch, then perform step-by-step reasoning.", "Use sequential-thinking tooling if available; otherwise perform explicit step-by-step reasoning in the main Codex session.",
		"Load the `WebSearch-thinking__sequentialthinking` tool via ToolSearch, then perform step-by-step reasoning.", "Use sequential-thinking tooling if available; otherwise perform explicit step-by-step reasoning in the main Codex session.",
		"Use TeamCreate to create a team, then spawn specialized teammates using `Agent(subagent_type=..., team_name=..., name=...)`. Each teammate loads its agent definition from `.codex/agents/`, inheriting tools, skills, model, and domain expertise. Teammates communicate directly via SendMessage.", "Spawn specialized agents with `spawn_agent(...)` and coordinate them from the main session with `send_input(...)` and `wait_agent(...)`. Assign each worker a disjoint write scope and do not rely on Claude Code Team APIs.",
		"For parallel tasks, include `auto pipeline worktree` in Agent() calls to enable worktree isolation.", "For parallel tasks, spawn separate workers with disjoint write scopes and follow `.codex/skills/worktree-isolation.md` for branch-isolation guidance.",
		"For parallel tasks, include `auto pipeline worktree` in spawn_agent() calls to enable worktree isolation.", "For parallel tasks, spawn separate workers with disjoint write scopes and follow `.codex/skills/worktree-isolation.md` for branch-isolation guidance.",
		"Claude Code automatically creates a worktree when `auto pipeline worktree` is passed to Agent(). No manual `git worktree add` is needed.", "Codex should not assume implicit worktree provisioning from agent flags. Use the documented worktree-isolation procedure when branch separation is required.",
		"Each Phase below MUST use an Agent() call", "Each Phase below MUST use a `spawn_agent(...)` call",
		"using the Agent tool", "using the `spawn_agent` tool",
		"maps to Codex subagent spawning.", "maps to Codex subagent spawning.",
		"Phase 0.5: Permission    → detect      (auto permission detect)", "Phase 0.5: Permission    → main session (decide autonomy vs confirmation)",
	)
	body = replacer.Replace(body)
	body = strings.ReplaceAll(body, "Agent(", "spawn_agent(")
	body = strings.ReplaceAll(body, "subagent_type =", "agent_type =")
	body = strings.ReplaceAll(body, "prompt = ", "message = ")
	body = strings.ReplaceAll(body, " (parallel, mode: plan)", " (parallel)")
	body = strings.ReplaceAll(body, " (mode: plan)", "")
	body = strings.ReplaceAll(body, " (mode: bypassPermissions)", "")
	body = strings.ReplaceAll(body, " (mode: bypassPermissions, parallel with worktree isolation)", " (parallel with worktree isolation)")
	body = strings.ReplaceAll(body, "  mode = PERMISSION_MODE == \"bypass\" ? \"bypassPermissions\" : \"plan\",\n", "")
	body = strings.ReplaceAll(body, "  mode = \"bypassPermissions\",\n", "")
	body = strings.ReplaceAll(body, "  mode = \"plan\",\n", "")
	body = strings.ReplaceAll(body, "    permissionMode = \"bypassPermissions\",\n", "")
	body = strings.ReplaceAll(body, "    permissionMode = \"plan\",\n", "")
	body = strings.ReplaceAll(body, "  permissionMode = \"bypassPermissions\",\n", "")
	body = strings.ReplaceAll(body, "  permissionMode = \"plan\",\n", "")
	body = strings.ReplaceAll(body, "PERMISSION_MODE=$(auto permission detect)\n", "")
	body = strings.ReplaceAll(body, "If the command fails or is unavailable, default to `PERMISSION_MODE=\"safe\"`.\n", "")
	return body
}

func splitSkillFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", strings.TrimSpace(content)
	}

	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", strings.TrimSpace(content)
	}

	frontmatter := strings.TrimSpace(content[:len("---\n")+idx+len("\n---")])
	body := strings.TrimSpace(rest[idx+len("\n---\n"):])
	return frontmatter, body
}

func injectAfterFirstHeading(body, block string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			out := make([]string, 0, len(lines)+4)
			out = append(out, lines[:i+1]...)
			out = append(out, "")
			out = append(out, block)
			out = append(out, "")
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	return block + "\n\n" + body
}
