# SPEC-HARNESS-WORKFLOW-RUNTIME-001: 생성 워크플로우 JS를 실제 Claude Code Workflow 런타임 API에 정합

**Status**: implemented
**Created**: 2026-06-22
**Domain**: HARNESS

## 목적

`/auto go --workflow`(route_a)와 `/auto go --team`(route_team)이 의존하는 **생성된 결정적 워크플로우 substrate JS**가 실제 Claude Code Workflow 런타임에서 **실행되지 않는다**. 이번 세션에서 설치본 `.claude/workflows/route_team.workflow.js`를 실제 Workflow 런타임에 디스패치했을 때 launch 단계에서 다음으로 실패했다(ground truth로 취급):

```
Workflow script has a syntax error and was not launched:
SyntaxError: Unexpected keyword 'export'
```

실제 API를 사용한 positive-control 워크플로우(`export const meta = {...}` 순수 리터럴 + top-level 본문 + `phase()`/`log()`)는 정상 launch(0 agents, ~5ms)되어, 계약이 양방향으로 경험적으로 확정되었다.

이 결함은 route_team 한정이 아니라 **substrate 패밀리 결함**이다. 부모 `route_a.workflow.js`도 동일한 `export default async function run()` + `agent.exec(...)` 구조를 가지며 실제 `agent('...')` LLM 호출이 0건이라 동일하게 launch 불가다. 따라서 SPEC-HARNESS-WORKFLOW-001의 "completed" 상태는 hermetic 테스트 + `auto workflow render` dry-run 증거에 의존했을 뿐 **실제 워크플로우 launch는 검증된 적이 없다**(부모 acceptance S15는 operational 오라클로 선언됐으나 실 런타임 launch가 아니었다). 이 SPEC이 route_a·route_team 양쪽의 그 갭을 닫는다.

이 작업은 SPEC-HARNESS-WORKFLOW-001(route_a 기반층)과 SPEC-HARNESS-WORKFLOW-TEAM-001(route_team team substrate)의 후속이다.

## Outcome Boundary

- **Outcome Lock**: 생성된 route_a·route_team 워크플로우 JS가 실제 Workflow 런타임에서 SyntaxError 없이 launch되고 phase를 실행한다. `/auto go --workflow`와 `/auto go --team`이 render/parity 통과에 그치지 않고 런타임에서 실제 동작한다.
- **Mandatory requirements**: 두 생성기를 실제 API에 정합(단일 `export const meta` 순수 리터럴, top-level 본문, 두 번째 `export` 금지, `env(` 금지, `agent.exec(` 금지, prompt-string `agent(prompt, opts)`, 단일 인자 `phase(title)`), per-run 컨텍스트·quality binding을 `args`로 전달, 결정적 게이트를 `agent.exec` 없이 재설계, wrong-API 테스트 갱신, 템플릿·설치 JS 재생성, skill/router/manifest 문서 정정, 양 route launch-contract 오라클 추가.
- **Explicit non-goals**: 결정적 phase SET 변경 금지(route_a 4-phase, route_team 8-phase 불변), quality resolver semantics 변경 금지(`resolveTeamQualityBinding`/`ModelForAgent`/`ResolveEffort`/`ResolveDepth` 불변), 비-claude 동작 변경 금지(regression-0), 정식 릴리스 금지.
- **Completion evidence**: 양 route에 대해 hermetic launch-contract 테스트 green(구체 expected/forbidden substring), 기존 게이트 green(parity·doctor·render·depth·whitelist), 그리고 메인 세션이 생성 JS를 Workflow 툴로 디스패치하여 SyntaxError 없이 launch + phase 실행(operational 증거).

## Requirements

### REQ-001 — 단일 `export const meta` 순수 리터럴로 시작
THE SYSTEM SHALL generate the route_a and route_team workflow JS so that it begins with a single `export const meta = {...}` pure-literal block carrying the required `name` and `description` fields and an optional `phases` array, and contains no second top-level `export` statement.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: `deriveWorkflowJS`/`deriveTeamWorkflowJS`가 JS를 생성할 때.
- Observability: 생성 JS의 첫 비주석 토큰이 `export const meta`이고 `export` 토큰이 정확히 1회 등장함을 launch-contract 테스트(S1/S2)로 확인한다.

