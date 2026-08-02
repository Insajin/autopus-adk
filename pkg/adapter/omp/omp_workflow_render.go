package omp

import (
	"fmt"
	"strings"

	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func ompRouterBody(prefix string) string {
	return prefix + strings.Join([]string{
		"`$ARGUMENTS`",
		"",
		"## Router Contract",
		"",
		"Treat the payload above as the complete text supplied after `/auto`.",
		"Route to exactly one matching detail skill from this exact map: " + ompWorkflowTargets() + ".",
		"Preserve `--model <provider/model>` and `--variant <value>` exactly as supplied.",
		"Do not fuzzy-correct an unknown subcommand; report it as unsupported and show the exact map.",
		"This is an emitted routing contract, not an OMP runtime parser or model-quality claim.",
	}, "\n")
}

func ompWorkflowTargets() string {
	targets := make([]string, 0, len(workflowSpecs)-1)
	for _, spec := range workflowSpecs {
		if spec.Name != "auto" {
			targets = append(targets, "`"+spec.Name+"`")
		}
	}
	return strings.Join(targets, ", ")
}

func splitOMPFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content
	}
	body := strings.TrimPrefix(rest[idx+4:], "\n")
	return rest[:idx], body
}

func normalizeOMPWorkflowBody(body string) string {
	body = strings.NewReplacer(
		".codex/", ".claude/",
		".opencode/", ".claude/",
		".gemini/", ".claude/",
		"@auto ", "/auto ",
		"@auto-", "/auto-",
		"## Codex Notes", "## OMP Notes",
		"## Codex 기본 실행 모델", "## OMP 기본 실행 모델",
		"Codex runtime", "OMP runtime",
		"Codex 런타임", "OMP 런타임",
		"Codex에서는", "OMP에서는",
		"Codex의", "OMP의",
		"Codex는", "OMP는",
		"Codex에서", "OMP에서",
		"spawn_agent(...)", "task tool",
		"spawn_agent", "task tool",
		"request_user_input", "user prompt",
	).Replace(body)
	body = pkgcontent.ReplacePlatformReferences(body, "omp")
	body = dedupeOMPReference(body, ".agents/skills/agent-pipeline/SKILL.md")
	return strings.TrimSpace(body)
}

func dedupeOMPReference(body, ref string) string {
	first := strings.Index(body, ref)
	if first < 0 {
		return body
	}
	head := body[:first+len(ref)]
	tail := strings.ReplaceAll(body[first+len(ref):], ref, "the pipeline reference above")
	return head + tail
}

func injectOMPInvocation(body, name string) string {
	subcommand := strings.TrimPrefix(name, "auto-")
	note := fmt.Sprintf("## OMP Invocation\n\n- `/auto %s ...`\n- `/%s ...`\n- Load detail skill `%s` for either entrypoint.", subcommand, name, name)
	return injectOMPAfterHeading(body, note)
}

func injectOMPAfterHeading(body, block string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			out := make([]string, 0, len(lines)+4)
			out = append(out, lines[:i+1]...)
			out = append(out, "", block, "")
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	return block + "\n\n" + body
}

func renderOMPCompactWorkflowSkill(spec workflowSpec) string {
	command, result := ompCompactWorkflowContract(spec.Name)
	sections := []string{
		"# " + spec.Name,
		"",
		"## OMP Invocation",
		"",
		"- `/auto " + strings.TrimPrefix(spec.Name, "auto-") + " ...`",
		"- `/" + spec.Name + " ...`",
		"- Load detail skill `" + spec.Name + "` for either entrypoint.",
	}
	if spec.Name == "auto-test" {
		sections = append(sections, "", "## Context Profile: test", "", "- Required: core,test", "- Optional: signature,learning", "- Excluded: canary")
	}
	sections = append(sections,
		"", "## 실행 계약", "", spec.Description,
		"", "1. 대상 디렉터리와 전달된 인자를 확인합니다.",
		"2. "+command,
		"3. "+result,
	)
	content := strings.Join(sections, "\n")
	return buildMarkdown(ompSkillFrontmatter(spec.Name, spec.Description), normalizeOMPWorkflowBody(content))
}

func ompCompactWorkflowContract(name string) (string, string) {
	switch name {
	case "auto-status":
		return "Bash tool로 `auto status`를 실행해 SPEC lifecycle과 module 상태를 수집합니다.", "SPEC별 status와 다음 실행 가능한 명령을 보고합니다."
	case "auto-update":
		return "Bash tool로 `auto update`를 실행해 현재 repo 또는 meta workspace의 harness surface를 갱신합니다.", "변경된 generated surface와 source-of-truth 경계를 보고합니다."
	case "auto-map":
		return "Workspace tools로 구조, entrypoint, 의존성을 분석합니다.", "핵심 파일과 다음 액션을 요약합니다."
	case "auto-why":
		return "Workspace tools로 Lore, SPEC, ARCHITECTURE를 검색해 요청된 결정의 근거를 추적합니다.", "근거 파일과 결정 이유를 인용합니다."
	case "auto-verify":
		return "Playwright로 대상 frontend의 핵심 flow와 visual state를 검증합니다.", "실행 evidence와 UX regression을 보고합니다."
	case "auto-secure":
		return "요청 범위를 OWASP 관점으로 감사합니다.", "실행 가능한 finding과 검증 공백을 보고합니다."
	case "auto-test":
		return "scenarios.md를 로드하고 선언된 E2E 시나리오를 실행합니다.", "시나리오별 PASS / WARN / FAIL과 evidence를 보고합니다."
	case "auto-dev":
		return "`auto plan` → `auto go` → `auto sync`를 순차 실행합니다.", "첫 실패 단계에서 멈추고 재개 방법을 보고합니다."
	case "auto-doctor":
		return "Bash tool로 `auto doctor`를 실행해 harness 설치와 platform wiring을 값으로 진단합니다.", "finding별 expected/got과 repair action을 보고합니다."
	case "auto-goal":
		return "현재 OMP 세션의 목표 의도를 확인하고 지원되는 goal surface만 사용합니다.", "미지원 persisted state를 만들지 않고 제약을 명시합니다."
	default:
		command := "Bash tool로 `" + strings.ReplaceAll(strings.TrimPrefix(name, "auto-"), "-", " ") + "` 대신 `auto " + strings.TrimPrefix(name, "auto-") + "`를 실행합니다."
		return command, "PASS / WARN / FAIL 결과와 후속 액션을 요약합니다."
	}
}
