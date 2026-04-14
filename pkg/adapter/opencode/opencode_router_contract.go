package opencode

import (
	"fmt"
	"strings"
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

func rewriteOpenCodeRouterBody(body string) string {
	body = normalizeOpenCodeMarkdown(strings.TrimSpace(body))
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

func injectRouterSupportedFlows(body string) string {
	start := strings.Index(body, "지원 서브커맨드:")
	rules := strings.Index(body, "\n\n규칙:")
	section := "지원 서브커맨드:\n" + routerSupportedFlows()
	if start < 0 || rules < 0 || rules <= start {
		return strings.TrimSpace(body) + "\n\n" + section
	}
	return body[:start] + section + body[rules:]
}
