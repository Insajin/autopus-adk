# SPEC-HARNESS-WORKFLOW-TEAM-001: `--team`을 claude-code 결정적 Workflow 기반층으로 대체

**Status**: implemented
**Created**: 2026-06-21
**Domain**: HARNESS-WORKFLOW
**Target module**: autopus-adk (하네스 정본)
**Extends (hard dependency)**: SPEC-HARNESS-WORKFLOW-001 (manifest SoT + parity 게이트 + doctor capability gate + fallback taxonomy + dry-run 렌더 + 결정적 exit-code Gate)

## 목적

SPEC-HARNESS-WORKFLOW-001은 `/auto go --workflow`(비-team) 라우트를 4-phase 결정적 Workflow로 만들었지만, `/auto go --team`(Agent Teams, Route B)은 여전히 메인 세션이 마크다운 절차를 해석해 팀원을 수동 스폰하므로 비결정적이고 단계 누락·표류·재개 불가 문제가 남아 있다. 본 SPEC은 claude-code 플랫폼에 한해 `--team` 사용자 의도("parallel multi-agent")를 결정적 Claude Code Workflow 실행 기반층으로 해소한다. Workflow는 전체 팀 phase 집합(planner → test-scaffold → executor×N → validator(Gate 2) → annotator(Phase 2.5) → tester(Phase 3) → reviewer+security-auditor(Phase 4))을 실제 `agent()` 오케스트레이션으로 인코딩하고, 기존 결정적 게이트(빌드/테스트 exit-code, release hygiene)·RALF 재시도 서킷브레이크·품질 모드(모델 tier + 오케스트레이션 깊이 동시 구동)를 보존·확장한다.

핵심 아키텍처 제약(코드 실사로 고정): 품질→effort/model 해석은 기존 resolver(`internal/cli/effort_resolve.go::ResolveEffort` + `pkg/cost/pricing.go::ModelForAgent`)를 재사용하며 포크하지 않는다. 동시에 `pkg/workflow`는 `pkg/content`/`internal/cli`를 import하지 않는다는 기존 경계(`pkg/workflow/schema.go:5`)를 지킨다. 따라서 품질→(model,effort) 해석은 CLI 디스패치 계층(`internal/cli`)이 수행하고, 결과 값은 `pkg/workflow`에 **데이터로** 주입한다. team phase 집합은 route_a(4-phase, 비-team)를 재정의하지 않기 위해 별도 정본 `[NEW] content/workflows/route_team.{md,schema.json}`로 신설하고, route_a와 동일한 공유 머신러리(schema parse·parity·doctor·gate·fallback·render·drift)를 재사용한다.

## Outcome Boundary

### Outcome Lock (사용자/운영자 가시 결과)
claude-code 플랫폼에서 `auto workflow doctor`가 통과하면 `/auto go --team`이 Agent Teams 대신 결정적 Claude Code Workflow 실행으로 완전히 서빙된다. 이 Workflow는 전체 팀 phase 집합을 실제 `agent()` 오케스트레이션으로 실행하고(planning → test_scaffold → implementation(병렬/worktree) → gate_build_test(Gate 2, exit-code) → annotation → testing → review(reviewer+security-auditor) → release_hygiene), 품질 모드가 모델 tier(기존 `ModelForAgent`)·effort(기존 `ResolveEffort`)·오케스트레이션 깊이(bounded fan-out/vote)를 함께 구동하며, 기존 결정적 게이트와 RALF 재시도(서킷브레이크 cap)를 보존한다. 비-claude 플랫폼과 doctor 실패 또는 사용자 disable 플래그(`--no-workflow`/config `workflow.team_default`)는 회귀 0으로 기존 동작을 유지한다(doctor 실패 → fail-fast Route A subagent pipeline; disable → 기존 Agent Teams; 비-claude → `--team` 불변). `--multi`(risk-tiered provider review)는 실행 기반층과 직교하며 결합되지 않는다.

