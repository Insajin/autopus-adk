package opencode

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	openCodeRouterContractTemplatePath = "claude/commands/auto-router.md.tmpl"
	specPathResolutionHeading          = "## SPEC Path Resolution"
	subcommandRoutingHeading           = "## Subcommand Routing"
)

func routerDescription() string {
	return "Autopus 명령 라우터 — OpenCode helper 및 workflow 서브커맨드를 해석합니다"
}

func routerSubcommandCount() int {
	return len(workflowSpecs) - 1
}

func routerSupportedFlows() string {
	var builder strings.Builder
	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}
		fmt.Fprintf(&builder, "- `%s`: %s\n", strings.TrimPrefix(spec.Name, "auto-"), spec.Description)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func routerDetailSkills() string {
	names := make([]string, 0, routerSubcommandCount())
	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}
		names = append(names, fmt.Sprintf("`%s`", spec.Name))
	}
	return strings.Join(names, ", ")
}

func routerSupportedSubcommandsInline() string {
	names := make([]string, 0, routerSubcommandCount())
	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}
		names = append(names, fmt.Sprintf("`%s`", strings.TrimPrefix(spec.Name, "auto-")))
	}
	return strings.Join(names, ", ")
}

func thinRouterSkillBody() string {
	sections := []string{
		strings.TrimSpace(skillInvocationNote("auto")),
		"",
		"## Router Contract",
		"",
		"- 먼저 global flags `--auto`, `--loop`, `--multi`, `--quality`, `--team`, `--solo`, `--model`, `--variant` 를 분리합니다.",
		"- 첫 non-flag 토큰을 subcommand로 사용합니다. 토큰이 없으면 사용자 의도를 분류합니다.",
		"- 지원 서브커맨드: " + routerSupportedSubcommandsInline(),
		"- `--model` / `--variant` 값은 이후 단계로 그대로 전달하고 자동으로 덮어쓰지 않습니다.",
		"- `--auto`는 기본 `task(...)` 기반 subagent-first pipeline 진행에 대한 명시적 승인입니다.",
		"- `--auto`가 없고 현재 OpenCode 런타임이 암묵적 `task(...)` 호출을 허용하지 않으면, 조용히 단일 세션으로 폴백하지 말고 서브에이전트 진행 여부 또는 `--solo` 선택을 사용자에게 확인합니다.",
		"- 서브커맨드를 해석한 뒤에는 반드시 대응하는 상세 스킬(" + routerDetailSkills() + ") 중 하나를 로드합니다.",
		"- 지원하지 않는 서브커맨드면 목록을 짧게 안내하고 가장 가까운 워크플로우를 제안합니다.",
		"- 이 스킬은 얇은 라우터입니다. 고정된 서브커맨드는 바로 상세 스킬로 넘깁니다.",
		"",
		"## Context Load",
		"",
		"- 서브커맨드를 처리하기 전에 `ARCHITECTURE.md`, `.autopus/project/product.md`, `.autopus/project/structure.md`, `.autopus/project/tech.md`, `.autopus/project/workspace.md`, `.autopus/project/scenarios.md`, `.autopus/project/canary.md`, `.autopus/context/signatures.md`, `.autopus/learnings/pipeline.jsonl`을 읽어 현재 프로젝트 컨텍스트를 복원합니다.",
		"- 위 문서가 모두 없으면 컨텍스트 부재를 명시하고 `/auto setup`을 권장합니다.",
		"- 서브커맨드가 명확해 보여도 이 로드 단계를 생략하지 않습니다.",
		"",
		"## SPEC Path Resolution",
		"",
		"- SPEC-ID를 받으면 실제 `SPEC_PATH`, `SPEC_DIR`, `TARGET_MODULE`, `WORKING_DIR`를 먼저 해석한 뒤 상세 스킬로 넘깁니다.",
		"- 해석 순서: `.autopus/specs/{SPEC-ID}/spec.md` → `**/.autopus/specs/{SPEC-ID}/spec.md` 재귀 탐색 (`.git`, `node_modules`, `vendor`, `.cache`, `dist` 제외)",
		"- 0개면 사용 가능한 SPEC 목록과 함께 중단하고, 2개 이상이면 duplicate 경로를 보고하고 중단합니다.",
		"- `auto-go`, `auto-review`, `auto-sync` 같은 상세 스킬은 루트 상대경로를 고정 가정하지 말고 해석된 값을 사용해야 합니다.",
		"",
		"## OpenCode Notes",
		"",
		"- `status`, `verify`, `test`, `doctor`는 thin wrapper 성격입니다.",
		"- `map`, `secure`, `why`는 OpenCode native analysis workflow로 처리합니다.",
		"- `dev`는 `plan -> go -> sync` 순서를 유지하며 `--auto`, `--loop`, `--team`, `--multi`, `--quality`, `--model`, `--variant` 를 하위 단계로 전달합니다.",
		"- 고정된 서브커맨드만 필요하면 `/auto-canary`, `/auto-plan`, `/auto-go` 같은 direct alias를 우선 사용하는 편이 더 짧고 저렴합니다.",
	}
	return strings.Join(sections, "\n")
}

