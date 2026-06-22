# SPEC-HARNESS-WORKFLOW-FIDELITY-001: route_team 생성 JS를 충실한 전문 에이전트 팀 디스패치로 격상

**Status**: implemented
**Created**: 2026-06-22
**Domain**: HARNESS

## 목적

`/auto go --team`(claude-code)이 의존하는 생성된 `route_team.workflow.js`는 SPEC-HARNESS-WORKFLOW-RUNTIME-001 이후 실제 Workflow 런타임에서 **launch는 되지만**, 에이전트 디스패치가 thin skeleton이라 오늘날 Route A subagent 파이프라인보다 **충실도가 낮다**. 현재 `pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS`는 phase마다 다음을 emit한다(이번 세션 ground truth로 확인):

```
await agent(`Execute planner agent for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}`, { model, effort });
```

세 가지 충실도 갭이 이를 skeleton으로 만든다:
1. **`agentType` 부재** → 전문 subagent type(`planner`/`executor`/`tester`/`annotator`/`reviewer`/`security-auditor`, 각각 TRUST-5 / TDD / OWASP / @AX 시스템 프롬프트 보유)이 아니라 **generic Workflow 에이전트**가 spawn된다.
2. **얇은 프롬프트** → spec id + workingDir만 전달, role task·requirements·plan 없음.
3. **task split·coordination 부재** → executor fan-out 루프가 index `i`로만 구분되는 동일 `Execute executor agent (fan-out ${i})` N회를 돌린다. planner 산출이 executor로 thread되지 않아 executor마다 per-task 배정·file ownership이 없고 중복/충돌한다.

이 SPEC은 생성 JS가 실제 `agentType` + role/task 프롬프트 + planner→executor task threading을 emit하도록 격상해, `/auto go --team`이 SPEC-HARNESS-WORKFLOW-TEAM-001의 Outcome Lock(default-on team substrate)을 **Route A subagent 파이프라인 동등-이상 기본값**으로 충족하게 만든다. SPEC-HARNESS-WORKFLOW-RUNTIME-001(launch 정합)의 후속이다.

## Outcome Boundary

- **Outcome Lock**: route_team 생성 JS가 충실한 전문 에이전트를 디스패치한다 — phase별 실제 `agentType` + role/task 프롬프트 + planner→executor task threading(planner가 structured-output schema로 task 리스트를 산출하고, executor fan-out이 `plan.tasks`를 받아 task별 배정+file ownership으로 `parallel(...)` + `isolation:'worktree'` 실행, fan-out 개수 = `min(plan.tasks.length, fan_out_cap)`). 그 결과 `/auto go --team`이 claude-code에서 Route A subagent 파이프라인 동등-이상 기본값이 된다.
- **Mandatory requirements**: phase→agentType 매핑 emit(REQ-001), review phase 2-role(reviewer + security-auditor, REQ-002), planner structured-output schema 캡처(REQ-003), executor fan-out을 `plan.tasks` 위로 threading + `parallel(...)` + `isolation:'worktree'` + fan-out=min(tasks,cap)(REQ-004), per-phase task-focused 프롬프트 enrichment(REQ-005), RUNTIME-001 segment A/B 가드 + 디스패처 게이트 barrier 보존(REQ-006), bounded caps(fan_out≤5, verify_votes≤3) + JS-injection whitelist 보존 + 신규 interpolation 표면 trust boundary 평가(REQ-007), 결정적/byte-stable 생성(런타임 동적 fan-out과 양립, REQ-008), wrong-API/skeleton 단언 테스트 갱신 + `.tmpl` 재생성 경로 + 설치 표면 downstream apply(REQ-009), skill/router 문서 정정(REQ-010), 비-claude 회귀 0(REQ-011), fidelity-contract 오라클 추가(REQ-012).
- **Explicit non-goals**: route_a의 LLM 에이전트 디스패치(route_a는 log-only 결정적 skeleton, team 에이전트 없음), quality-resolver semantics 변경(`resolveTeamQualityBinding`/`ModelForAgent`/`ResolveEffort`/`ResolveDepth` 불변), segmented-dispatch 재설계(RUNTIME-001 segment A/B + 디스패처 게이트 barrier 그대로), 결정적 phase SET 변경(route_team 8-phase 불변), worktree merge/coordination 로직을 JS로 이전(Go 런타임/디스패처 소유 경계 유지), 비-claude 동작 변경(regression-0), 정식 릴리스, 실 multi-agent real-LLM 종단 실행(operational Completion Debt).

