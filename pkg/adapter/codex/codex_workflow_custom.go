package codex

import (
	"fmt"
	"strings"
)

type customWorkflowBody struct {
	prompt string
	skill  string
}

func renderCustomWorkflowPrompt(spec workflowSpec) (string, bool) {
	body, ok := customWorkflowBodies(spec)
	if !ok {
		return "", false
	}
	frontmatter := fmt.Sprintf("---\ndescription: %q\n---", spec.Description)
	return frontmatter + "\n\n" + strings.TrimSpace(injectCodexBrandingBlock(body.prompt, false)) + "\n", true
}

func renderCustomWorkflowSkill(spec workflowSpec) (string, bool) {
	body, ok := customWorkflowBodies(spec)
	if !ok {
		return "", false
	}
	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\n---", spec.Name, spec.Description)
	return frontmatter + "\n\n" + strings.TrimSpace(injectCodexBrandingBlock(body.skill, false)) + "\n", true
}

func customWorkflowBodies(spec workflowSpec) (customWorkflowBody, bool) {
	switch spec.Name {
	case "auto-status":
		return cliWorkflowBody(spec.Name, "SPEC Dashboard", spec.Description, "auto status", "draft / approved / implemented / completed 상태를 요약하고 다음 액션을 제안합니다."), true
	case "auto-goal":
		return goalWorkflowBody(spec.Name, spec.Description), true
	case "auto-verify":
		return cliWorkflowBody(spec.Name, "Frontend UX Verification", spec.Description, "auto verify", "Playwright 기반 검증 결과와 자동 수정 가능 여부를 함께 보고합니다."), true
	case "auto-test":
		return cliWorkflowBody(spec.Name, "E2E Scenario Runner", spec.Description, "auto test run", "scenario별 PASS / FAIL 결과를 정리하고 실패 시 다음 복구 액션을 제안합니다."), true
	case "auto-doctor":
		return cliWorkflowBody(spec.Name, "Harness Diagnostics", spec.Description, "auto doctor", "platform wiring, rules, plugins, dependencies 상태를 요약하고 fix 필요 시 명시합니다."), true
	case "auto-map":
		return taskWorkflowBody(spec.Name, "Codebase Structure Analysis", spec.Description, "explorer", "Analyze the requested scope, summarize directory structure, entrypoints, dependencies, and notable files."), true
	case "auto-secure":
		return taskWorkflowBody(spec.Name, "Security Audit", spec.Description, "security-auditor", "Audit the requested scope using OWASP Top 10 categories. Focus on exploitable risks, missing tests, and secrets exposure."), true
	case "auto-why":
		return whyWorkflowBody(spec.Name, spec.Description), true
	case "auto-dev":
		return devWorkflowBody(spec.Name, spec.Description), true
	default:
		return customWorkflowBody{}, false
	}
}

