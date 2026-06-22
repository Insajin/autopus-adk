# SPEC-HARNESS-WORKFLOW-FIDELITY-001 수락 기준

오라클 우선. Must 시나리오는 생성 JS 텍스트에 대한 구체 expected/forbidden substring으로 충실도를 검증한다. 파일 존재·heading·exit code·non-empty만으로 Must를 닫지 않는다. S1~S7은 hermetic(실 schema에서 생성 후 JS 텍스트 단언), S8/S9는 회귀/문서 경계, S10은 operational(Completion Debt).

## Oracle Acceptance Notes

Must 시나리오(S1~S7)는 파일 존재, heading, exit code, non-empty output 같은 structural-only 신호만으로 닫지 않고, 생성 route_team JS 텍스트에 대한 concrete expected output(구체 expected/forbidden substring)을 단언한다. 대표 expected value:

- S1: `agentType: 'planner'`, `agentType: 'executor'`, `agentType: 'tester'`, `agentType: 'annotator'` (등록된 subagent type 이름과 byte-match).
- S2: `agentType: 'reviewer'` 와 `agentType: 'security-auditor'` 각각 ≥1회 (review phase 2-role).
- S3: `const PLAN_SCHEMA = {` 정확히 1회 + `tasks`/`id`/`description`/`files` property + planning 캡처 `const plan = await agent(` 의 opts에 `schema: PLAN_SCHEMA`.
- S4: `parallel(` + executor opts `isolation: 'worktree'` + fan-out 개수 `Math.min(` 위 `plan.tasks` length 와 `FANOUT_implementation`(fallback fan_out_cap=5) + executor 프롬프트의 `.id`/`.description`/`.files` 보간; index-only `Execute executor agent (fan-out ${i})` skeleton 부재.
- S5: `const SEGMENT = (args && args.segment) || 'A'` 1회 + segment A 마지막 `phase('gate_build_test')` + segment B 첫 `phase('annotation')`.
- S6: fan_out_cap=6 schema 거부(위반 필드 명명) + unsafe phase-id 거부; planner task description 은 생성 텍스트에 generation-time 미보간(런타임 `plan.tasks` 참조로만 소비).
- S7: 두 생성 JS byte-identical + 고정 fan-out 구조 토큰(`const PLAN_SCHEMA`, `parallel(`, `isolation: 'worktree'`, `Math.min(`) 존재; 첫 줄 GENERATED / DO NOT EDIT 경고 보존.

수치 tolerance가 필요한 시나리오는 없다(이 SPEC의 오라클은 생성 JS 텍스트의 정확한 expected substring 동등 비교). structural-only 신호(heading, exit code, non-empty output)는 보조이며 Must를 닫지 못한다.

## Test Scenarios

### S1: phase별 agentType + task-focused 프롬프트가 emit된다 (Must oracle)
Given route_team 실 schema(`content/workflows/route_team.schema.json`)에서 `deriveTeamWorkflowJS`로 JS를 생성한다.
When agent-driven phase(planning, test_scaffold, implementation, annotation, testing, review) 블록을 검사한다.
Then planning 블록은 `agentType: 'planner'`를, test_scaffold와 testing 블록은 `agentType: 'tester'`를, implementation 블록은 `agentType: 'executor'`를, annotation 블록은 `agentType: 'annotator'`를 포함한다.
And planning 프롬프트는 task assignment 산출 지시(예: `task assignment`)와 `${ctx.spec}`/`${ctx.workingDir}` 보간을 포함한다.
And 어떤 agent-driven 블록도 bare `agent(\`Execute planner agent for spec\`)` 같은 skeleton 프롬프트(role + spec id만)를 단독으로 사용하지 않는다.

### S2: review phase가 reviewer와 security-auditor 두 agentType를 디스패치한다 (Must oracle)
Given 생성된 route_team JS의 review phase 블록을 검사한다.
When review 블록의 `agent(...)` 호출들을 본다.
Then verify-vote 루프 호출은 `agentType: 'reviewer'`를 포함한다.
And 보안 감사 호출은 `agentType: 'security-auditor'`를 포함한다.
And review 블록에 `agentType: 'reviewer'`와 `agentType: 'security-auditor'`가 각각 최소 1회 등장한다.

### S3: planning이 inline PLAN_SCHEMA로 structured 출력을 캡처한다 (Must oracle)
Given 생성된 route_team JS 전체를 검사한다.
When PLAN_SCHEMA 선언과 planning 캡처를 본다.
Then JS는 `const PLAN_SCHEMA = {`로 시작하는 inline JSON Schema 리터럴을 정확히 1회 선언한다.
And PLAN_SCHEMA 리터럴은 `tasks` array와 그 item의 `id`, `description`, `files` property를 선언한다.
And planning 블록은 planner 결과를 `const plan = await agent(`로 캡처하고 그 opts에 `schema: PLAN_SCHEMA`를 포함한다.