### Mandatory requirements (Primary 슬라이스 한정)
- 전체 팀 phase parity(8 phase 집합을 실제 `agent()` 오케스트레이션으로 인코딩).
- 품질 모드 → (model, effort, depth) 배선, 기존 resolver 재사용(포크 금지).
- claude-code 전용 스코핑 + doctor-gated 활성화.
- 결정적 exit-code 게이트 보존(Gate 2 + release hygiene).
- parity 게이트를 model/effort/depth 필드로 확장·보존(드리프트 fail-closed).
- 비-claude 회귀 0, `--team` 플래그 의미 보존(claude-code에서 "parallel multi-agent" → Workflow 기반층).
- 품질→depth는 bounded(무한 loop-until-dry 금지, 명시 cap).
- JS-injection 방어: model/effort 문자열은 생성 JS에 보간되므로 whitelist 검증으로 fail-closed.
- `--multi` 직교 보존(quality/substrate와 비결합).

### Explicit non-goals (이번에 하지 않음)
- `--multi`(risk-tiered provider review)의 의미 변경 — 직교 유지.
- 비-claude `--team` 동작 변경 — Agent Teams는 이미 claude-code 전용이라 비-claude에서 폴백 중.
- Route A 기본 subagent pipeline 대체 — 본 SPEC은 `--team` 경로만 다룬다.
- 비-team `/auto go --workflow`의 기존 opt-in route_a 라우트 대체 — route_a phase 집합(phase-id/retry/budget/result-type)은 구조적으로 불변(phase **본문**만 `agent()` 보강).

### Completion evidence
parity 테스트 green(model/effort/depth 포함 드리프트 fail-closed), 품질→effort 오라클이 기존 resolver 출력과 일치, doctor-gate + fallback 테스트, dry-run 렌더가 per-phase agent() model/effort/depth를 노출, 비-claude 회귀 스위트 green, JS-injection 시도가 fail-closed. 추가로 (a) route_team이 generate에서 파생되고 claude adapter가 route_a와 함께 `.claude/workflows/route_team.workflow.js`를 설치, (b) `auto workflow render --route team`이 8-phase team manifest를 선택, (c) 품질 override가 `render --route team --quality ultra` overlay로 per-phase agent() opts(implementation model=claude-opus-4-8/effort=max, review votes=3/synthesis=true)에 도달, (d) 생성 route_team JS가 implementation executor fan-out 루프(fan_out_cap 한정)와 review reviewer+security_auditor agent() 호출을 포함, (e) 디스패치가 binding을 `AUTOPUS_WORKFLOW_QUALITY` env로 직렬화함을 hermetic 오라클로 검증한다.

단, claude-code Workflow 런타임이 설치된 route_team.workflow.js를 실제 실행하여 agent()가 env binding을 상속하고 실 LLM 트래픽으로 executor×N + reviewer + security_auditor를 구동하는 **live end-to-end 실행**은 결정적 hermetic 오라클로 만들 수 없는 operational 잔여이며 `research.md`의 `## Completion Debt`에 REAL debt로 기록한다(sync completion 전 운영 검증 필요).

## Requirements

EARS 형식. 각 요구사항의 정규 문장은 단독 라인이며, Priority(Must/Should/Nice)는 EARS type과 별도 축이다.

### REQ-001 — claude-code 전용 `--team` Workflow 기반층 활성화
WHEN `/auto go --team` runs on the claude-code platform and `auto workflow doctor` reports overall pass and no disable flag is set, THEN THE SYSTEM SHALL serve the run via the deterministic team Workflow substrate instead of Claude Code Agent Teams.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: platform=claude-code AND doctor verdict=pass AND `--no-workflow`/config `workflow.team_default=false` 미설정.
- Observability: 라우터가 방출하는 substrate 선택 로그 라인과 생성된 team workflow JS 디스패치로 확인한다.

