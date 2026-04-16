package codex

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) renderPluginWorkflowShim(cfg *config.HarnessConfig, spec workflowSpec) (string, error) {
	subcommand := strings.TrimPrefix(spec.Name, "auto-")
	body := compose(
		"# "+spec.Name+" — Thin Alias Shim",
		"",
		"## Codex Invocation",
		"",
		"Use this alias surface through any of these compatible forms:",
		"",
		"- `@auto "+subcommand+" ...` — canonical router when the local Autopus plugin is installed",
		"- `$"+spec.Name+" ...` — plugin-local direct alias shim",
		"- `$auto "+subcommand+" ...` — direct router skill invocation",
		"",
		"This file is not the detailed workflow definition.",
		"Reinterpret the user's latest `$"+spec.Name+" ...` request as `@auto "+subcommand+" ...`, preserve flags exactly, and immediately load skill `auto`.",
		"",
		fmt.Sprintf("**프로젝트**: %s | **모드**: %s", cfg.ProjectName, cfg.Mode),
		"",
		"## Alias Shim Contract",
		"",
		"- Treat this file as a thin alias shim only.",
		"- Immediately load skill `auto` and use it as the canonical router.",
		"- Preserve `--auto`, `--loop`, `--multi`, `--quality`, `--model`, `--variant`, `--team`, `--solo`, and subcommand-specific flags exactly as received.",
		"- Let the router own `Context Load`, `SPEC Path Resolution`, branding, and hand-off to the detailed workflow.",
		"- Do not execute workflow phases directly from this file when a detailed workflow exists.",
		"",
		"## Canonical Reinterpretation",
		"",
		"- Incoming alias: `$"+spec.Name+" <args>`",
		"- Canonical router payload: `@auto "+subcommand+" <args>`",
		"- Required next load: skill `auto`",
		"",
		"## Detailed Workflow Source",
		"",
		"After the router resolves the request, use these detailed sources:",
		"",
		"- `.autopus/plugins/auto/skills/auto/SKILL.md` — plugin-local canonical router surface",
		"- `.agents/skills/"+spec.Name+"/SKILL.md` — repository detailed workflow skill",
		"- `.codex/prompts/"+spec.Name+".md` — repository detailed prompt surface",
		"",
		"The router must load the detailed workflow after context restoration and SPEC path resolution.",
		"",
		"## Handoff Sequence",
		"",
		"1. Reinterpret the alias payload as `@auto "+subcommand+" ...`.",
		"2. Load skill `auto`.",
		"3. Let the router perform `Context Load` and, if relevant, `SPEC Path Resolution`.",
		"4. Let the router load the detailed `"+spec.Name+"` workflow before execution.",
	)
	body = injectCodexBrandingBlock(body, false)

	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\n---", spec.Name, spec.Description)
	return frontmatter + "\n\n" + strings.TrimSpace(body) + "\n", nil
}