func (a *Adapter) renderRouterContractBody(cfg *config.HarnessConfig) (string, error) {
	rendered, err := a.renderWorkflowPrompt(openCodeRouterContractTemplatePath, cfg)
	if err != nil {
		return "", err
	}
	_, body := splitFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		body = rendered
	}
	return strings.TrimSpace(body), nil
}

func rewriteOpenCodeRouterBody(body, contract string) string {
	body = normalizeOpenCodeMarkdown(strings.TrimSpace(body))
	body = strings.NewReplacer(
		"이 문서는 Codex용 canonical router surface 입니다.", "이 문서는 OpenCode용 canonical router surface 입니다.",
		"shared skill과 parity는 유지하되, 의미 해석은 Codex 규약(`task(...)`, `/auto`, `--auto`, `--team`)을 기준으로 하세요.", "shared skill과 parity는 유지하되, 의미 해석은 OpenCode 규약(`task(...)`, `/auto`, `--auto`, `--team`)을 기준으로 하세요.",
		"- Codex 하네스 기본값은 `task(...)` 기반 subagent-first 입니다.", "- OpenCode 하네스 기본값은 `task(...)` 기반 subagent-first 입니다.",
		"- Codex에서 `--auto`가 있으면, 기본 subagent pipeline 진행에 대한 명시적 승인으로 해석합니다.", "- OpenCode에서 `--auto`가 있으면, 기본 subagent pipeline 진행에 대한 명시적 승인으로 해석합니다.",
		"- 단, `--auto`가 없고 현재 Codex 런타임 정책이 암묵적 `task(...)` 호출을 허용하지 않으면 조용히 단일 세션으로 폴백하지 말고, 하네스 기본값과 제약을 명시적으로 설명한 뒤 사용자에게 서브에이전트 진행 여부 또는 `--solo` 선택을 받으세요.", "- 단, `--auto`가 없고 현재 OpenCode 런타임 정책이 암묵적 `task(...)` 호출을 허용하지 않으면 조용히 단일 세션으로 폴백하지 말고, 하네스 기본값과 제약을 명시적으로 설명한 뒤 사용자에게 서브에이전트 진행 여부 또는 `--solo` 선택을 받으세요.",
	).Replace(body)
	body = injectRouterSpecPathResolution(body, contract)
	body = injectRouterSupportedFlows(body)
	body = strings.ReplaceAll(body, "- 가능하면 같은 이름의 상세 스킬/프롬프트(`auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-canary`, `auto-idea`) 의미를 따릅니다.", "- 가능하면 같은 이름의 상세 스킬/프롬프트("+routerDetailSkills()+") 의미를 따릅니다.")
	body = strings.ReplaceAll(body, "- 가능하면 같은 이름의 상세 스킬/프롬프트(`auto-setup`, `auto-plan`, `auto-go`, `auto-fix`, `auto-review`, `auto-sync`, `auto-canary`, `auto-idea`) 의미를 따릅니다.", "- 가능하면 같은 이름의 상세 스킬/프롬프트("+routerDetailSkills()+") 의미를 따릅니다.")
	body = strings.ReplaceAll(body, "위 7개", fmt.Sprintf("위 %d개", routerSubcommandCount()))
	body = strings.ReplaceAll(body, "위 8개", fmt.Sprintf("위 %d개", routerSubcommandCount()))
	if strings.Contains(body, "## OpenCode Helper Notes") {
		return body
	}
	return body + "\n\n## OpenCode Helper Notes\n\n- `status`, `verify`, `test`, `doctor`는 대응하는 `auto` CLI thin wrapper입니다.\n- `map`, `secure`, `why`는 OpenCode native analysis workflow로 동작합니다.\n- 기본 OpenCode 모델은 `" + openCodeDefaultModel + "`로 가정합니다.\n- 사용자 오버라이드: `--model <provider/model>` / reasoning 오버라이드: `--variant <value>`\n- `dev`는 `plan → go → sync` 순서를 유지하며 `--auto`, `--loop`, `--team`, `--multi`, `--quality`, `--model`, `--variant` 플래그를 하위 단계로 전달해야 합니다."
}

func injectRouterSpecPathResolution(body, contract string) string {
	if strings.Contains(body, specPathResolutionHeading) {
		return body
	}
	section := extractRouterSection(contract, specPathResolutionHeading, subcommandRoutingHeading)
	if strings.TrimSpace(section) == "" {
		return body
	}
	start := strings.Index(body, "지원 서브커맨드:")
	if start < 0 {
		return strings.TrimSpace(body) + "\n\n" + section
	}
	return strings.TrimSpace(body[:start]) + "\n\n" + section + "\n\n" + strings.TrimLeft(body[start:], "\n")
}

func extractRouterSection(body, startHeading, endHeading string) string {
	start := strings.Index(body, startHeading)
	if start < 0 {
		return ""
	}
	end := strings.Index(body[start:], endHeading)
	if end < 0 {
		return strings.TrimSpace(body[start:])
	}
	return strings.TrimSpace(body[start : start+end])
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