### REQ-002 — 전체 팀 phase parity를 `agent()` 오케스트레이션으로 인코딩
THE SYSTEM SHALL define the team workflow as the ordered phases planning, test_scaffold, implementation, gate_build_test, annotation, testing, review, and release_hygiene, where every non-deterministic phase body emits a real `agent()` call and the two deterministic phases (gate_build_test, release_hygiene) cross into Go via `agent.exec`.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: team workflow 정의·생성 시점.
- Observability: dry-run 렌더의 phase_order 8개 순서와 생성 JS의 phase별 `agent(`/`agent.exec(` 호출 종류로 확인한다.
- Note: Gate 2 validator 역할은 결정적 gate_build_test phase(`agent.exec`)로 흡수되며 별도 LLM validator agent는 존재하지 않는다(REQ-005). Outcome Lock 다이어그램의 validator 슬롯은 이 결정적 게이트를 가리킨다.

### REQ-003 — 품질 모드 → model·effort는 기존 resolver 재사용
WHEN the team workflow dispatch resolves per-phase model and effort for a quality mode, THEN THE SYSTEM SHALL derive them from the existing `internal/cli/effort_resolve.go::ResolveEffort` and `pkg/cost/pricing.go::ModelForAgent`, and SHALL NOT fork a parallel quality mapping.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 디스패치 시 `--quality`(ultra/balanced) 해석 시점.
- Observability: 해석된 effort/model 값이 동일 입력에 대한 `ResolveEffort`/`ModelForAgent` 출력과 정확히 일치함을 디스패치 테스트로 확인한다.

### REQ-004 — 품질 모드 → bounded 오케스트레이션 깊이
THE SYSTEM SHALL map balanced quality to single-vote verification with synthesis disabled and ultra quality to bounded multi-vote adversarial verification (verify_votes = 3) with synthesis enabled, and SHALL reject any depth value above the hard caps verify_votes <= 3, fan_out_cap <= 5, and retry <= 3 at the parse and dispatch boundary (fail-closed, not silent clamp).
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 디스패치가 품질 모드를 depth profile로 해석할 때.
- Observability: `[NEW] pkg/workflow.ResolveDepth`의 반환 값과 cap 초과 입력 거부를 단위 테스트로 확인한다.

### REQ-005 — 결정적 exit-code 게이트 보존
THE SYSTEM SHALL keep the gate_build_test phase verdict derived from build and test command exit codes via the existing `auto workflow gate` bridge (`verdict_source: exit_code`) and the release_hygiene phase enforced via `auto check --hygiene --arch --quiet --staged`, with no LLM verdict substituting for either.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: team workflow가 gate_build_test 또는 release_hygiene phase에 도달할 때.
- Observability: gate phase의 `verdict_source: exit_code` 선언과 fake `CommandRunner`(build exit=1 주입)로 verdict=fail 재현으로 확인한다.

### REQ-006 — RALF 재시도와 서킷브레이크 상한
THE SYSTEM SHALL allow per-phase retries for the implementation and gate_build_test phases bounded by the schema retry value, and SHALL reject any schema retry value above the circuit-break cap (retry <= 3) so the retry loop cannot run unbounded.
- EARS type: Ubiquitous
- Priority: Should
- Trigger/Condition: 재시도 가능한 phase가 retry 값을 선언할 때.
- Observability: retry > cap 인 schema가 parse 단계에서 fail-closed 되고, cap 이내 retry는 보존됨을 테스트로 확인한다.

### REQ-007 — doctor-gated 활성화와 fail-fast 폴백
WHEN `/auto go --team` is selected on claude-code and the doctor capability gate reports overall fail, THEN THE SYSTEM SHALL classify the failure as fail-fast and fall back to the Route A subagent pipeline without executing any team workflow.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: platform=claude-code AND doctor verdict=fail(required 프리미티브 누락 또는 version < 2.1.154).
- Observability: 라우터의 `fail-fast` fallback 로그 라인과 Route A 진입으로 확인한다.

### REQ-008 — disable escape hatch는 기존 Agent Teams 동작 보존
WHEN the user passes `--no-workflow` or config `workflow.team_default` is false on claude-code, THEN THE SYSTEM SHALL run `/auto go --team` via the current Claude Code Agent Teams behavior unchanged, as a pre-route opt-out that is not a taxonomy failure.
- EARS type: Event-driven
- Priority: Should
- Trigger/Condition: claude-code AND disable 플래그/설정 true.
- Observability: substrate=agent-teams 라우팅과 team workflow 미디스패치로 확인한다.