### S4: executor fan-out이 plan.tasks 위로 threading되고 parallel + worktree로 실행된다 (Must oracle)
Given 생성된 route_team JS의 implementation phase 블록을 검사한다.
When fan-out 디스패치 구조를 본다.
Then implementation 블록은 `parallel(`를 포함한다.
And executor `agent(...)` opts는 `agentType: 'executor'`와 `isolation: 'worktree'`를 포함한다.
And fan-out 개수는 `Math.min(` 표현으로 `plan.tasks` length와 `FANOUT_implementation`(fallback fan_out_cap=5)의 최소값으로 산출된다.
And executor 프롬프트는 task의 `.id`, `.description`, `.files`(file ownership)를 보간한다.
And implementation 블록은 index-only로 구분되는 동일 `Execute executor agent (fan-out ${i})` skeleton 루프를 사용하지 않는다.

### S5: RUNTIME-001 segment 가드와 게이트 marker가 보존된다 (Must oracle)
Given 생성된 route_team JS를 검사한다.
When segment 가드와 게이트 marker를 본다.
Then JS는 `const SEGMENT = (args && args.segment) || 'A'` preamble을 정확히 1회 포함한다.
And segment-A 가드(`if (SEGMENT === 'A')`)의 마지막 `phase('...')` 호출은 `phase('gate_build_test')`이다.
And segment-B 가드(`if (SEGMENT === 'B')`)의 첫 `phase('...')` 호출은 `phase('annotation')`이다.
And planning과 implementation 블록은 segment-A 가드 안에 위치한다.

### S6: bounded caps와 JS-injection whitelist가 fail-closed로 보존된다 (Must oracle)
Given fan_out_cap=6(>5)을 선언하는 schema와 unsafe phase-id를 선언하는 schema를 각각 `ParseSchema`로 파싱한다.
When 두 schema를 파싱한다.
Then fan_out_cap=6 schema는 fan-out cap 위반으로 거부된다(에러 메시지가 위반 필드를 명명).
And unsafe phase-id schema는 JS-injection whitelist 위반으로 거부된다.
And 생성 JS 텍스트에는 planner task description이 generation-time으로 박힌 흔적이 없고(런타임 `plan.tasks` 참조로만 소비), 고정 agentType 리터럴은 등록된 subagent 이름과 일치한다.

### S7: 생성이 byte-stable이고 fan-out 구조가 schema 함수로 고정된다 (Must oracle)
Given 동일 route_team schema에서 `generateWorkflowTemplates`를 두 번 실행해 두 `.tmpl`을 만든다.
When 두 산출 JS를 비교한다.
Then 두 JS는 byte-identical이다.
And 생성 JS의 fan-out 구조 토큰(`const PLAN_SCHEMA`, `parallel(`, `isolation: 'worktree'`, `Math.min(`)이 모두 존재하며 schema 함수로 고정된다(fan-out 개수만 런타임 `plan.tasks` 의존).
And 첫 줄에 GENERATED / DO NOT EDIT 경고가 보존된다.

### S8: route_a 골든과 비-claude 표면이 회귀 0이다 (Must)
Given route_team 생성기 변경 후 `generateWorkflowTemplates`를 실행한다.
When route_a 생성 표면과 비-claude 산출물을 검사한다.
Then 재생성된 `route_a.workflow.js.tmpl`은 committed 골든과 byte-identical이다.
And 비-claude(codex/antigravity-cli/opencode) 산출물에 `route_team` 또는 `workflow`를 이름에 포함하는 `.js` 파일이 0건이다.

### S9: skill/router 문서가 faithful dispatch 계약을 명세한다 (Should)
Given 갱신된 `content/skills/harness-workflow.md`와 `content/skills/agent-teams.md`를 검사한다.
When implementation/review phase 서술을 본다.
Then 문서가 agentType 디스패치, planner structured-output schema, planner→executor task threading, parallel + worktree-isolation 실행을 명세한다.
And 문서가 더 이상 implementation phase를 index-only fan-out skeleton으로 기술하지 않는다.

### S10: 실 multi-agent 종단 실행 (Must, operational — Completion Debt)
Given claude-code Workflow 런타임과 설치된 `route_team.workflow.js`(auto update 적용본).
When 메인 세션이 `/auto go --team`으로 생성 JS를 args(segment 포함)와 함께 디스패치한다.
Then specialized agentType(planner/executor/tester/annotator/reviewer/security-auditor)가 generic Workflow 에이전트가 아닌 전문 시스템 프롬프트로 spawn된다.
And planner의 task 리스트가 executor로 thread되어 executor가 task별 배정+file ownership으로 `parallel` + `isolation:'worktree'` 실행된다.
And subagent는 Workflow 툴 호출 불가이므로 이 operational 확인은 메인 세션 책임이며 hermetic 오라클(S1~S7)로 대체 불가다(실 런타임+LLM 트래픽 의존). 이 시나리오는 research.md `## Completion Debt`로 등재되어 sync completion을 막는다.