## Requirements

### REQ-001 — phase별 `agentType`를 등록된 subagent type 이름으로 emit
THE SYSTEM SHALL emit each route_team agent-driven phase's `agent(prompt, opts)` call with an `agentType` field in the opts object whose value is the registered subagent type name for that phase per the phase→agentType map (planning→`planner`, test_scaffold→`tester`, implementation→`executor`, annotation→`annotator`, testing→`tester`, review→`reviewer`), so the specialized subagent system prompt applies instead of the generic Workflow agent.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: `deriveTeamWorkflowJS`가 agent-driven phase 블록을 생성할 때.
- Observability: 각 agent-driven phase 블록의 `agent(...)` opts에 `agentType: '<registered-name>'`이 존재하고 그 값이 phase→agentType 매핑과 일치함을 fidelity-contract 오라클(S1)로 확인한다.

### REQ-002 — review phase는 reviewer와 security-auditor 두 agentType를 디스패치
WHERE a phase is the review phase, THE SYSTEM SHALL emit one `agent(...)` call with `agentType: 'reviewer'` for the verify-vote loop and one `agent(...)` call with `agentType: 'security-auditor'`, so the review phase runs both specialized roles rather than a single generic agent.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: 생성기가 review phase 블록(verify-vote + 보안 감사)을 작성할 때.
- Observability: review 블록에 `agentType: 'reviewer'`와 `agentType: 'security-auditor'`가 각각 존재함을 S2로 확인한다.

### REQ-003 — planning phase는 structured-output schema로 task 리스트를 캡처
THE SYSTEM SHALL emit the planning phase so it runs the planner with a `schema` field in the opts object referencing an inline PLAN_SCHEMA constant, and SHALL capture the planner's validated result into a JS variable (the plan) whose `tasks` array drives the implementation phase fan-out.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 planning phase 블록을 작성할 때.
- Observability: 생성 JS에 `const PLAN_SCHEMA = {` inline 선언이 존재하고, planning 블록이 `agent(...)`를 `schema: PLAN_SCHEMA`를 포함하는 opts로 호출하며 그 반환을 `const ... = await agent(`로 캡처함을 S3로 확인한다.

### REQ-004 — executor fan-out을 planner의 task 위로 threading하고 parallel + worktree로 실행
WHEN the implementation phase dispatches executors, THEN THE SYSTEM SHALL iterate over the captured plan's `tasks` array bounded by `min(plan.tasks.length, fan_out_cap)`, SHALL build each executor's prompt from that task's id, description, and file ownership, and SHALL run the executors concurrently via `parallel(...)` with each executor agent carrying `isolation: 'worktree'`.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 생성된 implementation phase 블록이 executor fan-out을 디스패치할 때(런타임).
- Observability: implementation 블록에 `parallel(`, executor `agent(...)` opts의 `agentType: 'executor'` + `isolation: 'worktree'`, fan-out 개수를 `Math.min(plan.tasks.length, FANOUT_implementation)`로 산출하는 표현, executor 프롬프트가 task의 id/description/file ownership 토큰을 보간함을 S4로 확인한다.

