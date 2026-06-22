# SPEC-HARNESS-WORKFLOW-FIDELITY-001 리서치

## 기존 코드 분석

> **As-built note (2026-06-22)**: 이 SPEC은 **구현 완료(status: implemented)** 상태다. 아래 "thin skeleton 실물" 서술은 구현 **이전** baseline(pre-implementation ground truth)을 기록한 것이며, 현재 working tree의 `pkg/content/workflow_generate_team.go`는 이미 faithful dispatch(phase별 agentType + `PLAN_SCHEMA` 캡처 + task-threaded executor fan-out + `parallel()` + `isolation:'worktree'`)로 격상됐다. 즉 이 절은 "왜 이 변경이 필요했는가"의 근거 baseline이지 현재 코드 상태가 아니다. As-built 검증은 `## Self-Verify Summary`(attempt 2, F-001~F-005 closure)와 hermetic 오라클(`TestFidelityContract_RouteTeam`, `TestS1S19_TeamDeterministicGeneration`, `TestLaunchContract_RouteTeam`) green으로 닫혔다.

이 SPEC의 변경 표면은 대부분 검증된 기존 코드다(Reference Discipline 표 참조). 변경 baseline으로 Read/Grep 직접 확인한 ground truth(pre-implementation):

- **현 생성기(thin skeleton 실물)**: `pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS`(L33-93). `writeTeamPhaseBlock`(L98-121)이 default 분기에서 `await agent(\`Execute %s agent for spec ${ctx.spec || ''} in ${ctx.workingDir || ''}\`, %s)`(L117-118)를 emit. `writeTeamFanOutBlock`(L126-134)이 `for (let i = 0; i < FANOUT_...; i++)` 안에서 `agent(\`Execute %s agent (fan-out ${i}) ...\`)`(L131-132)를 emit — index `i`로만 구분, planner 산출 미참조, `parallel`/`isolation` 없음. `writeTeamReviewBlock`(L139-155)이 verify-vote 루프 + `agent(\`Execute security auditor ...\`)`(L150) + synthesis를 emit. `teamAgentOpt`(L171-174)가 `{ model, effort }` opts만 빌드(agentType/schema/isolation 없음).
- **생성기 role 맵(agentType 부재)**: `pkg/content/workflow_generate_team.go::teamPhaseRoles`(L13-20) — 단일 string, `review: "reviewer"`(security_auditor 별도 하드코딩), `test_scaffold: "test_scaffold"`(등록된 agent 아님). 이 role 문자열은 프롬프트 텍스트에만 쓰이고 `agentType` opts로는 전달되지 않는다 → 런타임은 generic Workflow 에이전트를 spawn.
- **CLI binding role 맵(별도 맵)**: `internal/cli/workflow_quality_binding.go::teamPhaseRoles`(L18-25) — string slice, `review: {reviewer, security_auditor}`. 이는 quality binding 계산용(`resolveTeamQualityBinding`, L33-61)이며 generator의 맵과 별개다. **두 맵은 reconcile 필요**: agentType 값은 등록된 subagent type 이름이어야 한다.
- **등록된 subagent type 이름(agentType 값의 권위)**: `.claude/agents/autopus/*.md` frontmatter `name` — `executor`, `planner`, `tester`, `annotator`, `reviewer`, `security-auditor`(하이픈). `test_scaffold`/`security_auditor`(언더스코어) agent 파일은 미존재. 따라서 agentType 값 = `planner`/`tester`/`executor`/`annotator`/`reviewer`/`security-auditor`이고, test_scaffold phase → `tester` agent로 매핑.
- **parity 게이트(추가 토큰 안전성)**: `pkg/content/workflow_parity.go::checkPerPhaseQualityParity`(L101-135)는 `phaseJSBlock`(L140-151, marker=`phase('<id>'`)이 슬라이스한 블록에서 `model=`/`effort=`/`fan_out_cap=`/`verify_votes=`/`synthesis=` 토큰의 **PRESENCE만** 검사한다. 추가되는 `agentType:`/`schema:`/`parallel(`/`isolation:` 토큰은 기존 토큰을 제거하지 않으므로 parity-safe(additive). `extractPhaseIDsFromJS`(L156-180)는 `phase('<id>')`/`{title:'<id>'}` marker로 phase SET을 추출 — 변경 없음.
- **JS-injection whitelist(trust boundary)**: `pkg/workflow/schema.go::ParseSchema`(L57+)의 `isSafePhaseID`/`isSafeAgentModel`/`isSafeEffort`/`isSafeResultType` + `validateDepthCaps`(`pkg/workflow/depth.go`, MaxFanOut=5/MaxVerifyVotes=3/MaxRetry=3, L18-20). model/effort/phase-id/result_type만 generation-time 보간 값. agentType는 generator 고정 리터럴(whitelist 밖), planner task description은 런타임 데이터(생성 텍스트 미보간).
- **depth 해석(불변)**: `pkg/workflow/depth.go::ResolveDepth`(L23-35) — ultra={3,5,true}, default={1,5,false}. quality resolver는 non-goal(불변).
- **segment 가드/게이트 marker(RUNTIME-001, 보존)**: `deriveTeamWorkflowJS`(L67-90)가 `const SEGMENT = (args && args.segment) || 'A'`(L63) + segment A/B 가드로 phase를 분리, gate_build_test/release_hygiene을 `phase(id)+log()` marker로만 emit(L102-109). launch-contract 오라클 `pkg/content/workflow_launch_contract_test.go::assertSegmentGuards`(L61-100)가 segment A 마지막=gate_build_test, segment B 첫=annotation을 단언.
- **두 generated 표면(RUNTIME-001 Finding 3)**: `auto generate-templates`(`cmd/generate-templates` → `pkg/content/generate.go::generateWorkflowTemplates`)는 `templates/claude/workflows/route_team.workflow.js.tmpl`만 write. 설치된 `.claude/workflows/route_team.workflow.js`는 claude 어댑터(`pkg/adapter/claude/claude_workflow.go::workflowFiles`)가 `auto update` 때 재설치하는 downstream apply.
- **skill 문서(현 서술)**: `content/skills/harness-workflow.md`(L157,L161)가 implementation=`agent() — bounded executor fan-out`, review=`reviewer`+`security-auditor` parallel로 이미 서술하나 agentType/planner-schema/task-threading 디스패치 계약은 미명세. args 계약(`{spec, workingDir, quality, segment}`)은 L183-206에 존재.
- **route_a(non-goal 근거)**: `pkg/content/workflow_generate.go::deriveWorkflowJS`(L99+)는 agent 호출이 0건인 log-only 결정적 skeleton(gate/hygiene marker만). team 에이전트가 없어 이 SPEC의 디스패치 충실도 대상이 아니다.