### REQ-009 — 비-claude `--team` 회귀 0
WHEN the codex, gemini, or opencode adapter renders the `/auto go --team` route, THEN THE SYSTEM SHALL emit no team workflow JS, no team workflow route, and preserve the existing platform-native `--team` semantics unchanged.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 비-claude 어댑터 Generate/라우팅 시점.
- Observability: 비-claude 산출물에 이름이 `route_team`/`workflow`를 포함하는 `.js` 0건, 기존 `--team` 표면 존재로 확인한다.

### REQ-010 — parity 게이트를 model/effort/depth로 확장, fail-closed
IF the derived team workflow JS and `route_team.schema.json` diverge on phase-id, retry, budget, result-type, model, effort, or depth (verify_votes, fan_out_cap, synthesis) sets, OR any schema phase-id is absent as a string token in `route_team.md`, THEN THE SYSTEM SHALL fail generation closed with a non-zero exit code, name the diverging element, and skip writing the JS.
- EARS type: Unwanted behavior
- Priority: Must
- Trigger/Condition: parity 게이트가 확장된 필드 집합의 불일치를 탐지할 때.
- Observability: 종료 코드와 stderr의 diverging element 이름(예: `planning.model`)으로 확인한다.

### REQ-011 — model/effort JS-injection whitelist 검증
WHEN the schema parser reads a phase model or effort string, THEN THE SYSTEM SHALL accept only whitelisted model identifiers and effort enum values and SHALL fail closed at the parse boundary for any other value, because these strings are interpolated into the generated workflow JS.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: `ParseSchema`가 phase의 model/effort 문자열을 읽을 때.
- Observability: 비-whitelist model/effort(예: `claude-opus-4-8");evil((`)가 parse 에러로 거부되고 JS가 생성되지 않음을 확인한다.

### REQ-012 — dry-run 렌더가 model/effort/depth 노출
WHEN `auto workflow render --dry-run` runs against the team manifest, THEN THE SYSTEM SHALL surface each phase's resolved model, effort, and depth alongside the existing phase order, gate verdict source, and prompt-manifest hash.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: team manifest 대상 render 실행 시점.
- Observability: stdout `DryRunReport`의 per-phase model/effort/depth 필드와 골든 비교로 확인한다.

### REQ-013 — `--multi`는 실행 기반층과 직교
THE SYSTEM SHALL keep `--multi` as risk-tiered provider review that auto-engages only at high or critical risk tier within the review phase, independent of quality mode and execution substrate, and SHALL NOT couple `--multi` to the workflow substrate selection.
- EARS type: Ubiquitous
- Priority: Should
- Trigger/Condition: `--team`과 `--multi`가 함께 지정될 때.
- Observability: `--team --multi`가 substrate 선택을 바꾸지 않고 provider review가 review phase에서만 risk tier 기준으로 활성화됨을 확인한다.

### REQ-014 — prompt-layer manifest 계약 분류
THE SYSTEM SHALL classify the team workflow prompt context as stable (phase structure, schema model/effort/depth defaults), snapshot (doctor capability report, version pin, manifest content hashes), and ephemeral (per-run resolved quality override, run id, complexity, multi risk tier), so that the deterministic prompt-manifest hash via `pkg/workflow/render.go::PromptManifestHash` excludes ephemeral context and stays stable across per-run quality changes.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: render가 prompt 레이어를 prompt-manifest 해시로 접을 때.
- Observability: per-run quality 변경이 해시를 바꾸지 않고 schema 구조 변경이 해시를 바꿈을 골든 테스트로 확인한다.