func cliWorkflowBody(name, title, summary, command, result string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — "+title,
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 실행 원칙",
		"",
		"- 이 워크플로우는 `"+command+"` CLI thin wrapper입니다.",
		"- 전달된 인자와 플래그를 그대로 유지합니다.",
		"- Bash tool로 실제 명령을 실행하고 결과를 요약합니다.",
		"",
		"## 실행 명령",
		"",
		"`"+command+"`",
	)

	skill := compose(
		"# "+name+" — "+title,
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto "+strings.TrimPrefix(name, "auto-")+" ...`",
		"- `$"+name+" ...`",
		"- `$auto "+strings.TrimPrefix(name, "auto-")+" ...`",
		"",
		"## 실행 순서",
		"",
		"1. 대상 디렉터리와 전달된 플래그를 확인합니다.",
		"2. Bash tool로 `"+command+"`를 실행합니다.",
		"3. "+result,
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

func goalWorkflowBody(name, summary string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — Codex Goal Wrapper",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 역할",
		"",
		"`auto goal`은 Codex `/goal` thread 기능을 Autopus router에서 부르는 thin wrapper입니다. 별도 ADK persisted state를 만들거나 `.autopus` 파일에 목표 상태를 저장하지 않습니다.",
		"",
		"## 지원 호출",
		"",
		"- `@auto goal` 또는 `@auto goal status`: `get_goal`로 현재 Codex goal 상태를 확인합니다.",
		"- `@auto goal \"<objective>\" [--budget N]`: 활성 goal이 없을 때 `create_goal`로 objective와 optional token budget을 설정합니다.",
		"- `@auto goal complete`: 목표가 실제로 달성되었고 남은 필수 작업이 없을 때만 `update_goal(status=\"complete\")`를 호출합니다.",
		"- `@auto goal blocked`: 같은 blocking condition이 goal turn 기준 3회 이상 반복되고 의미 있는 진행이 불가능할 때만 `update_goal(status=\"blocked\")`를 호출합니다.",
		"- `@auto goal clear|pause|resume`: 현재 Codex tool surface에 대응 tool이 없으면 `/goal clear`, `/goal pause`, `/goal resume` slash command 사용을 안내합니다.",
		"",
		"## 실행 원칙",
		"",
		"- 새 goal을 만들기 전에 가능하면 `get_goal`로 active goal 존재 여부를 먼저 확인합니다.",
		"- active goal이 있으면 덮어쓰려고 하지 말고 현재 objective/status/budget을 보고하고 다음 행동을 제안합니다.",
		"- goal tool이 현재 런타임에 노출되지 않으면 실패로 숨기지 말고 `/goal <objective>` 또는 `/goal status` fallback을 안내합니다.",
		"- 완료/blocked 표시는 Codex goal tool contract를 그대로 따르며, 편의상 premature completion을 만들지 않습니다.",
		"",
		"## 결과 형식",
		"",
		"- `goal_status`: active / complete / blocked / unavailable",
		"- `objective`: 확인된 목표 또는 생성 요청 목표",
		"- `next_required_step`: 이어서 실행할 `@auto ...` 명령 또는 `/goal ...` fallback",
	)

	skill := compose(
		"# "+name+" — Codex Goal Wrapper",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto goal`",
		"- `@auto goal status`",
		"- `@auto goal \"<objective>\" [--budget N]`",
		"- `@auto goal complete`",
		"- `@auto goal blocked`",
		"- `$auto-goal ...`",
		"- `$auto goal ...`",
		"",
		"## Contract",
		"",
		"- `auto goal` is a thin wrapper over the Codex `/goal` thread feature.",
		"- It is not an ADK persisted state and must not write goal state to `.autopus` or project files.",
		"- Prefer Codex goal tools when available: `get_goal`, `create_goal`, and `update_goal`.",
		"- If the goal tools are unavailable, explain the runtime limitation and give the matching `/goal` slash command.",
		"",
		"## Command Mapping",
		"",
		"1. `@auto goal` or `@auto goal status`: call `get_goal` and summarize objective, status, budget, and next step.",
		"2. `@auto goal \"<objective>\" [--budget N]`: call `get_goal` first when possible; if no goal exists, call `create_goal(objective=\"<objective>\", token_budget=N)`.",
		"3. `@auto goal complete`: call `update_goal(status=\"complete\")` only when the objective is actually achieved and no required work remains.",
		"4. `@auto goal blocked`: call `update_goal(status=\"blocked\")` only after the same blocking condition recurs for at least three consecutive goal turns.",
		"5. `@auto goal clear|pause|resume`: do not invent local behavior; use `/goal clear`, `/goal pause`, or `/goal resume` if the runtime supports those slash commands.",
		"",
		"## Handoff",
		"",
		"- When a goal is active, subsequent `@auto plan`, `@auto go`, `@auto dev`, and `@auto sync` work should preserve the objective and report `goal_status` in completion handoff.",
		"- If a requested workflow would conflict with the active goal, report the conflict before starting the workflow.",
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

func taskWorkflowBody(name, title, summary, agentType, message string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — "+title,
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 실행 원칙",
		"",
		"- Codex에서는 `spawn_agent(...)` 기반 subagent-first 로 진행합니다.",
		"- 대상 path가 있으면 해당 범위를, 없으면 현재 프로젝트 루트를 분석합니다.",
		"",
		"## 권장 서브에이전트 호출",
		"",
		"```python",
		"spawn_agent(",
		"    agent_type=\""+agentType+"\",",
		"    message=\""+message+"\",",
		")",
		"```",
	)

	skill := compose(
		"# "+name+" — "+title,
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto "+strings.TrimPrefix(name, "auto-")+" ...`",
		"- `$"+name+" ...`",
		"- `$auto "+strings.TrimPrefix(name, "auto-")+" ...`",
		"",
		"## 실행 순서",
		"",
		"1. 분석 범위를 결정합니다.",
		"2. `spawn_agent(...)`로 `"+agentType+"`를 호출해 결과를 수집합니다.",
		"3. 주요 findings와 다음 액션을 3개 이내로 정리합니다.",
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

func whyWorkflowBody(name, summary string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — Decision Rationale Query",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 실행 원칙",
		"",
		"- path가 주어지면 `auto lore context <path>`를 우선 사용합니다.",
		"- path가 없으면 `ARCHITECTURE.md`, 관련 SPEC, CHANGELOG, Lore context를 근거로 답합니다.",
		"- 근거가 부족하면 한 개의 짧은 질문으로 범위를 좁힙니다.",
	)

	skill := compose(
		"# "+name+" — Decision Rationale Query",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto why ...`",
		"- `$auto-why ...`",
		"- `$auto why ...`",
		"",
		"## 실행 순서",
		"",
		"1. 입력이 path 중심인지 질문 중심인지 구분합니다.",
		"2. path가 있으면 Bash tool로 `auto lore context <path>`를 실행합니다.",
		"3. 추가 근거가 필요하면 관련 SPEC / ARCHITECTURE / CHANGELOG를 읽고 이유를 요약합니다.",
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

func devWorkflowBody(name, summary string) customWorkflowBody {
	prompt := compose(
		"# "+name+" — Full Development Cycle",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## 실행 순서",
		"",
		"1. 기능 설명을 기준으로 `auto-plan`을 먼저 수행합니다.",
		"2. 생성된 SPEC-ID를 기준으로 `auto-go`를 진행합니다.",
		"3. 구현이 끝나면 `auto-sync`를 수행합니다.",
		"4. `--auto`, `--loop`, `--team`, `--multi`, `--quality` 플래그는 가능한 한 하위 단계로 전달합니다.",
	)

	skill := compose(
		"# "+name+" — Full Development Cycle",
		"",
		"## 설명",
		"",
		summary,
		"",
		"## Codex Invocation",
		"",
		"- `@auto dev ...`",
		"- `$auto-dev ...`",
		"- `$auto dev ...`",
		"",
		"## 실행 규칙",
		"",
		"- `dev`는 `plan → go → sync`를 순차 실행하는 orchestration wrapper입니다.",
		"- `--team`은 Codex native `multi_agent` 도구 기반 Lead/Builder/Guardian 팀 프로파일로 하위 `go` 단계에 전달합니다.",
		"- 각 단계가 실패하면 조용히 건너뛰지 말고 실패 지점과 재개 방법을 명시합니다.",
	)

	return customWorkflowBody{prompt: prompt, skill: skill}
}

func compose(lines ...string) string {
	return strings.Join(lines, "\n")
}