## Outcome Lock

- **User-visible outcome**: `/auto go --team`(claude-code)이 충실한 전문 에이전트 팀을 디스패치한다 — 생성 route_team JS가 phase별 실제 agentType + role/task 프롬프트 + planner→executor task threading을 emit해, SPEC-HARNESS-WORKFLOW-TEAM-001의 default-on Outcome Lock을 Route A subagent 파이프라인 동등-이상 기본값으로 충족한다.
- **Mandatory requirements**: REQ-001~012. 핵심 = phase별 agentType(등록 이름) + review 2-role + planner structured-output schema 캡처 + executor task-threaded fan-out(`parallel` + `isolation:'worktree'` + min(tasks,cap)) + task-focused 프롬프트 + segment/게이트 barrier 보존 + caps/whitelist 보존 + byte-stable + 테스트/`.tmpl`/문서 정합 + fidelity-contract 오라클.
- **Explicit non-goals**: route_a LLM 디스패치(team 에이전트 없음), quality resolver semantics 변경, segmented-dispatch 재설계, route_team phase SET 변경, worktree merge/coordination을 JS로 이전(Go 소유 유지), 비-claude 동작 변경(regression-0), 정식 릴리스, 실 multi-agent real-LLM 종단 실행(operational Completion Debt).
- **Completion evidence**: route_team에 대해 hermetic fidelity-contract 테스트 green(S1~S4, 구체 expected/forbidden substring: `agentType: 'executor'`, `schema: PLAN_SCHEMA`, `parallel(`, `isolation: 'worktree'`, `Math.min(`+`plan.tasks`, reviewer+security-auditor) + byte-stable S7 + segment 보존 S5 + cap/whitelist fail-closed S6 + route_a 골든·비-claude 회귀 0(S8) + 메인 세션 실 런타임 디스패치로 specialized agentType + task-threaded executor 구동(S10, operational Completion Debt).

## Visual Planning Brief

데이터-플로우(planner → schema → task list → parallel executors with isolation)는 plan.md `## Visual Planning Brief` mermaid 다이어그램에 있다. 요지:

1. planning: `const plan = await agent(planPrompt, { agentType:'planner', schema: PLAN_SCHEMA, model, effort })` → `plan.tasks = [{id, description, files}, ...]` (schema-validated).
2. implementation: `cap = Math.min(plan.tasks.length, FANOUT_implementation≤5)` → `parallel(tasks[0..cap].map(t => agent(execPrompt(t.id,t.description,t.files), { agentType:'executor', isolation:'worktree', model, effort })))`.
3. fan-out 구조(PLAN_SCHEMA, parallel/isolation 토큰, Math.min 표현)는 generation-time 고정(byte-stable). fan-out 개수만 런타임 동적.
4. worktree merge/coordination은 Go 런타임/디스패처 소유. JS는 sequencing + dispatch만.
5. gate_build_test marker → 디스패처 exit-code barrier(JS 밖) → verdict=pass면 segment B(annotation → testing → review[reviewer + security-auditor] → release_hygiene).

## 설계 결정