### REQ-015 — 품질 override가 생성 route_team agent() opts에 바인딩
WHEN the team workflow dispatch resolves a per-run quality override, THEN THE SYSTEM SHALL construct a per-phase quality binding from the existing resolvers (`ResolveEffort`, `ModelForAgent`, `ResolveDepth`) keyed by a phase-id-to-agent-role map, serialize it into the `AUTOPUS_WORKFLOW_QUALITY` environment so the generated route_team agent() opts resolve to the override value with the schema baseline literal as the fallback, and surface the overlaid per-phase values through `auto workflow render --route team --quality <mode>`.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: 디스패치가 `--quality`(ultra/balanced)를 per-phase binding으로 해석할 때.
- Observability: `render --route team --quality ultra`의 DryRunReport per-phase 필드가 override 값(implementation model=claude-opus-4-8/effort=max, review verify_votes=3/synthesis=true)을 보이고, 생성 route_team JS의 agent() opts가 `RT.<phase>` runtime binding을 schema baseline 리터럴 fallback과 함께 참조하며, 디스패치가 binding을 `AUTOPUS_WORKFLOW_QUALITY` env JSON으로 직렬화함으로 확인한다.

### REQ-016 — route_team 설치와 render route 선택
THE SYSTEM SHALL derive the route_team JS from its manifest during generate-templates, install it as `.claude/workflows/route_team.workflow.js` through the claude adapter alongside route_a, and let `auto workflow render` select the route_a or route_team manifest via a `--route` flag that defaults to route_a.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: generate-templates·claude adapter Generate·`auto workflow render` 실행 시점.
- Observability: claude adapter Generate 산출물에 `route_a.workflow.js`와 `route_team.workflow.js`가 모두 존재하고, `render --route team`이 8-phase team manifest를, 플래그 미지정 기본값이 4-phase route_a를 선택함으로 확인한다.

### REQ-017 — multi-agent fan-out과 security_auditor 오케스트레이션
THE SYSTEM SHALL emit the route_team implementation phase body as a bounded executor fan-out loop within fan_out_cap and the review phase body with both a reviewer agent() call and a security_auditor agent() call, so that a single-executor render or a render omitting the security_auditor fails the oracle.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: route_team JS 본문 생성 시점.
- Observability: 생성 route_team JS의 implementation 블록이 executor fan-out 루프(`for` + `agent('executor'` + fan_out_cap 참조)를, review 블록이 `agent('reviewer'`와 `agent('security_auditor'` 호출을 모두 포함함으로 확인한다.

## 생성 파일 상세

| 파일/심볼 | 역할 | 상태 |
|-----------|------|------|
| `content/workflows/route_team.md` | team 사람 계약 정본(8 phase/gate 서술) | [NEW] |
| `content/workflows/route_team.schema.json` | team phase/model/effort/depth 정본 | [NEW] |
| `templates/claude/workflows/route_team.workflow.js.tmpl` | manifest 파생 team JS 어댑터(agent() 오케스트레이션) | [NEW] generated |
| `.claude/workflows/route_team.workflow.js` | claude 설치 산출물(직접편집 금지) | [NEW] generated |
| `pkg/workflow/schema.go::PhaseDef` (+ Model/Effort/depth 필드, rawPhase 확장) | phase model/effort/depth parse | 기존(확장) |
| `pkg/workflow/schema_validate.go` (`isSafeAgentModel`/`isSafeEffort`/model whitelist) | JS-injection 방어 검증 | [NEW] |
| `pkg/workflow/depth.go` (`DepthProfile`/`ResolveDepth`/`MaxVerifyVotes`=3/`MaxFanOut`=5/`MaxRetry`=3) | 품질→bounded depth | [NEW] |
| `pkg/workflow/render.go::DryRunReport` (+ per-phase model/effort/depth) | dry-run 노출 확장 | 기존(확장) |
| `pkg/content/workflow_parity.go` | parity 게이트를 model/effort/depth로 확장 | 기존(확장) |
| `pkg/content/workflow_generate.go` | route_team JS 파생 + agent() 본문 방출 | 기존(확장) |
| `pkg/cost/pricing.go::QualityModeToModels` (+ test_scaffold/annotator/security_auditor roles) | team role 모델 매핑 | 기존(확장) |
| `internal/cli/workflow.go` / 신규 dispatch seam | `--team` substrate 라우팅 + 품질 해석 주입 | 기존(확장)+[NEW] |
| `templates/claude/commands/auto-router.md.tmpl` | Route B → workflow substrate 라우팅, Quality Mode Step 2.1, flag table(`--no-workflow`), fallback 로그 | 기존(확장) |
| `content/skills/agent-teams.md` | claude-code workflow 치환 명시 | 기존(확장) |
| `content/skills/harness-workflow.md` | team phase 집합 문서화(claude-scoped) | 기존(확장) |
| `pkg/workflow/doctor.go` / `gate.go` / `fallback.go` / `drift_gate.go` | 공유 머신러리 재사용(무변경) | 기존(재사용) |
| `pkg/workflow/binding.go` (`QualityBinding`/`PhaseBinding`/`OverlayPhases`) | 품질 binding 데이터 타입 + render overlay(internal/cli 미import) | [NEW] |
| `pkg/content/workflow_generate_team.go` | route_team JS 본문(agent fan-out/security_auditor/`RT.<phase>` override 참조) 방출 | [NEW] |
| `internal/cli/workflow_quality_binding.go` (`resolveTeamQualityBinding`, phase-id↔role map, `AUTOPUS_WORKFLOW_QUALITY` 직렬화) | 품질→per-phase binding 해석 + env 직렬화 | [NEW] |
| `internal/cli/workflow_render.go` (+`--route`/`--quality` 플래그, route별 embed 상수 선택) | render route 선택 + binding overlay | 기존(확장) |
| `pkg/adapter/claude/claude_workflow.go::workflowFiles` (+route_team FileMapping) | route_team 설치(`.claude/workflows/route_team.workflow.js`) | 기존(확장) |
| `pkg/content/workflow_generate.go::generateWorkflowTemplates`/`deriveWorkflowJS` (route 파라미터화) | route_a/route_team 양쪽 파생 | 기존(확장) |

