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
	body = normalizeCodexInvocationBody(body)
	body = normalizeCodexHelperPaths(body)
	invoNote := strings.TrimSpace(fmt.Sprintf(`
## Codex Invocation

Use this skill through either of these surfaces:

- %s — preferred when the local Autopus plugin is installed from %s
- %s — direct repository skill invocation

Direct skill loads should treat the user's latest %s request as the arguments.
This skill is a thin router. After resolving the subcommand, load the matching detailed skill (%s, %s, %s, %s, %s, %s, %s) before executing the workflow.
`,
		"`@auto <subcommand> ...`",
		"`/plugins`",
		"`$auto <subcommand> ...`",
		"`auto ...`",
		"`auto-plan`",
		"`auto-go`",
		"`auto-fix`",
		"`auto-review`",
		"`auto-sync`",
		"`auto-canary`",
		"`auto-idea`",
	))
	body = injectAfterFirstHeading(body, invoNote)

	frontmatter := strings.TrimSpace(fmt.Sprintf(`---
name: auto
description: >
  Autopus Codex router skill. Use when the user wants %s or %s workflows such as plan, go, fix, review, sync, canary, and idea.
---`, "`@auto ...`", "`$auto ...`"))
	return frontmatter + "\n\n" + strings.TrimSpace(body) + "\n", nil
}

func (a *Adapter) renderTemplateAsSkill(cfg *config.HarnessConfig, spec standardSkillSpec) (string, error) {
	tmplContent, err := templates.FS.ReadFile(spec.TemplatePath)
	if err != nil {
		return "", fmt.Errorf("codex skill 템플릿 읽기 실패 %s: %w", spec.TemplatePath, err)
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