### REQ-005 — phase별 프롬프트를 task-focused instruction으로 enrichment
THE SYSTEM SHALL build each agent-driven phase's prompt as a task-focused instruction that encodes the role intent and the per-run context (target SPEC, working directory) and, for the implementation phase, the specific task id, description, and file ownership, replacing the thin "Execute <role> agent for spec ..." skeleton prompt.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 agent-driven phase 프롬프트를 작성할 때.
- Observability: planner 프롬프트가 task assignment 산출 지시를, executor 프롬프트가 task id/description/file ownership을, 그 외 role 프롬프트가 role intent + `${ctx.spec}`/`${ctx.workingDir}` 보간을 포함하고 bare `Execute <role> agent` skeleton 형태가 아님을 S1/S4로 확인한다.

### REQ-006 — RUNTIME-001 segment 가드 + 디스패처 게이트 barrier 보존
THE SYSTEM SHALL keep the RUNTIME-001 `const SEGMENT = (args && args.segment) || 'A'` preamble, the single segment-A guard (ending at the `gate_build_test` boundary marker) and single segment-B guard (starting at `annotation`), and the dispatcher exit-code gate barrier between segments, with the planner→executor threading occurring inside the segment-A implementation phase.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 segment 가드와 phase 블록을 작성할 때.
- Observability: 기존 launch-contract 오라클의 segment 가드 단언(segment A 마지막 phase=`gate_build_test`, segment B 첫 phase=`annotation`)이 여전히 green이고 planner/implementation 블록이 segment A 안에 위치함을 S5로 확인한다.

### REQ-007 — bounded caps·JS-injection whitelist 보존, 신규 interpolation 표면 평가
THE SYSTEM SHALL preserve the bounded depth caps (fan_out_cap ≤ 5, verify_votes ≤ 3, retry ≤ 3) and the schema-parse JS-injection whitelist (phase-id, model, effort, result_type), and SHALL keep planner-produced task descriptions as runtime data evaluated by the agent at run time (not generation-time string interpolation into the JS template), so no untrusted planner output is concatenated into the generated JS source.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성·parse 경계와 런타임 fan-out task 소비 지점.
- Observability: cap 초과/비-whitelist 입력이 여전히 fail-closed이고(S6), 생성 JS 텍스트에 planner task description이 generation-time으로 박히지 않으며 task 데이터가 `plan.tasks[i]` 런타임 참조로만 소비됨을 research.md trust-boundary 분석과 S6로 확인한다.

### REQ-008 — 결정적·byte-stable 생성과 런타임 동적 fan-out 양립
WHEN `auto generate-templates` runs, THEN THE SYSTEM SHALL produce the route_team JS as a pure function of the schema (byte-identical across runs, no timestamps, no randomness), with the fan-out count being runtime-dynamic (`min(plan.tasks.length, fan_out_cap)`) while the emitted fan-out STRUCTURE (loop bound expression, PLAN_SCHEMA literal, parallel/isolation tokens) is fixed at generation time.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 생성기가 두 번 이상 실행될 때.
- Observability: 동일 schema에서 생성 JS가 byte-identical이고(S7), fan-out 구조 토큰이 schema 함수로 고정·fan-out 개수만 런타임 `plan.tasks` 의존임을 S7/S4로 확인한다.

### REQ-009 — wrong-API/skeleton 테스트 갱신, `.tmpl` 재생성, 설치 표면 downstream apply
WHEN the generator changes to faithful dispatch, THEN THE SYSTEM SHALL update `pkg/content/workflow_generate_team_test.go` (and the launch-contract test where it asserts the skeleton agent shape) to assert the faithful contract (agentType, schema capture, parallel/isolation, task-threaded fan-out), SHALL regenerate `templates/claude/workflows/route_team.workflow.js.tmpl` through `auto generate-templates` (which writes only the `.tmpl` surface), and SHALL document that refreshing the installed `.claude/workflows/route_team.workflow.js` surface is a downstream `auto update` apply by the claude adapter.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 생성기 변경 후 `auto generate-templates` 실행 시점과 설치 표면 갱신을 위한 `auto update` 시점.
- Observability: 갱신 테스트가 bare `agent(\`Execute ... agent\`)` skeleton을 더 이상 단언하지 않고 faithful 토큰을 단언하며 green이고, 재생성된 `.tmpl` 첫 줄 GENERATED 경고가 보존되며, 설치 `.claude/*.js` drift가 `auto update`로 닫힘을 plan T6/T6b로 확인한다.

