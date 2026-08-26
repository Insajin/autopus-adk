package codex

import "strings"

func cliWorkflowBody(name, title, summary, command, result string) customWorkflowBody {

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

	return customWorkflowBody{skill: skill}
}

func goalWorkflowBody(name, summary string) customWorkflowBody {

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

	return customWorkflowBody{skill: skill}
}

func taskWorkflowBody(name, title, summary, agentType, message string) customWorkflowBody {

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

	return customWorkflowBody{skill: skill}
}

func whyWorkflowBody(name, summary string) customWorkflowBody {

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

	return customWorkflowBody{skill: skill}
}

func devWorkflowBody(name, summary string) customWorkflowBody {

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

	return customWorkflowBody{skill: skill}
}

func compose(lines ...string) string {
	return strings.Join(lines, "\n")
}