### REQ-002 — top-level 본문, entry 함수 금지
THE SYSTEM SHALL emit the workflow logic in the top-level script body after `meta`, and SHALL NOT emit an `export default async function run()` entry function or any function-wrapped run entry.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 meta 이후 본문을 작성할 때.
- Observability: 생성 JS에 `export default`와 `function run(` 부분문자열이 0건임을 S1/S2로 확인한다.

### REQ-003 — `env(`·`agent.exec(` 미참조
THE SYSTEM SHALL NOT reference `env(` or `agent.exec(` in any generated workflow JS, because neither global exists in the real Workflow runtime API.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 게이트 phase 또는 quality preamble을 작성할 때.
- Observability: 생성 route_a·route_team JS에 `env(`와 `agent.exec(` 부분문자열이 0건임을 S1/S2로 확인한다.

### REQ-004 — agent 호출은 args에서 만든 task prompt 문자열 우선
THE SYSTEM SHALL build each agent phase's `agent(prompt, opts)` call so that the first argument is a non-trivial TASK STRING that encodes the phase role together with per-run context interpolated from the runtime `args` global (for example a backtick template literal of the form "As the executor for ${args.spec}, implement task ... in ${args.workingDir}" passed as the first argument, with {model, effort} as the second argument), and SHALL NOT use the role-name-only `agent('executor')` form.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: agent-driven phase 본문 생성 시점.
- Observability: agent 호출의 첫 인자가 `args`/`ctx` 토큰을 보간한 비자명 task template literal(따옴표로 감싼 단일 role identifier가 아님)이고 두 번째 인자가 model 키를 포함하는 opts 객체임을 S3로 확인한다.

### REQ-005 — per-run 컨텍스트·quality binding·segment는 `args`로 전달
WHEN the dispatcher launches route_a or route_team on the Workflow runtime, THEN THE SYSTEM SHALL deliver per-run context (target SPEC, working directory, quality binding) through the Workflow `args` input, the route_team JS SHALL read its quality binding from `args.quality` rather than from any environment variable, and the dispatcher SHALL pass the active segment through `args.segment` so the generated JS runs only that segment's phase blocks.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 메인 세션 디스패처가 워크플로우를 segment 단위로 launch할 때.
- Observability: route_team JS가 `args`/`args.quality`/`args.segment`를 읽고 `env('AUTOPUS_WORKFLOW_QUALITY')`를 읽지 않음(S4), agent 호출이 args에서 만든 task 문자열을 받음(S3), segment 가드가 phase를 분리함(S11), 그리고 skill 문서가 args 스키마(`{spec, workingDir, quality, segment}`)를 명세함으로 확인한다.

### REQ-006 — 단일 인자 `phase(title)`, parity 토큰 보존
THE SYSTEM SHALL emit `phase(title)` as a single-argument call and SHALL carry each phase's retry, budget, result-type, model, effort, and depth tokens outside the `phase()` call so the parity gate continues to detect schema/JS drift.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 생성기가 phase 블록을 작성하고 parity 게이트가 토큰을 검사할 때.
- Observability: `phase('id')` 호출이 단일 인자이고, 동일 phase 블록 내에 retry/budget/result-type/model/effort/depth 토큰이 존재하여 parity 게이트가 green임을 S5로 확인한다.

### REQ-007 — 결정적 게이트는 segment 경계 마커 + JS 밖 barrier로 실행
WHERE a phase is the deterministic gate (gate_build_test or release_hygiene), THE SYSTEM SHALL emit that phase as a segment-boundary marker in the JS (a single-argument `phase(id)` call plus `log(...)`, with no gate logic), and the dispatcher SHALL launch the workflow in segments around each gate so that the exit-code verdict from the existing `auto workflow gate` and `auto check --hygiene --arch --quiet --staged` Go bridges acts as a hard barrier between the pre-gate segment and the post-gate segment, with no shell-out primitive embedded in the JS.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: 생성기가 게이트 phase를 segment 경계로 emit하고 디스패처가 segment 사이에서 게이트를 실행할 때.
- Observability: 두 게이트 phase 본문에 `agent.exec(`가 0건이고 segment A 가드의 마지막 `phase(` 호출이 `phase('gate_build_test')`·segment B 가드의 첫 `phase(` 호출이 `phase('annotation')`(route_team)이며, skill/router 문서가 디스패처의 segment-launch + barrier 계약(게이트 verdict가 pass가 아니면 post-gate segment를 launch하지 않음)을 명세함을 S6·S11로 확인한다.