### REQ-010 — skill/router 문서를 faithful dispatch 계약으로 정정
THE SYSTEM SHALL update `content/skills/harness-workflow.md` and `content/skills/agent-teams.md` so the documented implementation/review phase descriptions state the agentType dispatch, the planner→executor task threading, and the parallel/worktree-isolation execution, matching the generated JS contract.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 문서가 implementation/review phase 디스패치를 기술하는 지점.
- Observability: 두 skill 문서가 agentType·planner schema·task threading·parallel+isolation을 명세함을 문서 grep로 확인한다(S9).

### REQ-011 — 비-claude `--team`/`--workflow` 회귀 0
WHILE running on the codex, antigravity-cli, or opencode adapter, THE SYSTEM SHALL emit no route_team workflow JS and SHALL preserve the existing platform-native `--team` behavior and route_a generated surface unchanged.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: 비-claude 어댑터 Generate/라우팅 시점.
- Observability: 비-claude 산출물에 `route_team`/`workflow`를 이름에 포함하는 `.js` 0건, route_a 생성 표면 byte-unchanged(골든 회귀)임을 S8로 확인한다.

### REQ-012 — fidelity-contract 오라클 fail-closed
WHEN the hermetic fidelity-contract test runs against the generated route_team JS, THEN THE SYSTEM SHALL assert the JS dispatches specialized agentType per phase, captures the planner schema, threads executors over the plan's tasks via `parallel(...)` with `isolation: 'worktree'`, and bounds fan-out to `min(tasks, cap)`, failing the test on the bare generic `agent('Execute ... agent')` skeleton or any missing faithful token.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: `[NEW] pkg/content/workflow_fidelity_contract_test.go` 실행 시점.
- Observability: skeleton/누락 fixture 주입 시 fail, 실제 생성 JS에 대해 pass임을 S1~S4로 확인한다.

## 생성 파일 상세

| 파일 | 역할 | 종류 |
|------|------|------|
| `pkg/content/workflow_generate_team.go` | route_team 생성기를 faithful dispatch로 격상: phase별 `agentType`, planner `schema` 캡처, executor `parallel`+`isolation:'worktree'`+task threading, 프롬프트 enrichment(REQ-001~008). 300줄 한계 근접 시 fan-out/review/schema emit 헬퍼를 분리 파일로 split | 기존 수정 |
| `pkg/content/workflow_generate_team_test.go` | skeleton 단언(`agent(\`Execute executor`, fan-out index-only)을 faithful 단언(agentType/schema/parallel/isolation/task-threaded)으로 교체(REQ-009,012) | 기존 수정 |
| `pkg/content/workflow_launch_contract_test.go` | S3 agent-call shape 단언을 faithful agentType/task prompt와 양립하도록 갱신(REQ-009); segment 가드 단언 보존(REQ-006) | 기존 수정 |
| `templates/claude/workflows/route_team.workflow.js.tmpl` | `auto generate-templates`가 SoT에서 재생성하는 embedded 템플릿(edit-forbidden) | generate-templates 재생성 |
| `.claude/workflows/route_team.workflow.js` | claude 어댑터(`claude_workflow.go::workflowFiles`)가 `auto update` 때 `.tmpl`에서 재설치하는 표면 | auto update 재설치(generate-templates 범위 밖) |
| `content/skills/harness-workflow.md` | implementation/review phase의 agentType·planner schema·task threading·parallel/isolation 서술 정정(REQ-010) | 기존 수정 |
| `content/skills/agent-teams.md` | team substrate 디스패치 충실도 서술 정정(REQ-010) | 기존 수정 |
| `pkg/content/workflow_fidelity_contract_test.go` | route_team faithful-dispatch 오라클(REQ-012) | `[NEW]` |

