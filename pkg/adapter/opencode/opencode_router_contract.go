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