## Related SPECs

- Extends: SPEC-HARNESS-WORKFLOW-001 (manifest SoT + parity + doctor + fallback + render + 결정적 gate). 본 SPEC은 그 공유 머신러리를 재사용하고 model/effort/depth로 확장한다.
- Sibling SPEC Decision: **none** — 단일 Primary SPEC이 Outcome Lock을 닫는다. research.md `## Sibling SPEC Decision` 참조.
- 후속(자동 생성 금지): research.md `## Evolution Ideas` 참조.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 claude 전용 substrate 활성화 | T9, T13 | S1, S10 | INV-008, INV-010 |
| REQ-002 전체 팀 phase parity | T5, T6, T7 | S1 | INV-008 |
| REQ-003 품질→model/effort resolver 재사용 | T13 | S2, S3 | INV-001, INV-002 |
| REQ-004 품질→bounded depth | T3 | S4 | INV-003 |
| REQ-005 결정적 exit-code 게이트 보존 | T5, T6 | S11 | INV-004 (gate verdict_source) |
| REQ-006 RALF 재시도 서킷브레이크 | T1, T3 | S13 | INV-009 |
| REQ-007 doctor-gated fail-fast 폴백 | T13 | S7, S8 | INV-005, INV-006 |
| REQ-008 disable escape hatch | T9, T13 | S12 | INV-010 |
| REQ-009 비-claude 회귀 0 | T14 | S10 | INV-010 |
| REQ-010 parity model/effort/depth fail-closed | T7, T12 | S5 | INV-004 |
| REQ-011 JS-injection whitelist | T2 | S6 | INV-007 |
| REQ-012 dry-run model/effort/depth 노출 | T4 | S9 | INV-008 |
| REQ-013 `--multi` 직교 | T9 | S14 | INV-010 |
| REQ-014 prompt-layer manifest 계약 | T4 | S15 | INV-008 (stable/ephemeral) |
| REQ-015 품질 override가 agent() opts에 바인딩 | T15, T16, T18 | S16, S19, S20 | INV-002, INV-003 |
| REQ-016 route_team 설치 + render route 선택 | T16, T17, T18 | S17, S18 | INV-008 |
| REQ-017 multi-agent fan-out + security_auditor | T16 | S19 | INV-003, INV-008 |