> 두 generated 표면 구분(RUNTIME-001과 동일): `auto generate-templates`(`cmd/generate-templates` → `pkg/content/generate.go::generateWorkflowTemplates`)는 `templates/claude/workflows/route_team.workflow.js.tmpl`만 write한다. 설치된 `.claude/workflows/route_team.workflow.js`는 claude 어댑터(`pkg/adapter/claude/claude_workflow.go::workflowFiles`)가 `auto init`/`auto update` 때 그 `.tmpl`에서 `OverwriteAlways`로 재설치한다. autopus-adk는 generator + `.tmpl`을 소유하고, 설치 표면 갱신은 `auto update`가 수행하는 downstream apply다.

## PLAN_SCHEMA 형태

planner의 structured-output `schema`(REQ-003)는 생성 JS에 inline JSON Schema 리터럴 `const PLAN_SCHEMA`로 emit된다. 형태:

```js
const PLAN_SCHEMA = {
  type: 'object',
  properties: {
    tasks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id:    { type: 'string' },              // task 식별자 (예: "T1")
          description: { type: 'string' },        // executor가 구현할 작업
          files: { type: 'array', items: { type: 'string' } } // file ownership (충돌 방지 경계)
        },
        required: ['id', 'description', 'files']
      }
    }
  },
  required: ['tasks']
};
```

planning 블록은 `const plan = await agent(planPrompt, { agentType: 'planner', schema: PLAN_SCHEMA, model, effort });`로 검증된 객체를 캡처하고, implementation 블록은 `const cap = Math.min((plan && plan.tasks ? plan.tasks.length : 0), FANOUT_implementation);` 위에서 `parallel(...)`로 executor를 task별 디스패치한다. PLAN_SCHEMA 리터럴과 fan-out 구조는 schema 함수로 generation-time 고정(byte-stable, REQ-008)이고 fan-out 개수만 런타임 `plan.tasks` 의존이다.

## Related SPECs

- **SPEC-HARNESS-WORKFLOW-TEAM-001** (route_team team substrate, status: implemented) — 이 SPEC이 그 Outcome Lock의 default-on 충실도 갭(thin skeleton)을 닫는다.
- **SPEC-HARNESS-WORKFLOW-RUNTIME-001** (launch 정합, status: implemented) — 이 SPEC이 그 위에서 디스패치 충실도를 격상한다(launch는 RUNTIME-001이 닫음).
- **SPEC-HARNESS-WORKFLOW-001** (route_a 기반층, status: completed) — route_a는 이 SPEC의 non-goal(team 에이전트 없음).

이 SPEC은 단일 Primary SPEC이다. sibling SPEC 없음(research.md `## Sibling SPEC Decision` 참조).

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1, T2 | S1 | INV-001 |
| REQ-002 | T2 | S2 | INV-002 |
| REQ-003 | T1, T3 | S3 | INV-003 |
| REQ-004 | T3, T4 | S4 | INV-004, INV-005 |
| REQ-005 | T2, T3 | S1, S4 | INV-006 |
| REQ-006 | T4 | S5 | INV-007 |
| REQ-007 | T3, T4 | S6 | INV-008 |
| REQ-008 | T1, T4 | S7 | INV-009 |
| REQ-009 | T5, T6, T6b | S1, S2, S3, S4 | INV-001, INV-003, INV-004 |
| REQ-010 | T7 | S9 | INV-001, INV-004 |
| REQ-011 | T6 | S8 | INV-010 |
| REQ-012 | T5 | S1, S2, S3, S4 | INV-001, INV-002, INV-003, INV-004, INV-005 |