- **왜 agentType opts에 추가만 하고 prompt 형태는 task-focused로 교체하는가?** agentType가 role 시스템 프롬프트(TRUST-5/TDD/OWASP/@AX)를 운반하므로, prompt는 role 선언이 아니라 **task instruction**이 되어야 효과적이다(REQ-005). bare `Execute planner agent`는 agentType 없이는 generic 에이전트를, agentType 있어도 빈약한 task를 준다. 두 변경(agentType + task prompt)은 같은 phase 블록에서 함께 일어난다.
- **왜 planner를 structured-output schema로 캡처하는가(task threading의 핵심)?** 현 skeleton의 근본 결함은 planner 산출이 executor로 thread되지 않는 것이다. 자유 텍스트 산출은 JS가 파싱해 fan-out에 배분할 수 없다. `schema: PLAN_SCHEMA`는 runtime이 검증된 객체(`plan.tasks`)를 반환하게 강제하므로 JS가 결정적으로 `plan.tasks[i]`를 executor에 배분할 수 있다(REQ-003,004). PLAN_SCHEMA는 task id/description/file ownership 3필드 — file ownership이 executor 간 충돌 방지 경계다.
- **왜 fan-out 개수 = min(plan.tasks.length, fan_out_cap)인가?** task가 cap보다 많으면 worktree 동시성 한계(5)를 초과하므로 cap으로 bound. task가 cap보다 적으면 빈 executor를 돌리지 않도록 task 수로 bound. 이는 런타임 동적(plan.tasks 의존)이지만 **구조**(Math.min 표현, parallel 래핑)는 generation-time 고정이라 byte-stable invariant(REQ-008)와 양립한다.
- **왜 parallel + isolation:'worktree'인가?** executor가 동일 파일을 동시 수정하면 충돌한다. `isolation:'worktree'`(공식 API 확인: `AgentInput`의 isolation이 "worktree" 수용)는 각 executor 편집을 별도 git worktree로 격리한다. `parallel(...)`은 동시 실행 barrier. 5-worktree cap은 fan_out_cap≤5로 이미 bound(worktree-safety 룰과 정합). merge/coordination은 Go 런타임 소유(RUNTIME-001 경계) — JS는 디스패치만.
- **왜 route_a는 non-goal인가?** route_a 생성기(`deriveWorkflowJS`)는 agent 호출이 0건인 log-only 결정적 skeleton이다. team 에이전트가 없어 agentType/task threading 대상이 없다. route_a의 LLM 디스패치는 별도 product 결정이며 이 SPEC의 충실도 갭(team 디스패치)과 무관하다.
- **신규 interpolation 표면 trust boundary(REQ-007)**: planner task description은 untrusted LLM 산출이다. 이를 생성 JS 텍스트에 generation-time 보간하면 JS-injection 위험이 생긴다. 설계는 task description을 **런타임 데이터**로만 다룬다 — JS는 `plan.tasks[i].description`를 런타임 에이전트 프롬프트로 전달하고, 생성 소스에는 박지 않는다. 따라서 generation-time JS-injection whitelist(phase-id/model/effort/result_type) 표면은 불변이고, agentType는 generator 고정 리터럴(등록 이름과 byte-match)이라 새 generation-time interpolation을 추가하지 않는다. `schema: PLAN_SCHEMA`의 검증이 planner 산출 형태를 추가로 제약한다(임의 필드 차단).
- **agentType 값 정합 위험**: agentType 문자열이 등록된 subagent type 이름과 다르면(예: `security_auditor` 언더스코어, `test_scaffold`) 런타임이 매칭 실패해 generic 에이전트로 폴백한다. 따라서 생성기 맵 값을 등록 이름(`security-auditor`, `tester`)으로 고정하고 fidelity 오라클이 정확한 문자열을 단언한다(S1/S2).

## Technology Stack Decision

이 SPEC은 기존 autopus-adk Go 모듈을 수정하는 **brownfield** 작업이다(신규 프로젝트/스캐폴드 아님). 기존 manifest major version을 보존하며 migration은 scope 밖이다. 새 런타임 dependency를 추가하지 않는다 — Claude Code Workflow 런타임 API(`agent`/`phase`/`log`/`parallel`/`pipeline`/`args`)는 호스트 런타임이 제공하는 ambient global이며 이 SPEC이 버전을 선택하지 않는다.

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go module `github.com/insajin/autopus-adk` (기존) | 기존 `go.mod` 유지(신규 의존성 0) | repo `go.mod` (compatibility constraint) | 2026-06-22 | 신규 라이브러리 도입(불필요) |
| brownfield | Claude Code Workflow runtime API (호스트 ambient global) | 호스트 제공(이 SPEC이 버전 미선택); doctor MinVersion 2.1.154는 RUNTIME-001 소유 불변 | docs.claude.com Agent SDK reference — `AgentInput.subagent_type` + `isolation:"worktree"` 확인; `pkg/workflow/doctor.go::MinVersion` | 2026-06-22 | — |

> **Workflow 런타임 API 계약(verified, 2026-06-22)**: 생성 JS가 의존하는 실 Claude Code **Workflow** 도구 계약은 `agent(prompt, opts)` (opts: `{agentType, model, schema, isolation:'worktree', label, phase}`) + `parallel(thunks: Array<() => Promise>)` + `phase(title)` + `log()` + `args`이다. opts 키는 **`agentType`(camelCase)** 이고 `parallel`은 **이미 호출된 promise를 spread하는 게 아니라 thunk 배열을 단일 인자로** 받는다. 이 계약은 본 세션에서 **실 Workflow 런타임에 0-서브에이전트 probe를 디스패치해 경험적으로 확인**했다: `parallel([() => Promise.resolve('a'), ...])` → `["a","b","c"]` 반환, `parallel([])` → `[]` clean no-op(0 agents·25ms·에러 0). 따라서 생성기는 fan-out을 `executors.push(() => agent(...))` + `parallel(executors)`(배열) 형태로 emit하며, degenerate floor가 빈 `plan.tasks`의 silent no-op을 막는다. (이전 baseline의 `parallel(...executors)` spread 형태는 이 계약과 어긋나 실 런타임에서 crash했을 것 — F-001 인접 correctness fix로 종결.)
>
> agentType/isolation/parallel은 baseline 코드베이스에 미참조였고(`rg` → generator/JS 산출 0건), 이 SPEC이 추가하는 net-new runtime API 표면이다. `agentType` 값(`planner`/`tester`/`executor`/`annotator`/`reviewer`/`security-auditor`)은 등록된 subagent type 레지스트리와 byte-match하는 고정 Go 리터럴이다.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "No `agentType` → spawns the GENERIC Workflow agent, not the specialized planner/executor/... subagent types" | API call shape / agentType per phase | 각 agent-driven phase 블록 opts의 `agentType: '<registered-name>'` | S1 |
| INV-002 | "The review phase emits two roles (reviewer + security-auditor)" | dual-role dispatch | review 블록의 reviewer + security-auditor agentType | S2 |
| INV-003 | "the planning phase runs the planner with a structured-output schema and the workflow captures its result, e.g. const plan = await agent(planPrompt, { agentType:'planner', schema: PLAN_SCHEMA })" | structured-output capture | `const PLAN_SCHEMA` 리터럴 + `const plan = await agent(... schema: PLAN_SCHEMA ...)` | S3 |
| INV-004 | "the implementation phase dispatches executors over plan.tasks, each executor getting a specific task + file ownership ... fan-out count = min(plan.tasks.length, fan_out_cap)" | task-threaded fan-out | implementation 블록의 `Math.min(plan.tasks.length, FANOUT_implementation)` + task id/description/files 보간 | S4 |
| INV-005 | "run via parallel(...) with isolation:'worktree'" | concurrency + isolation | implementation 블록의 `parallel(` + executor `isolation: 'worktree'` | S4 |
| INV-006 | "the per-phase prompt becomes a task-focused instruction (planner: produce the task assignment table; executor: Implement task <id>: <desc>, files: <ownership>)" | prompt enrichment | agent-driven phase 프롬프트 텍스트(role intent + task + context 보간) | S1, S4 |
| INV-007 | "keep RUNTIME-001's args.segment A/B guards + the dispatcher exit-code gate barriers; the planner→executor threading happens within segment A" | segment/gate boundary 보존 | segment A/B 가드 + gate marker + threading 위치 | S5 |
| INV-008 | "Bounded caps (fan_out ≤5, verify_votes ≤3) preserved. JS-injection whitelist (model/effort/phase-id/result_type) preserved ... task descriptions from the planner are runtime data ... not generation-time interpolation" | runtime caps + trust boundary | parse fail-closed + 생성 텍스트 미보간(런타임 plan.tasks 소비) | S6 |
| INV-009 | "how it stays deterministic/byte-stable while being runtime-dynamic in fan-out count" | determinism / byte-stable | byte-identical 생성 + 고정 fan-out 구조 토큰 | S7 |
| INV-010 | "route_a is a deterministic log-only skeleton (no LLM agents) — decide whether route_a is in scope (likely NOT) ... state it as a non-goal" + 비-claude regression-0 | non-goal / regression invariant | route_a 골든 byte-unchanged + 비-claude route_team `.js` 0건 | S8 |