### REQ-008 — wrong-API 테스트 갱신, `.tmpl` 재생성, 설치 표면은 downstream apply
WHEN the generators change to the real API, THEN THE SYSTEM SHALL update the wrong-API tests `pkg/content/workflow_generate_test.go`, `pkg/content/workflow_generate_team_test.go`, and `pkg/content/workflow_parity_team_test.go` to assert the real contract, SHALL regenerate the SoT-derived templates `templates/claude/workflows/route_a.workflow.js.tmpl` and `templates/claude/workflows/route_team.workflow.js.tmpl` through `auto generate-templates` (which writes only the `.tmpl` surface), and SHALL document that refreshing the installed `.claude/workflows/route_*.workflow.js` surface is a downstream `auto update` apply by the claude adapter, not part of `generate-templates`.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 생성기 변경 후 `auto generate-templates`(템플릿 재생성) 실행 시점과 설치 표면 갱신을 위한 `auto update` 시점.
- Observability: 갱신 테스트가 `agent.exec`/`export default`/`env(`를 더 이상 단언하지 않고 green이며, 재생성된 두 `.tmpl` 첫 줄의 GENERATED 경고가 보존되고, 설치 `.claude/*.js`의 구버전 drift가 `auto update` 적용으로 닫힘을 plan T6·T6b로 확인한다.

### REQ-009 — skill/router/manifest 문서를 실제 계약으로 정정
THE SYSTEM SHALL correct `content/skills/harness-workflow.md`, `content/skills/agent-teams.md`, `templates/claude/commands/auto-router.md.tmpl`, and the workflow manifest contract docs so that the documented Workflow API globals, the gate mechanism, and the quality-binding delivery channel match the real runtime contract.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 문서가 API globals·게이트·quality 전달을 기술하는 모든 지점.
- Observability: "Workflow API globals: agent, phase, log, env" 코멘트와 `agent.exec` 게이트 서술과 `AUTOPUS_WORKFLOW_QUALITY` env 서술이 실제 계약(args 전달, JS 외부 게이트)으로 교체됨을 문서 grep로 확인한다.

### REQ-010 — 기존 invariant 보존
WHEN `auto generate-templates` runs, THEN THE SYSTEM SHALL keep the manifest SoT, the parity gate, the doctor capability gate (MinVersion 2.1.154), the fallback taxonomy, the bounded depth caps (verify_votes ≤ 3, fan_out_cap ≤ 5, retry ≤ 3), the JS-injection whitelist, and the claude-only scoping intact.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 생성·parse·render 경계.
- Observability: `pkg/workflow`·`pkg/content` 기존 단위 테스트가 green이고 cap 초과/비-whitelist 입력이 여전히 fail-closed임을 S8로 확인한다.

### REQ-011 — launch-contract 오라클 fail-closed
WHEN the hermetic launch-contract test runs against the generated route_a and route_team JS, THEN THE SYSTEM SHALL assert the JS begins with `export const meta`, contains no second `export`, contains no `env(` or `agent.exec(`, uses a top-level body, and builds agent prompts as strings from args, failing the test on any violation.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: `[NEW] pkg/content/workflow_launch_contract_test.go` 실행 시점.
- Observability: 위반 substring 주입 시 테스트 fail, 실제 생성 JS에 대해 pass임을 S1/S2/S3로 확인한다.

### REQ-012 — 비-claude `--workflow`/`--team` 회귀 0
WHILE running on the codex, antigravity-cli, or opencode adapter, THE SYSTEM SHALL emit no route_a or route_team workflow JS and SHALL preserve the existing Route A and platform-native `--team` behavior unchanged.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: 비-claude 어댑터 Generate/라우팅 시점.
- Observability: 비-claude 산출물에 `route_a`/`route_team`/`workflow`를 이름에 포함하는 `.js` 0건, 기존 Route A 표면 존재로 S7로 확인한다.

## 생성 파일 상세