각 oracle acceptance는 구체 expected/forbidden substring 또는 값을 가진다(예: `agentType: 'planner'`/`'executor'`/`'tester'`/`'annotator'`/`'reviewer'`/`'security-auditor'`, `const PLAN_SCHEMA = {`, `schema: PLAN_SCHEMA`, `parallel(`, `isolation: 'worktree'`, `Math.min(` + `plan.tasks`, task `.id`/`.description`/`.files` 보간, byte-identical, fan_out_cap=6 거부, segment A 마지막=gate_build_test·segment B 첫=annotation, route_a 골든 byte-identical). structural-only(heading/파일 존재/exit success)로 Must를 닫지 않는다.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| phase별 agentType 디스패치 | Primary SPEC T1/T2, S1 | covered |
| review 2-role(reviewer + security-auditor) | Primary SPEC T2, S2 | covered |
| planner structured-output schema 캡처 | Primary SPEC T1/T3, S3 | covered |
| executor task-threaded fan-out(plan.tasks, file ownership) | Primary SPEC T3/T4, S4 | covered |
| parallel + isolation:'worktree' | Primary SPEC T4, S4 | covered |
| task-focused 프롬프트 enrichment | Primary SPEC T2/T3, S1/S4 | covered |
| segment 가드 + 게이트 barrier 보존 | Primary SPEC T4, S5 | covered |
| bounded caps + JS-injection whitelist + trust boundary | Primary SPEC T3/T4, S6 | covered |
| byte-stable 생성 vs 런타임 동적 fan-out | Primary SPEC T1/T4, S7 | covered |
| route_a 골든 + 비-claude 회귀 0 | Primary SPEC T6, S8 | covered |
| fidelity-contract 오라클(hermetic) | Primary SPEC T5, S1~S4 | covered |
| skill/router 문서 정정 | Primary SPEC T7, S9 | covered |
| 설치 표면 drift 닫기(auto update) | Primary SPEC T6b | covered |
| 실 multi-agent real-LLM 종단 실행(operational) | Primary SPEC, S10 | completion-debt |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| Operational 실 multi-agent real-LLM 종단 실행 확인 | Outcome Lock completion evidence | claude-code Workflow 런타임이 설치된 route_team.workflow.js를 `/auto go --team`으로 실제 디스패치해, specialized agentType(planner/executor/tester/annotator/reviewer/security-auditor)가 generic 아닌 전문 프롬프트로 spawn되고 planner task 리스트가 executor로 thread되어 task별 배정+file ownership으로 parallel+worktree-isolation 실행됨을 메인 세션이 1회 실증(S10). subagent는 Workflow 툴 호출 불가이므로 메인 세션 책임. GENERATED-JS 충실도(agentType + threading + parallel/isolation present and correct)는 Completion Debt가 아니라 in-scope이며 hermetic 오라클 S1~S4 + byte-stable S7로 닫힌다(deferred 아님). |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| route_a 비-team 라우트에도 LLM agentType 디스패치 도입 | route_a는 log-only 결정적 skeleton(team 에이전트 없음); Outcome Lock은 route_team만 닫음 | 사용자가 명시적으로 route_a LLM 디스패치를 요청 |
| `pipeline(...)` 기반 phase 간 산출 파이프라이닝(planner→executor 외 더 깊은 단계) | 현재 planner→executor 단일 threading으로 충실도 동등-이상 달성; 추가 파이프라인은 별도 가치 | 사용자가 다단계 산출 파이프라이닝을 요청 |
| PLAN_SCHEMA에 dependency/ordering 필드 추가(task 간 의존 그래프) | id/description/files 3필드로 file-ownership 충돌 방지 충분; 의존 그래프는 추가 복잡도 | 사용자가 task 의존 순서 보장을 요청 |
| executor worktree 산출 자동 merge 정책을 JS로 노출 | merge/coordination은 Go 런타임 소유 경계(non-goal); JS는 디스패치만 | 사용자가 JS-소유 merge 정책을 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | route_team 생성 JS 디스패치 충실도(agentType + planner schema + task threading + parallel/isolation + 프롬프트 + 그 오라클/문서)는 단일 cohesive change다. 독립 사용자 결과·별도 repo ownership·migration sequencing·보안/컴플라이언스 경계 분리 사유 없음. Primary SPEC 단독으로 ≤25 태스크(8)·≤40 소스 파일(생성기1+split1+테스트3+오라클1+`.tmpl`1+문서2≈9)이라 분할 임계 미달. | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS`/`writeTeamPhaseBlock`/`writeTeamFanOutBlock`/`writeTeamReviewBlock`/`teamAgentOpt`/`teamPhaseRoles` | existing | Read 확인(L13-174), skeleton agent prompt·index-only fan-out·agentType 부재 실재 |
| `internal/cli/workflow_quality_binding.go::teamPhaseRoles`/`resolveTeamQualityBinding`/`serializeTeamQualityBinding` | existing | Read 확인(L18-71), review={reviewer,security_auditor} slice 맵 실재 |
| `pkg/content/workflow_parity.go::checkPerPhaseQualityParity`/`phaseJSBlock`/`extractPhaseIDsFromJS` | existing | Read 확인(L101-180), 토큰 PRESENCE 검사 → 추가 토큰 parity-safe |
| `pkg/workflow/schema.go::ParseSchema` + `isSafe*` whitelist | existing | Read/Grep 확인(L57+, L76-102), generation-time 보간 표면 = phase-id/model/effort/result_type |
| `pkg/workflow/depth.go::ResolveDepth`/`MaxFanOut`/`MaxVerifyVotes`/`MaxRetry` | existing | Read 확인(L18-35), caps=5/3/3 불변 |
| `pkg/content/workflow_launch_contract_test.go::assertSegmentGuards`/`TestLaunchContract_RouteTeam` | existing | Read 확인(L61-207), segment 가드 + S3 agent-call 단언 실재 |
| `pkg/content/workflow_generate_team_test.go` (skeleton 단언) | existing | Read 확인(L66-78), `agent(\`Execute executor`/`agent(\`Execute security auditor` 단언 실재 → 갱신 대상 |
| `.claude/agents/autopus/{executor,planner,tester,annotator,reviewer,security-auditor}.md` frontmatter name | existing | Grep 확인, 등록 이름=하이픈(security-auditor), test_scaffold/security_auditor agent 미존재 |
| `pkg/adapter/claude/claude_workflow.go::workflowFiles` | existing | RUNTIME-001 research 확인 인용, 설치 표면 downstream apply |
| `content/skills/harness-workflow.md` / `content/skills/agent-teams.md` | existing | Read/Grep 확인(L157/L161/L183-206), 현 서술 정정 대상 |
| `[NEW] pkg/content/workflow_fidelity_contract_test.go::Test...` | [NEW] planned addition | 미존재 — fidelity-contract 오라클(REQ-012) |
| `[NEW] pkg/content/workflow_generate_team_dispatch.go` | [NEW] planned addition | 미존재 — 300줄 한계 근접 시 dispatch 헬퍼 split(조건부) |
| `templates/claude/workflows/route_team.workflow.js.tmpl` | generated surface | generate-templates 재생성 표면(SoT-derived, edit-forbidden) |
| `.claude/workflows/route_team.workflow.js` | generated/installed surface | auto update 재설치(generate-templates 범위 밖) |
| `agentType`/`isolation`/`parallel`/`schema` runtime opts | [NEW] runtime API surface | `rg` 확인 코드베이스 미참조; 공식 docs(AgentInput.subagent_type + isolation "worktree") + 프롬프트 verified capability |

## Reviewer Brief

- **Intended scope**: route_team 생성 JS(`deriveTeamWorkflowJS`)를 thin skeleton에서 faithful dispatch로 격상 — phase별 agentType + planner structured-output schema 캡처 + executor task-threaded fan-out(parallel + isolation:'worktree' + min(tasks,cap)) + task-focused 프롬프트. 그 hermetic 오라클·`.tmpl` 재생성·skill 문서 정정 포함.
- **Explicit non-goals(리뷰어가 새 scope로 확장 금지)**: route_a LLM 디스패치, quality resolver semantics 변경, segmented-dispatch 재설계, route_team phase SET 변경, worktree merge/coordination을 JS로 이전, 비-claude 동작 변경, 정식 릴리스, 실 multi-agent real-LLM 종단 실행(operational Debt).
- **Self-verified**: Traceability Matrix(REQ↔task↔scenario↔INV), Semantic Invariant Inventory(INV-001~010 → oracle), oracle acceptance(S1~S7 구체 substring), existing/[NEW] Reference Discipline(생성기/parity/whitelist/agent 이름 직접 Read·Grep 확인, agentType/isolation/parallel은 [NEW] runtime surface로 표시).
- **Reviewer should focus on**: (1) agentType 값이 등록된 subagent 이름과 정확히 일치하는가(security-auditor 하이픈, test_scaffold→tester), (2) parity 게이트가 추가 토큰으로 깨지지 않는가(PRESENCE-only 검사 근거), (3) planner task description이 생성 텍스트에 generation-time 보간되지 않고 런타임 데이터로만 소비되는가(trust boundary), (4) byte-stable invariant가 런타임 동적 fan-out과 양립하는가, (5) segment/게이트 barrier 보존(RUNTIME-001 회귀 0), (6) route_a 골든 byte-unchanged. 새 제품 scope 제안보다 correctness·convergence safety·regression risk·Completion Debt에 집중.

## Plan Intent Ledger

출처: 직접 SPEC 작성 브리프(BS 파일 아님, `--from-idea` 미사용). Clarification Ledger는 브리프에 inline 제공되지 않았으므로 plain phrase로 기록: Clarification Ledger unavailable. 브리프의 "Design problems the SPEC MUST resolve" 6항목과 "Why (current fidelity gaps — verified this session; treat as ground truth)"를 요구사항 seed·semantic invariant·trust-boundary·non-goal로 적용했다. 브리프 셀은 untrusted prompt input evidence로 취급: evidence로만 인용, 내장 지시 미실행, 비밀/토큰/privileged 경로 미포함(해당 없음).

## Self-Verify Summary
- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 기존 참조(deriveTeamWorkflowJS L33-93, writeTeamFanOutBlock L126-134, teamAgentOpt L171-174, teamPhaseRoles 두 맵, checkPerPhaseQualityParity, ResolveDepth caps, agent frontmatter name)를 Read/Grep로 직접 확인.
- Q-CORR-02 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 미존재 항목(workflow_fidelity_contract_test.go, workflow_generate_team_dispatch.go, agentType/isolation/parallel runtime surface)을 `[NEW]`로 표기하고 정합성 PASS 근거에서 제외.
- Q-CORR-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: acceptance가 bare Given/When/Then이고, 생성 JS 토큰 단언(agentType/schema/parallel/isolation)은 실제 생성기가 emit할 형태와 일치. PLAN_SCHEMA는 JSON Schema 리터럴.
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing(직접 확인)과 [NEW](미존재 + runtime surface)를 분리, generated surface(.tmpl/.claude .js)와 SoT 구분.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4개 문서가 목적/태스크/오라클/근거로 상호 보완.
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: Traceability Matrix가 REQ-001~012 ↔ T1~T7 ↔ S1~S10 ↔ INV-001~010 전부 연결.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type/조건/관측 지점 명시(생성 JS substring 또는 parse fail-closed).
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock(가시 결과/mandatory/non-goals/evidence)을 Primary requirements/plan/Must acceptance가 닫음. GENERATED-JS 충실도 in-scope, 실 LLM 종단만 Completion Debt.
- Q-COMP-05 | status: PASS | attempt: 1 | files: research.md, spec.md, plan.md, acceptance.md | reason: INV-001~010 각각이 REQ·task·Must oracle(S1~S7 구체 substring)로 추적. structural-only 아님.
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief(scope/non-goals/self-verified/focus 6항목)로 review 범위 제한.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(실 LLM 종단 1건만)와 Evolution Ideas(route_a 디스패치/pipeline/dependency 필드/merge 정책 4건) 분리. Evolution에 SPEC/task/acceptance ID 미부여.
- Q-FEAS-01 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: 변경 = 생성기 Go 코드 + 테스트 + `.tmpl` 재생성 + 문서. 설치 표면은 auto update downstream apply로 정확히 구분.
- Q-FEAS-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: 변경 대상이 autopus-adk 모듈 실제 경로(pkg/content, content/skills)에 맞고 generated(.tmpl/.claude .js) vs SoT 구분.
- Q-FEAS-03 | status: PASS | attempt: 1 | files: acceptance.md, plan.md | reason: 오라클이 hermetic Go 테스트(생성 후 JS 텍스트 단언)로 현 저장소에서 실행 가능. operational S10만 런타임 의존(Debt 명시).
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 모호어(should/might/could 등) 없이 SHALL 단정. 
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority는 Must만 사용, EARS type과 별도 축.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: REQ/AC 완결 문장, acceptance는 bare Given/When/Then/And.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: planner task description(untrusted LLM 산출)을 런타임 데이터로만 소비, 생성 텍스트 미보간; JS-injection whitelist 표면 불변; agentType 고정 리터럴. REQ-007 + 설계 결정에 trust boundary 명시.
- Q-SEC-02 | status: N/A | attempt: 1 | files: research.md | reason: 비밀값/토큰/credential/privileged 절대 경로를 다루지 않음. isolation:'worktree'는 git worktree 격리(경로 traversal 아님), agentType는 고정 이름 리터럴.
- Q-SEC-03 | status: N/A | attempt: 1 | files: spec.md | reason: 별도 로그/audit artifact를 만들지 않음. 생성 JS의 log() marker는 RUNTIME-001 소유 불변이며 이 SPEC이 새 retained artifact를 추가하지 않음.
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md | reason: 단일 문제(route_team 생성 JS 디스패치 충실도)와 밀접 변경 대상(생성기 + 그 검증/문서)으로 수렴.
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock 필수 작업(GENERATED-JS 충실도)은 Primary SPEC in-scope, 실 LLM 종단만 Completion Debt(sync 차단). Evolution은 Outcome Lock 밖.
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: sibling 없음(단일 Primary), Sibling SPEC Decision=none 명시.

### As-built F-closure (attempt 2, 2026-06-22)

리뷰 judge(claude)=PASS(critical 0/security 0/major 1) + 후속 reviewer(APPROVE, 0 blocker) + security-auditor(PASS, 0 Critical/High/Medium) 게이트를 거쳐 아래 finding을 종결했다. 코드/테스트/`.tmpl` 모두 working tree에 반영, go build/vet/gofmt clean·`go test -race`(pkg/workflow·pkg/content·internal/cli) green.

- **F-001 (major, feasibility) | CLOSED** | files: pkg/workflow/doctor.go, doctor_test.go, internal/cli/workflow_test.go | `parallel`/`isolation`을 AdvisoryPrimitives → **RequiredPrimitives 승격**. 미지원 런타임은 doctor fail → fail-fast Route A(폴백)로, "launch pass 후 런타임 crash"(Route A보다 나쁨) 비대칭 해소. liveProber는 모든 primitive에 동일 `present` 반환 → 실 경로 불변. 신규 단언 `TestEvaluate_ParallelIsolationAreRequiredGating`·`TestWorkflowDoctor_ParallelUnavailableFailsGate`, advisory 예시는 `budget`으로 이동.
- **F-001 인접 (correctness) | CLOSED** | files: pkg/content/workflow_generate_team.go, workflow_fidelity_contract_test.go | baseline의 `parallel(...executors)`(spread of invoked promises)는 실 Workflow 계약(`parallel(thunks: Array<() => Promise>)`)과 어긋나 crash했을 형태. fan-out을 `push(() => agent(...))` thunk + `parallel(executors)` 배열로 수정, 실 런타임 probe로 경험적 확인(Technology Stack Decision note 참조). 오라클에 `parallel(...` 금지 + `push(() => agent(` 단언 추가.
- **F-002 (minor, completeness) | CLOSED** | files: pkg/content/workflow_generate_team.go, workflow_fidelity_contract_test.go | 빈/실패 `plan.tasks`의 zero-executor silent no-op → **degenerate floor**(executors.length===0 시 전체-SPEC 단일 fallback executor) 추가. 오라클 `length === 0` 단언으로 잠금.
- **F-003 (minor, correctness) | CLOSED** | files: pkg/content/workflow_launch_contract_test.go | launch-contract의 role-only 회귀 가드(`agent('`/`agent("` 부재 + implementation은 `agent(taskPrompt`) 보존·갱신. faithful prompt는 inline template literal 유지라 가드 유효.
- **F-004 (minor, completeness) | CLOSED** | files: pkg/content/workflow_generate_team_test.go | review synthesis/vote 호출 모두 `agentType:'reviewer'`(count==2), audit는 `security-auditor`(count==1) 단언으로 일관성 잠금.
- **F-005 (suggestion) | DEFERRED→Completion Debt** | S10 operational 단계에 parallel/isolation 런타임 honor 확인 포함. F-001 doctor 승격으로 미지원 시 fail-fast 보장됐고, 실 honor 관측은 실 LLM 종단(Completion Debt) 범위. 0-서브에이전트 probe로 parallel 계약은 경험적 확인 완료.
- **codex REVISE(merge) 판정 재조정**: codex의 "agentType 키 미입증" major는 실 Workflow 계약 + 경험적 probe로 종결; "args.quality cap 우회"는 args.quality를 Go 디스패처(bounded ResolveDepth ≤5/3)만 설정하고 런타임 `min(tasks,cap)`이 추가 bound → 신뢰 경계 내. claude judge PASS + reviewer/security gate PASS를 권위로 채택(per-provider authority).

### Live e2e findings (2026-06-22) — args 전파 버그 발견·수정

사용자 승인 후 실 Workflow 런타임(v2.1.174)에 디스패치한 live 검증 결과:

1. **launch 무결성(PASS)**: 재생성 route_team.workflow.js를 실 런타임에 디스패치 — `meta` 파싱·전 phase 본문 정상(SyntaxError 0).
2. **parallel 계약(PASS)**: 0-서브에이전트 probe로 `parallel([() => Promise.resolve(...)])` → 결과 배열, `parallel([])` → clean no-op 경험적 확인.
3. ⭐**args 전파 버그(CRITICAL, 발견→수정→재검증)**: **실 런타임은 `args` 글로벌을 object가 아니라 JSON STRING으로 전달한다**(`typeof args === 'string'` 실측). 따라서 baseline의 `const SEGMENT = (args && args.segment) || 'A'`는 항상 'A'로 폴백하고 `const ctx = args`는 `.spec`이 없어 빈 컨텍스트가 된다 → **segment B가 영영 실행 불가**(segmented dispatch 무력화)·executor가 SPEC 컨텍스트 결여. RUNTIME-001이 "launch PROVEN"이라 한 것은 **args 없이** 돌린 0-agent 실행이라 이 버그가 잠복했다(launch는 됐으나 args는 한 번도 실전 전달된 적 없음 — `[[learning_adversarial_verify_feed_reality]]`). **수정**: 두 생성기 공유 preamble에 `const ARGV = (typeof args === 'string') ? (args ? JSON.parse(args) : {}) : (args || {})` 정규화 추가(route_a + route_team 모두), 이후 `ctx`/`RT`/`SEGMENT`는 ARGV에서 읽음. **재검증**: 수정 전 sentinel `segment:'Z'`는 planner 1개를 잘못 spawn(SEGMENT 버그 'A')했으나, 수정 후 동일 sentinel은 **0 agents**(SEGMENT 정상 'Z') → fix 실증. launch-contract 오라클에 ARGV 정규화 단언 추가. (route_a 표면도 변하므로 golden 재생성; route_a segment dispatch도 동일 버그였으므로 동반 수정 — RUNTIME-001 correctness 후속.)
4. **planner schema 캡처(PASS)**: 실 planner agent(opus, agentType:'planner')에 `schema: PLAN_SCHEMA`로 디스패치 → 검증된 `{tasks:[{id,description,files}]}` 2-task 반환(file ownership greeting.go vs greeting_test.go 비충돌). agentType 해석 + structured-output 캡처 + task threading 연료가 실 런타임에서 동작 실증(1 agent·45.8k tok·13.5s).
5. **executor parallel/worktree fan-out(PASS)**: 실 executor agent 2개를 `parallel([() => agent(prompt, {agentType:'executor', isolation:'worktree'})])`로 디스패치 → 2개 동시 spawn(agentCount=2·72.5k tok·12.2s), 각자 별도 격리 worktree(`.claude/worktrees/wf_<runid>-1`, `-2`)에서 trivial 파일 생성, **main tree 무변경(격리 holds)**. parallel + agentType:'executor' + isolation:'worktree' 실 런타임 실증. 정리(`git worktree remove --force` + 브랜치 삭제)로 baseline 복귀.
6. ⭐⭐**END-TO-END BLOCKER 발견(CRITICAL) → ✅RESOLVED(신규 `auto workflow merge` 단계)**: executor fan-out e2e가 substrate-level 결함을 드러냈다 — **Workflow 런타임 `isolation:'worktree'` worktree(`.claude/worktrees/wf_*`)는 merge되지 않고**(변경 있으면 auto-remove도 안 됨, e2e가 un-merged 잔존 실증), 디스패처 계약에 merge 단계가 없으며, `pkg/pipeline.WorktreeManager`(legacy 별개 메커니즘)는 route_team이 미호출(`grep .claude/worktrees` 소비처 0). →executor 산출 orphan→gate vacuous pass→segment B 빈 tree review = 종단 비기능. **해결**: 신규 결정적 Go 단계 `auto workflow merge --run <runid>`(`pkg/workflow/merge.go`+`merge_copy.go`+`merge_links_{unix,other}.go`, CLI `internal/cli/workflow_merge.go`)가 runID에 속한 `.claude/worktrees/wf_<runid>-*` worktree의 uncommitted 변경을 workingDir로 consolidate(file-ownership 충돌 감지·skip+report)·`git add`·worktree 정리. 디스패처 계약(`harness-workflow.md ### Segmented Dispatch Contract`) 5-step으로 갱신: launch A → **`auto workflow merge`** → `auto workflow gate` → launch B → hygiene. **실 런타임 live 통합 검증**: 2 executor가 격리 worktree에 disjoint 파일 생성 → merge가 둘 다 main tree로 consolidate+stage+worktree 제거(exit 0). ⭐**security-auditor 3-라운드 적대 감사로 실 취약점 4건 발견·수정·재실측**: H-2(과다선택 데이터손실: `.claude/worktrees/` EvalSymlinks 봉쇄로 main/외부 worktree 제외), M-1(심링크 dst 탈출+비원자: `ensureWithin`+temp/rename), H-1(심링크 src exfil: Lstat 거부+O_NOFOLLOW), H-1-RESIDUAL(**하드링크 exfil**, 실측 PRIVATE-KEY 유출 확인→`Nlink>1` 수집스킵+fd 백스톱 이중방어). 최종 감사 **PASS(Critical/High 0)**. 잔여 L-2(특수문자 파일명 무성 누락, 비차단). → FIDELITY-001 Completion Debt "실 multi-agent 종단 실행"의 worktree-merge blocker는 닫힘.

7. ⭐⭐**단일 실 SPEC chained run 종단 SUCCESS(2026-06-22) — Completion Debt 닫힘**: 디스패처(메인 세션)가 throwaway SPEC(독립 2패키지 pkg/alpha+pkg/beta)로 5-step 종단 1회 실행: **segment A**(4 agents=planner+tester+executor×2, planner가 2 disjoint task 산출, executor가 각자 worktree에 alpha.go/beta.go) → **`auto workflow merge`**(`-uall` fix로 새 디렉터리 중첩 파일 alpha.go/beta.go 둘 다 consolidate+stage+worktree제거) → **`auto workflow gate`**(`{verdict:pass, verdict_source:exit_code, build_exit:0, test_exit:0}`) → **segment B**(4 agents=annotator+tester+reviewer+security-auditor, 머지된 코드 위 동작, annotation이 alpha/beta에 @AX 태그) → **`auto check --hygiene --arch --staged`(exit 0)**. ~8 실 agent·~400k tok. 전 단계 연결·runid 전달·gate barrier·segment B 머지코드 인식 실증. throwaway 전량 정리(pkg/alpha,beta 제거·worktree baseline). **2개 실 발견**: (a) ⭐**`-uall` 버그**(1차 chained run이 적발: 기본 porcelain이 새 untracked 디렉터리를 `?? dir/`로 collapse→중첩 파일 누락→executor 산출 유실, fix dcb20b8) (b) **executor file-ownership 중복**(1차 run서 executor -4가 배정 밖 greeting.go 생성→-3과 충돌→merge skip-both→build 불가; 2차 run서 명시적 disjoint task로 회피). (b)는 merge 결함 아닌 executor coordination 성숙 항목 → ✅**수정**: planner 프롬프트를 "isolated-worktree 병렬 실행용 disjoint task 분해 + 상호의존 파일(impl+test)은 한 task로 그룹화" 제약으로 enrich하고 executor 프롬프트에 "배정된 files만 소유·생성" 가드 추가(`workflow_generate_team.go`). **planner-only probe로 실증**: 동일 impl+test SPEC을 새 프롬프트로 plan→taskCount=1·greeting.go와 greeting_test.go가 같은 task(grouped=true), planner가 "두 파일 상호의존→단일 executor 소유 필수"를 명시 추론. 오버랩→conflict→incomplete-merge 근본 원인 제거. **결론: route_team 종단 기능 PROVEN + coordination rough edge 닫힘.** 남은 것은 release 결정(team_default 코드배선 여부·버전 bump→tag→goreleaser→homebrew)뿐.

## Authoring Preflight

`auto spec validate .autopus/specs/SPEC-HARNESS-WORKFLOW-FIDELITY-001 --strict` 최초 실행은 canonical English 섹션 헤더 누락(`## Requirements`/`## Implementation Strategy`/`## Tasks`/`## Test Scenarios`/`## Oracle Acceptance Notes`)과 Must acceptance structural-only 판정으로 6 error 반환. SPEC 문서 헤더를 canonical 영문으로 정정하고 `## Oracle Acceptance Notes`(concrete expected output 신호 + 대표 expected substring)를 추가해 재실행 결과 `SPEC 검증 통과`(EXIT=0). 변경은 SPEC 4개 문서에 한정(소스/`.tmpl`/config 무수정).
- Q-COMP-01 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: canonical English 섹션 헤더(## Requirements/## Implementation Strategy/## Tasks/## Test Scenarios) 정정으로 authoring preflight 섹션 누락 해소.
- Q-COMP-05 | status: PASS | attempt: 2 | files: acceptance.md | reason: ## Oracle Acceptance Notes에 concrete expected output 신호 + 대표 expected substring 추가로 Must structural-only 판정 해소, `auto spec validate --strict` EXIT=0.