| 파일 | 역할 | 종류 |
|------|------|------|
| `pkg/content/workflow_generate.go` | route_a 생성기를 실제 API로 재작성(REQ-001~004,006,007) | 기존 수정 |
| `pkg/content/workflow_generate_team.go` | route_team 생성기를 실제 API로 재작성, `env(` 제거, `args.quality` 사용(REQ-001~007) | 기존 수정 |
| `pkg/content/workflow_parity.go` | parity 토큰을 새 emission 위치(코멘트/const)로 재지정(REQ-006,010) | 기존 수정 |
| `internal/cli/workflow_quality_binding.go` | env 키 전달을 args 페이로드 전달로 전환(REQ-005); binding 계산 불변 | 기존 수정 |
| `pkg/content/workflow_generate_test.go` | route_a wrong-API 단언 교체(REQ-008,011) | 기존 수정 |
| `pkg/content/workflow_generate_team_test.go` | route_team wrong-API 단언 교체(REQ-008,011) | 기존 수정 |
| `pkg/content/workflow_parity_team_test.go` | `teamDriftJS`의 `export default` 픽스처 교체(REQ-008) | 기존 수정 |
| `templates/claude/workflows/route_a.workflow.js.tmpl` | `auto generate-templates`가 SoT에서 재생성하는 embedded 템플릿(edit-forbidden) | generate-templates 재생성 |
| `templates/claude/workflows/route_team.workflow.js.tmpl` | `auto generate-templates`가 SoT에서 재생성하는 embedded 템플릿(edit-forbidden) | generate-templates 재생성 |
| `.claude/workflows/route_a.workflow.js` | claude 어댑터(`claude_workflow.go::workflowFiles`)가 `auto init`/`auto update` 때 `.tmpl`에서 설치하는 표면 | auto update 재설치(generate-templates 범위 밖) |
| `.claude/workflows/route_team.workflow.js` | claude 어댑터가 `auto init`/`auto update` 때 `.tmpl`에서 설치하는 표면 | auto update 재설치(generate-templates 범위 밖) |
| `content/skills/harness-workflow.md` | API globals·게이트·quality 전달 서술 정정(REQ-009) | 기존 수정 |
| `content/skills/agent-teams.md` | substrate 디스패치 서술 정정(REQ-009) | 기존 수정 |
| `templates/claude/commands/auto-router.md.tmpl` | dispatch/게이트/quality 서술 정정(REQ-009) | 기존 수정 |
| `pkg/content/workflow_launch_contract_test.go` | 양 route launch-contract 오라클(REQ-011) | `[NEW]` |

> 두 generated 표면 구분: `auto generate-templates`(`cmd/generate-templates` → `pkg/content/generate.go::generateWorkflowTemplates`)는 `templates/claude/workflows/route_*.workflow.js.tmpl`만 write한다. 설치된 `.claude/workflows/route_*.workflow.js`는 claude 어댑터(`pkg/adapter/claude/claude_workflow.go::workflowFiles`)가 `auto init`/`auto update` 때 그 `.tmpl`에서 `OverwriteAlways`로 재설치한다. autopus-adk는 generators + `.tmpl`을 소유하고, 설치 표면 갱신은 `auto update`가 수행하는 downstream apply다.

## Related SPECs

- **SPEC-HARNESS-WORKFLOW-001** (route_a 기반층, status: completed) — 이 SPEC이 그 render-only 증거의 live-launch 갭을 닫는다.
- **SPEC-HARNESS-WORKFLOW-TEAM-001** (route_team substrate, status: implemented) — 동일 패밀리 결함을 공유; 이 SPEC이 두 route를 함께 정합한다.

이 SPEC은 단일 Primary SPEC이다. sibling SPEC 없음(아래 Traceability와 research.md `## Sibling SPEC Decision` 참조).

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1, T2, T6 | S1, S2 | INV-001 |
| REQ-002 | T1, T2 | S1, S2 | INV-003 |
| REQ-003 | T1, T2 | S1, S2 | INV-002 |
| REQ-004 | T1, T2 | S3 | INV-004 |
| REQ-005 | T2, T4, T7 | S4, S11 | INV-005 |
| REQ-006 | T1, T2, T3 | S5 | INV-006 |
| REQ-007 | T1, T2, T7 | S6, S11 | INV-007 |
| REQ-008 | T5, T6, T6b | S1, S2, S3 | INV-001, INV-002 |
| REQ-009 | T7 | S6, S9 | INV-005, INV-007 |
| REQ-010 | T3, T6 | S8, S12 | INV-008 |
| REQ-011 | T5 | S1, S2, S3 | INV-001, INV-002, INV-003, INV-004 |
| REQ-012 | T6 | S7 | INV-008 |
