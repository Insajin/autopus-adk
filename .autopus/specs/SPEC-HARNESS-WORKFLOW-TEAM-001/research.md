# SPEC-HARNESS-WORKFLOW-TEAM-001 리서치

## 기존 코드 분석

부모 SPEC-HARNESS-WORKFLOW-001이 구축한 공유 머신러리를 직접 코드 실사로 확인했다(전부 EXISTING, 경로/심볼 검증 완료).

- `pkg/workflow/schema.go` — `PhaseDef{ID,Retry,Budget,ResultType}`(:18-23). `ParseSchema`가 `dec.DisallowUnknownFields()`(:50) 사용 → schema에 새 키(model/effort/depth)를 추가하면 `rawPhase`도 함께 확장하지 않는 한 parse가 깨진다(핵심 구현 제약). `isSafePhaseID`(:86-95)가 `[A-Za-z0-9_-]` 외 문자를 거부(JS-injection 방어 선례). `rawPhase`는 `result_type`/`verdict_source` 양쪽 키를 관용(:33-39). accessor `PhaseIDs/RetrySet/BudgetSet/ResultTypeSet`. 패키지 주석(:5): "This package MUST NOT import pkg/content or internal/cli."
- `pkg/workflow/render.go` — `DryRunReport{PhaseOrder,GateVerdictSource,ManifestPath,SchemaPath,PromptManifestHash,JS}`(:13-19). `Render(s, layers, jsContent, manifestPath, schemaPath)`는 순수 함수(파일 미read). `PromptManifestHash`(:41-60)가 non-ephemeral(stable+snapshot) 레이어만 접고 ephemeral 제외.
- `pkg/workflow/doctor.go` — `MinVersion="2.1.154"`(:7). `RequiredPrimitives=[claude,agent,schema,phase]`(:19), `AdvisoryPrimitives=[parallel,isolation,budget,agent-model-override]`(:22, `agent-model-override`가 이미 예견됨). `EvaluateCapabilities(Prober) CapabilityReport`(:56) Overall pass/fail.
- `pkg/workflow/doctor_version.go` — `versionAtLeast(got,min)`(:11), 빈/비숫자 세그먼트→0 이라 "" < "2.1.154".
- `pkg/workflow/gate.go` — `VerdictSourceExitCode="exit_code"`(:11). `EvaluateGate(ctx,CommandRunner,build,test)`(:39), 둘 다 exit 0→pass. `CommandRunner` 주입 seam(:17-22). `GateResult{Verdict,VerdictSource,BuildExit,TestExit}`.
- `pkg/workflow/fallback.go` — `FallbackClass` {fail-fast,fail-closed,resumable,explicit}. `FailureKind` 매핑: non_claude_platform→fail-fast, doctor_fail→fail-fast, parity_drift→fail-closed, execution_abort→resumable, api_unavailable→explicit. `Classify(k)(class,ok)` unknown→ok=false.
- `pkg/workflow/drift_gate.go` — `GeneratedSurfacePrefixes`, `GeneratedSurfaceExactPaths`, `DefaultSourceLimit=300`, `Hygiene(...)`, `DetectGeneratedDrift`(`path.Clean` 정규화로 traversal 우회 차단).
- `internal/cli/effort_resolve.go` — `ResolveEffort`(:39) 우선순위 flag>env(CLAUDE_CODE_EFFORT_LEVEL)>frontmatter>quality_mode>settings_default. `resolveQualityMode(quality,complexity,model)`(:127). `resolveUltraMode`(:140): opus-4-8/opus-4-7→max, haiku-4-5→stripped, else→high. `resolveBalancedMode`(:172): complexity=high→high, else→medium. `normalizeModelID`(:191): "claude-opus-4-8"→"opus-4-8".
- `internal/cli/effort_types.go` — `EffortValue` {low,medium,high,xhigh,max, stripped=""}. `EffortSourceQualityMode="quality_mode"`. `EffortResult{Effort,Source,Model,Reason}`.
- `pkg/cost/pricing.go` — `ModelForAgent(qualityMode,agentName)`(:72)는 `QualityModeToModels`(:43)를 조회. ultra→전 agent claude-opus-4-8; balanced→planner/architect=claude-opus-4-8, executor/tester/reviewer/validator=claude-sonnet-4-6. 미등록 agent→빈 문자열. test_scaffold/annotator/security_auditor 미등록(rg 확인) → [NEW] role 확장 필요.
- `pkg/cost/estimator.go::NewEstimator(qualityMode)`(:17). `pkg/cost/report.go::ModelForAgent` 호출처(:26).
- `internal/cli/global_flags.go` — `collectGlobalFlags`(:43)가 Quality/Effort/MultiMode 플래그 수집. `validateQualityPreset`(:105)가 autopus.yaml `quality.presets` 검증.
- `internal/cli/workflow.go` — `newWorkflowCmd`가 doctor/gate/render 서브커맨드 등록(:25-27). `internal/cli/workflow_gate.go`, `workflow_render.go` 존재.
- `pkg/content/workflow_generate.go`, `workflow_parity.go`, `workflow.go` — 생성/ parity 로직(존재 확인).
- `pkg/adapter/claude/claude_workflow.go` — claude 어댑터 workflow 쓰기(존재 확인).
- `content/workflows/route_a.{md,schema.json}` — route_a SoT(4 phase: planning/implementation/gate_build_test/release_hygiene, model/effort 키 없음).
- `templates/claude/workflows/route_a.workflow.js.tmpl` — planning/implementation 본문이 `log()`만 호출(agent() 0건, :19-24). gate가 `agent.exec(['auto','workflow','gate'])`(:28), release_hygiene가 `agent.exec(['auto','check','--hygiene','--arch','--quiet','--staged'])`(:37).
- skill SoT(생성된 `.claude/skills/auto/SKILL.md`가 아니라 정본): `templates/claude/commands/auto-router.md.tmpl`(Route B/Route A/Route Deterministic Workflow, Step 2.1 Quality Mode, flag table, fallback 로그 `[workflow] fallback-class=fail-fast reason={doctor_fail|non_claude_platform}`, `--team --multi` 결합 노트), `content/skills/agent-teams.md`(`--team` 활성화/팀 구성/`.members<4`→Route A 폴백), `content/skills/harness-workflow.md`(`--workflow` opt-in, claude-scoped, fallback 표), `content/skills/adaptive-quality.md`(Quality Mode refinement, balanced-only 복잡도 escalation).

## Outcome Lock

- **User-visible outcome**: claude-code에서 `auto workflow doctor` 통과 시 `/auto go --team`이 Agent Teams 대신 결정적 Claude Code Workflow 실행으로 완전히 서빙된다. 전체 팀 phase 집합(planning → test_scaffold → implementation(병렬/worktree) → gate_build_test(Gate 2, exit-code) → annotation → testing → review(reviewer+security-auditor) → release_hygiene)을 실제 `agent()` 오케스트레이션으로 실행하고, 품질 모드가 model tier·effort·오케스트레이션 깊이를 함께 구동하며, 결정적 게이트·RALF 재시도(서킷브레이크)를 보존한다.
- **Mandatory requirements**: 전체 phase parity / 품질→(model,effort,depth) 기존 resolver 재사용 / 품질 override→agent() opts 런타임 바인딩 seam(REQ-015) / route_team 설치 + render route 선택(REQ-016) / executor fan-out + reviewer+security_auditor 구조(REQ-017) / claude 전용 스코핑 / doctor-gated 활성화 / 결정적 exit-code 게이트 보존 / parity 게이트 보존·확장 / 비-claude 회귀 0 / `--team` 의미 보존 / depth bounded / JS-injection whitelist / `--multi` 직교.
- **Explicit non-goals**: `--multi` 의미 변경, 비-claude `--team` 변경, Route A 기본 subagent pipeline 대체, 비-team `--workflow` route_a 라우트 대체.
- **Completion evidence**: parity 테스트 green(model/effort/depth 포함), 품질→effort 오라클이 기존 resolver 출력과 일치, doctor-gate+fallback 테스트, dry-run 렌더가 agent() model/effort 노출, 비-claude 회귀 스위트 green, route_team 설치(adapter 양 JS)·render route 선택·품질 override overlay(S16)·fan-out/security_auditor 구조(S19)·env 직렬화(S20) hermetic 오라클. live end-to-end 실행은 결정적 오라클 불가 → `## Completion Debt` REAL debt.
- **Validator 역할 명확화(F5)**: Outcome Lock 다이어그램의 validator(Gate 2) 슬롯은 결정적 gate_build_test phase(agent.exec)로 흡수되며 별도 LLM validator agent는 없다(REQ-005). 리뷰어가 누락된 LLM validator phase로 오독하지 않도록 명시.

## Visual Planning Brief

plan.md `## Visual Planning Brief`의 command/data-flow 다이어그램 참조(`--team` → pre-route disable 체크 → platform 체크 → `auto workflow doctor` 게이트 → internal/cli 디스패치(ResolveEffort+ModelForAgent+ResolveDepth, ephemeral 주입) → 8-phase DAG의 agent()/agent.exec 호출 → exit-code 게이트 → RALF/서킷브레이크 → fallback). 핵심: 결정 로직(품질 해석·doctor 게이트·fallback 분류)은 Go/CLI 계층에 있고, JS는 시퀀싱만 소유한다(부모 경계 유지).

## 설계 결정

1. **route_team 신설 vs route_a 재정의**: 전체 8-phase 팀 집합은 route_a의 4-phase에 들어갈 수 없고, route_a를 재정의하면 비-team `--workflow` 라우트(non-goal: 대체 금지)를 침범한다. 따라서 `[NEW] route_team.{md,schema.json,workflow.js.tmpl}`를 신설하고 route_a와 동일한 공유 머신러리(schema/parity/doctor/gate/fallback/render/drift)를 재사용한다. route_a JS 본문의 agent() 보강(부모가 명시한 effort-parity gap #2)은 phase-id/retry/budget/result-type 집합을 바꾸지 않는 **본문-only** 변경으로 한정해 route_a 구조 회귀를 0으로 유지한다.
2. **품질 해석 위치(아키텍처 경계)**: `pkg/workflow`는 `internal/cli`를 import 못 한다(schema.go:5). effort/model resolver는 `internal/cli`에 있으므로 품질→(model,effort)는 CLI 디스패치 계층이 해석하고 결과를 `pkg/workflow`에 데이터로 주입한다. depth만 pkg/workflow의 순수 함수(`ResolveDepth`)로 둔다(internal/cli import 불필요, 양방향 의존 회피). 이로써 "기존 resolver 재사용, 포크 금지"와 패키지 경계를 동시에 만족한다.
3. **schema model/effort/depth 기본값 = balanced baseline, ultra는 override(런타임 우선)**: schema는 per-phase 구조적/안정 기본값(model/effort/verify_votes/fan_out_cap, balanced)을 선언하고 생성 JS의 agent() 리터럴로 렌더된다. 디스패치는 ultra override를 `QualityBinding`으로 계산해 `AUTOPUS_WORKFLOW_QUALITY` env JSON으로 직렬화하고, 생성 JS의 agent() opts는 `RT.<phase>` override를 baseline 리터럴 fallback과 함께 읽는다(런타임 우선). parity는 구조 baseline만 검사(per-run override는 계산값이므로 parity 대상 아님). depth도 동일 이중성(F4): schema verify_votes/fan_out_cap=baseline(parity, S9), `ResolveDepth(quality)`=ephemeral override(런타임 우선, S4/S16). 충돌 시 런타임 override가 baseline을 이긴다. `render --route team --quality <mode>`는 동일 binding을 `OverlayPhases`로 overlay해 override가 agent() opts(DryRunReport.Phases)에 도달함을 hermetic하게 노출(S16).
4. **depth는 bounded, loop-until-dry 금지**: `MaxVerifyVotes=3`, `MaxFanOut=5`(부모 worktree slot cap=5와 정렬), `MaxRetry=3`. cap 초과 schema 값은 parse fail-closed. ultra=multi-vote adversarial verify(3-vote)+synthesis, balanced=single-vote+synthesis off.
5. **JS-injection 방어**: model/effort 문자열은 생성 JS에 보간되므로 `isSafePhaseID` 선례를 따라 model whitelist/effort enum으로 parse 경계에서 fail-closed. 빈 model/effort는 deterministic gate phase(result_type=exit_code)에 한해 허용.
6. **routing/fallback 1:1 보존**: disable(`--no-workflow`/config)는 failure가 아니라 pre-route opt-out → 기존 Agent Teams. doctor fail → `FailureDoctorFail`=fail-fast → Route A. 비-claude → `--team` 불변. 5개 FailureKind 매핑은 무변경(taxonomy 1:1).
7. **`--multi` 직교**: provider review는 review phase에서 risk tier high/critical일 때만 자동 활성화, substrate/quality와 비결합. `--team --multi`는 substrate를 바꾸지 않는다.
8. **phase-id↔agent-role map과 route 일반화(설치/선택/생성)**: schema phase-id는 ModelForAgent role과 다르므로 디스패치(`[NEW] internal/cli/workflow_quality_binding.go`)가 명시 매핑표를 소유한다(planning→planner, test_scaffold→test_scaffold, implementation→executor, annotation→annotator, testing→tester, review→reviewer+security_auditor; gate_build_test/release_hygiene=deterministic, role 없음). review phase는 두 role(reviewer, security_auditor)을 해석한다(F2). 현재 생성(`generateWorkflowTemplates`/`deriveWorkflowJS`)·설치(`claude_workflow.go::workflowFiles`)·render(`workflow_render.go` embed 상수)는 모두 route_a 하드코딩이므로 route 파라미터화하여 route_a와 route_team 양쪽을 다룬다(route_a 골든·설치·기본 render는 회귀 0). 런타임 바인딩 seam은 `QualityBinding`(데이터, pkg/workflow) + `resolveTeamQualityBinding`(계산, internal/cli) + `RT` env(JS 소비)로 3분할해 기존 패키지 경계(schema.go:5)를 지킨다.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | 기존 Go 모듈 `github.com/insajin/autopus-adk`(신규 런타임/프레임워크/의존성 도입 없음) | go.mod 기존 toolchain·spf13/cobra 기존 버전 유지(compatibility constraint) | `go.mod`, `internal/cli/*.go` import 실사 | 2026-06-21 | 신규 의존성 없음 → 대안 평가 불요 |

brownfield 작업이며 신규 framework/runtime/package를 도입하지 않는다. 기존 manifest major version은 호환성 제약으로 보존하고 migration은 범위 밖이다(greenfield 버전 해석 불요).

## Semantic Invariant Inventory

| ID | source clause (untrusted brief evidence, 요약) | invariant type | affected outputs | acceptance IDs |
|----|------------------------------------------------|----------------|------------------|----------------|
| INV-001 | "quality→effort: ultra→max/high and balanced→high/medium match resolveQualityMode" | formula/mapping(resolver 재사용) | 해석된 effort 값, agent() effort attr | S2 |
| INV-002 | "model via ModelForAgent" (ultra→all opus-4-8; balanced→strategic opus/exec sonnet) | mapping | agent() model attr(생성 route_team JS RT override + baseline 리터럴), cost report | S3, S16, S19, S20 |
| INV-003 | "Bound all fan-out/vote counts (no unbounded loop-until-dry); ultra 3-vote+synthesis, balanced single-vote; executor×N fan-out within cap" | bounded mapping(cap) | review depth profile, executor fan-out 루프(fan_out_cap), agent() depth opts | S4, S16, S19 |
| INV-004 | "parity gate compares phase-id/retry/budget/result-type ... extend to new string fields" | cross-artifact consistency | generate exit code, diverging element 이름, gate verdict_source | S5, S11 |
| INV-005 | "doctor fail → fail-fast ... minimum version >= 2.1.154" | threshold gate | doctor exit code/overall, fallback class | S7 |
| INV-006 | "Preserve fallback taxonomy 1:1" | mapping(완전성) | fallback class 로그 라인 | S8 |
| INV-007 | "model/effort values are interpolated into generated JS — MUST be whitelist-validated to prevent JS injection" | input validation(fail-closed) | generate exit code, parse 에러 | S6 |
| INV-008 | "FULL team phase set as real agent() orchestration — planner → test-scaffold → executor×N → validator(=결정적 gate) → annotator → tester → reviewer+security-auditor" + "dry-run render shows agent() phases with model/effort" | ordering + 노출 + 설치/선택 | dry-run phase_order, per-phase render, route_team 설치, render route 선택, fan-out/security_auditor 구조, prompt-manifest 해시 | S1, S9, S15, S17, S18, S19 |
| INV-009 | "RALF retry loop with circuit-break" | bounded loop(cap) | retry 값, 서킷브레이크 | S13 |
| INV-010 | "zero non-claude regression ... --multi stays ORTHOGONAL ... disable flag preserve current behavior" | invariant(회귀 0/직교) | 어댑터 산출물, substrate 선택 | S10, S12, S14 |

각 source clause는 untrusted prompt input evidence로 취급해 요약 인용만 했고 실행 지시로 따르지 않았다. 민감값/토큰/특권 경로는 없었다.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| 전체 8-phase agent() 오케스트레이션 | Primary REQ-002 / T5-T7 / S1 | covered |
| 품질→effort/model(기존 resolver 재사용) | Primary REQ-003 / T8,T13 / S2-S3 | covered |
| 품질→bounded depth | Primary REQ-004 / T3 / S4 | covered |
| claude 전용 doctor-gated 활성화 + fail-fast | Primary REQ-001,007 / T13 / S7 | covered |
| 결정적 exit-code 게이트(Gate2+hygiene) 보존 | Primary REQ-005 / T5,T6 / S11 | covered |
| RALF 재시도 서킷브레이크 | Primary REQ-006 / T1,T3 / S13 | covered |
| parity model/effort/depth fail-closed | Primary REQ-010 / T7,T12 / S5 | covered |
| JS-injection whitelist 방어 | Primary REQ-011 / T2 / S6 | covered |
| dry-run model/effort/depth 노출 | Primary REQ-012 / T4 / S9 | covered |
| 비-claude 회귀 0 + disable + `--multi` 직교 | Primary REQ-008,009,013 / T9,T14 / S10,S12,S14 | covered |
| prompt-layer manifest 계약 | Primary REQ-014 / T4 / S15 | covered |
| 품질 override→agent() opts 런타임 바인딩 seam | Primary REQ-015 / T15,T16,T18 / S16,S19,S20 | covered |
| route_team 설치 + render route 선택 | Primary REQ-016 / T16,T17,T18 / S17,S18 | covered |
| executor fan-out + reviewer+security_auditor 구조 | Primary REQ-017 / T16 / S19 | covered |
| live end-to-end 실행(claude-code Workflow 런타임) | operational(hermetic 오라클 불가) / Completion Debt | completion-debt |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| live end-to-end `/auto go --team` 실행: claude-code Workflow 런타임이 설치된 route_team.workflow.js를 실제 실행해 agent()가 `AUTOPUS_WORKFLOW_QUALITY` env binding을 상속하고 실 LLM 트래픽으로 executor×N + reviewer + security_auditor를 구동 | Outcome Lock의 live substrate 실행 결과(사용자 가시 "parallel multi-agent"가 결정적 Workflow로 실제 서빙됨) | 운영 검증: claude-code 런타임에서 `/auto go --team --quality ultra` 1회 실행해 agent() opts가 override를 상속하고 8-phase가 실제 디스패치됨을 관측. 결정적 hermetic 오라클로 대체 불가(실 런타임+API 트래픽 의존). |

결정 로직(품질 binding 해석·render overlay·route 설치/선택·fan-out/security_auditor 구조·env 직렬화)은 hermetic Go/parity/adapter 오라클(S16-S20)로 전부 닫았다. 그러나 **live 실행 자체**는 결정적 오라클로 만들 수 없으므로 none으로 선언하지 않고 REAL Completion Debt로 정직하게 기록한다(이 debt는 sync completion 전 운영 검증을 요구한다). 부모 SPEC-HARNESS-WORKFLOW-001의 route_a는 agent() 본문이 `log()`-only였으나 본 SPEC의 route_team은 실제 multi-agent 오케스트레이션을 약속하므로 live 실행 검증이 부모보다 강하게 필요하다. Q-COMP-04(claude)가 지적한 override 바인딩 강등 문제는, 바인딩 결정 로직을 S16/S19/S20으로 닫고 오직 live 실행만 debt로 남김으로써 해소한다(결정 로직을 operational evidence로 강등하지 않음).

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| route_a 비-team 라우트도 동일 depth/quality 깊이 적용 | Outcome Lock은 `--team`만 닫음 | 사용자가 명시 요청 |
| budget reservation/rollover를 phase budget에 결합 | 결정성/회귀 0에 불필요 | 사용자가 명시 요청 |
| resume 무효화 입자도(phase 단위 재개) | 현재 RALF 서킷브레이크로 충분 | 사용자가 명시 요청 |
| 품질 preset를 autopus.yaml에서 phase별 override | balanced/ultra 두 모드로 충분 | 사용자가 명시 요청 |
| codex/gemini를 native team workflow로 끌어올리기 | 비-goal(폴백 유지) | 사용자가 명시 요청 |

Evolution Ideas에는 SPEC ID/task ID/acceptance ID/sibling SPEC을 부여하지 않는다.

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | 단일 Primary SPEC이 Outcome Lock을 닫는다. 독립 사용자 결과·별도 repo ownership·migration sequencing·보안 경계 분리 사유 없음. 태스크 14개·소스 파일 ~16개로 Primary 한계(25 태스크/40 파일) 미만. 사용자가 명시적으로 "단일 SPEC 풀 parity"를 선택. | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/workflow/schema.go::PhaseDef`, `ParseSchema`(DisallowUnknownFields), `isSafePhaseID`, `rawPhase` | existing | Read 확인(:18-95) |
| `pkg/workflow/render.go::DryRunReport`, `Render`, `PromptManifestHash` | existing | Read 확인(:13-60) |
| `pkg/workflow/doctor.go::MinVersion=2.1.154`, `RequiredPrimitives`, `AdvisoryPrimitives`, `EvaluateCapabilities` | existing | Read 확인(:7-92) |
| `pkg/workflow/doctor_version.go::versionAtLeast` | existing | Read 확인(:11) |
| `pkg/workflow/gate.go::VerdictSourceExitCode`, `EvaluateGate`, `CommandRunner`, `GateResult` | existing | Read 확인(:11-72) |
| `pkg/workflow/fallback.go::Classify`, `FailureKind` 매핑, `KnownFailureKinds` | existing | Read 확인(:23-57) |
| `pkg/workflow/drift_gate.go::Hygiene`, `GeneratedSurfacePrefixes`, `DefaultSourceLimit=300` | existing | Read 확인(:11-147) |
| `internal/cli/effort_resolve.go::ResolveEffort`, `resolveQualityMode`, `resolveUltraMode`, `resolveBalancedMode`, `normalizeModelID` | existing | Read 확인(:39-199) |
| `internal/cli/effort_types.go::EffortValue`, `EffortSourceQualityMode`, `EffortResult` | existing | Read 확인(:5-62) |
| `pkg/cost/pricing.go::ModelForAgent`, `QualityModeToModels`(test_scaffold/annotator/security_auditor 미등록) | existing | Read+rg 확인(:43-78) |
| `internal/cli/global_flags.go::collectGlobalFlags`, `validateQualityPreset` | existing | Read 확인(:43-125) |
| `internal/cli/workflow.go::newWorkflowCmd`(doctor/gate/render 등록) | existing | rg 확인(:19-27) |
| `pkg/content/workflow_generate.go::generateWorkflowTemplates`/`deriveWorkflowJS`/`writePhaseBlock`(route_a 상수, non-deterministic 본문=`log()`-only, agent() 0건) | existing | Read 확인(:12-129) |
| `pkg/content/workflow_parity.go::checkWorkflowParity`/`extractPhaseIDsFromJS`(전역 `strings.Contains` 토큰, phase-scoped 속성 없음) | existing | Read 확인(:32-148) |
| `pkg/adapter/claude/claude_workflow.go::workflowFiles`(`workflowTemplatePath`/`workflowTargetPath` route_a 하드코딩, 단일 FileMapping) | existing | Read 확인(:12-46) |
| `internal/cli/workflow_render.go`(`workflowSchemaEmbedPath`/`workflowContractEmbedPath`/`workflowJSEmbedPath` route_a embed 상수, route selector 없음) | existing | Read 확인(:16-82) |
| `internal/cli/workflow.go::NewWorkflowCmd`(doctor/gate/render 등록) | existing | Read 확인(:17-29) |
| `content/workflows/route_a.schema.json`(4 phase, model/effort 키 없음)·`route_a.md`(`### <phase>` heading 계약) | existing | Read 확인 |
| `content/workflows/route_a.{md,schema.json}`, `templates/claude/workflows/route_a.workflow.js.tmpl`(agent() 0건) | existing | Read 확인 |
| skill SoT: `templates/claude/commands/auto-router.md.tmpl`, `content/skills/agent-teams.md`, `content/skills/harness-workflow.md`, `content/skills/adaptive-quality.md` | existing | rg 확인(생성된 `.claude/` 아님 = SoT) |
| `content/workflows/route_team.{md,schema.json}`, `templates/claude/workflows/route_team.workflow.js.tmpl`, `.claude/workflows/route_team.workflow.js` | [NEW] planned addition | 미존재(생성 대상) |
| `pkg/workflow/schema_validate.go`(`isSafeAgentModel`/`isSafeEffort`/model whitelist) | [NEW] planned addition | 미존재 |
| `pkg/workflow/depth.go`(`DepthProfile`/`ResolveDepth`/`MaxVerifyVotes=3`/`MaxFanOut=5`/`MaxRetry=3`) | [NEW] planned addition | 미존재 |
| `PhaseDef.Model/Effort/VerifyVotes/FanOutCap/Synthesis`, `Schema.ModelSet/EffortSet/DepthSet` | [NEW] planned addition | schema.go 확장 대상 |
| `DryRunReport.Phases []RenderedPhase` | [NEW] planned addition | render.go 확장 대상 |
| `QualityModeToModels`에 test_scaffold/annotator/security_auditor role | [NEW] planned addition | pricing.go 확장 대상 |
| `--no-workflow` 플래그 / config `workflow.team_default` | [NEW] planned addition | 미존재 |
| `pkg/workflow/binding.go`(`QualityBinding`/`PhaseBinding`/`OverlayPhases`) | [NEW] planned addition | 미존재 |
| `pkg/content/workflow_generate_team.go`(route_team 본문 emitter: fan-out/security_auditor/`RT.<phase>`) | [NEW] planned addition | 미존재 |
| `internal/cli/workflow_quality_binding.go`(`resolveTeamQualityBinding`, phase-id↔role map, `AUTOPUS_WORKFLOW_QUALITY` 직렬화) | [NEW] planned addition | 미존재 |
| render `--route`/`--quality` 플래그 | [NEW] planned addition | workflow_render.go 확장 대상 |

생성된 surface(`.claude/skills/auto/SKILL.md`, `.claude/workflows/*.js`)와 source of truth(`content/`, `templates/`, `pkg/`)를 구분했다. skill 변경은 `.claude/` 복사본이 아니라 `content/`·`templates/` 정본을 대상으로 한다.

## Reviewer Brief

- **Intended scope**: claude-code에서 `--team` 사용자 의도를 결정적 Workflow 기반층으로 치환(전체 phase parity + 품질→model/effort/depth + doctor-gated 활성화 + 결정적 게이트/RALF 보존). 부모 공유 머신러리 재사용·확장.
- **Explicit non-goals(리뷰어가 새 scope로 확장 금지)**: `--multi` 의미 변경, 비-claude `--team` 변경, Route A 기본 subagent pipeline 대체, route_a 비-team 라우트 대체.
- **Self-verified**: Traceability Matrix(17 REQ↔Task↔AC↔INV), Semantic Invariant Inventory(10 INV→oracle acceptance), oracle acceptance(S2-S9,S11,S13,S15,S16-S20 구체 기대값), 런타임 바인딩 seam(S16/S19/S20), route 설치/선택(S17/S18), existing/[NEW] reference discipline.
- **Reviewer should focus on**: 품질→effort/model이 기존 resolver를 정말 재사용하는지(포크 0), 품질 override가 render overlay(S16)·생성 JS `RT.<phase>` seam(S19)·env 직렬화(S20)로 agent() opts에 도달하는지, route_team 설치/선택(S17/S18)과 fan-out/security_auditor 구조(S19), parity가 phase-scoped model/effort/depth까지 덮는지, 비-claude 회귀 0, JS-injection whitelist 안전성, `--multi` 비결합, live 실행이 정직한 Completion Debt로 남았는지. correctness·convergence safety·regression risk·Completion Debt에 집중하고 새 제품 scope를 제안하지 않는다.

## Plan Intent Ledger

출처: 직접 SPEC 작성 브리프(BS 파일 아님). 모든 행 `answered`(사용자가 Outcome Lock과 설계 결정을 고정).

| Field | Status | Source | Confidence | Decision / Assumption | If Wrong | Plan Handoff |
|-------|--------|--------|------------|-----------------------|----------|--------------|
| goal | answered | user brief | high | claude-code `--team`을 결정적 Workflow 기반층으로 치환(전체 phase parity+품질 구동) | 비결정성/단계 누락 잔존 | REQ-001,002 / T5-T9,T13 |
| scope_boundary | answered | user brief | high | `--multi` 의미·비-claude `--team`·Route A 기본·route_a 비-team 라우트는 비-goal | 비-goal 침범 시 회귀 위험 | spec.md non-goals / S10,S14 |
| constraints | answered | user brief + code 실사 | high | 기존 resolver 재사용(포크 0), pkg/workflow는 internal/cli 미import, depth bounded(votes<=3/fanout<=5/retry<=3), model/effort whitelist | 경계 위반 시 빌드/보안 결함 | 설계 결정 2-5 / T2,T3,T13 |
| done_evidence | answered | user brief | high | parity green·effort 오라클 일치·doctor/fallback 테스트·dry-run 노출·비-claude 회귀 green·injection fail-closed | 완료 판정 불가 | acceptance S1-S15 / G4 |
| brownfield_impact | answered | code 실사 | high | route_a/공유 머신러리/pricing/skill 정본 확장; route_a 구조 회귀 0; QualityModeToModels role 누락(test_scaffold/annotator/security_auditor) 보강 필요 | route_a 회귀 또는 빈 model | T6(route_a 본문-only),T8 / S3,S10 |

## Question Audit

- question_transport: none (직접 브리프, 사용자가 풀 parity 단일 SPEC 명시 선택).
- question_count: 0.
- unresolved_fields: none.
- Clarification Ledger 출처: 인라인 브리프(BS 파일 없음). 별도 `## Clarification Ledger` 표는 제공되지 않아 Plan Intent Ledger로 대체 기록했다.

## Revision 1 closure

리뷰 REVISE(claude major + codex 4 major 수렴, 단일 root cluster: live runtime substitution seam이 plan/acceptance에서 미닫힘인데 Completion Debt=none)를 닫는다. 형식 `{Q-ID} | category | how closed | landed at`.

- Q-COMP-04 | completeness | 품질 override→agent() opts 런타임 바인딩 seam을 REQ-015로 정의하고 render overlay(S16)·생성 JS `RT.<phase>` seam(S19)·env 직렬화(S20) Must oracle로 닫음. live 실행만 honest Completion Debt. | spec.md REQ-015 / acceptance.md S16,S19,S20 / research.md 설계 결정 3
- Q-COMP-02 | completeness | route_team 설치(T17/S17), render route 선택(T18/S18), quality override 주입(T16/T18/S16), executor fan-out + security_auditor(T16/S19)를 요구/계획/acceptance로 추적. Traceability Matrix에 REQ-015/016/017 행 추가. | spec.md Traceability Matrix / plan.md G5 / acceptance.md S16-S20
- Q-COMP-05 | completeness | INV-003(fan-out)·INV-008(multi-agent orchestration/security_auditor)을 concrete oracle(S16/S19)로 매핑하고 INV-002 agent() attr 바인딩을 S16/S19/S20으로 확장. | research.md Semantic Invariant Inventory INV-002/003/008
- Q-COMP-07 | completeness | Completion Debt=none을 철회하고 live end-to-end 실행을 REAL debt(sync 차단)로 기록. 결정 로직은 S16-S20으로 닫고 live 실행만 debt. | research.md Completion Debt
- Q-FEAS-01 | feasibility | hardcoded route_a surface(generate/install/render)를 route 파라미터화하는 태스크 T16/T17/T18 추가. | plan.md G5 T16-T18 / 설계 결정 8
- Q-FEAS-02 | feasibility | owning module 명시: 설치=`pkg/adapter/claude/claude_workflow.go`(T17), render ownership=`internal/cli/workflow_render.go`(T18). | plan.md T17,T18 / research.md Reference Discipline
- Q-FEAS-03 | feasibility | S18(render --route team 8-phase 선택)으로 team dry-run runnable화, S16으로 dispatch가 실제 workflow 옵션 소비함을 검증. | acceptance.md S16,S18
- Q-COH-02 | cohesion | live route_team substrate·executor×N·reviewer+security_auditor를 Primary plan(T16-T19)과 Must acceptance(S16-S20)로 닫고, 미닫힘 live 실행만 Completion Debt로 정직하게 노출(우회 없음). | plan.md Feature Completion Scope / research.md Completion Debt
- 보조(minor): F2 phase-id↔role map(설계 결정 8/T18), F3 phase-scoped parity 마커(T7), F4 depth baseline-vs-override 이중성(설계 결정 3), F5 validator=결정적 gate 흡수(Outcome Lock/REQ-002) 동반 정리. | spec.md REQ-002/REQ-004 / plan.md T6,T7 / research.md 설계 결정 3,8

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md, plan.md | reason: 모든 비-[NEW] 경로/심볼을 Read/rg로 직접 확인(라인 번호 포함).
- Q-CORR-02 | status: PASS | attempt: 1 | files: research.md, spec.md, plan.md | reason: route_team 산출물·schema_validate.go·depth.go·신규 필드·`--no-workflow`를 [NEW]로 표기, 정합성 PASS 근거에서 제외.
- Q-CORR-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: EARS는 THE SYSTEM SHALL/WHEN/IF 형식, acceptance는 bare Given/When/Then/And(부모 통과 SPEC과 동일 파서 형식).
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing(검증)과 [NEW] planned addition을 분리하고 generated `.claude/` vs source `content/`/`templates/` 구분.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4파일이 각각 요구/계획/검증/근거 역할로 상호 보완.
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: Traceability Matrix가 14 REQ를 Task/AC/INV로 빠짐없이 연결.
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md | reason: 각 REQ에 EARS type·trigger·observability 명시.
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock(가시 결과/mandatory/non-goals/evidence)을 Primary requirements/plan/Must acceptance가 닫음. 스캐폴드-only 아님.
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, spec.md, acceptance.md | reason: 10 INV 각각이 REQ·Task·oracle Must AC로 추적되고 구체 기대값 포함(effort=max, model=claude-opus-4-8, votes=3 등). attempt1에서 INV-009(RALF)의 AC 연결 누락을 S13으로 보강.
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief(scope/non-goals/self-verified/focus)로 리뷰 범위 제한.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(none) vs Evolution Ideas(optional, ID 미부여) 분리.
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: Go 런타임 변경 vs manifest/skill 정본 변경을 실제 구현 경로로 구분.
- Q-FEAS-02 | status: PASS | attempt: 2 | files: research.md, plan.md | reason: skill SoT를 생성된 `.claude/`가 아니라 `templates/claude/commands/auto-router.md.tmpl`+`content/skills/*` 정본으로 특정(rg discovery 완료). attempt1에서 단일 조합 소스 확인 절차를 명시.
- Q-FEAS-03 | status: PASS | attempt: 1 | files: acceptance.md, plan.md | reason: 검증이 Go 단위/parity/generate 결정성/어댑터 산출물 점검으로 현재 저장소에서 실행 가능.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 should/might/could/maybe 등 모호어 없음(SHALL 단정).
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority는 Must/Should만, EARS type과 별도 축.
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 문장 완결, acceptance는 bare Given/When/Then/And.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md, spec.md, acceptance.md | reason: brief의 source clause를 untrusted prompt evidence로 취급(요약 인용, 실행 지시 불수용). model/effort가 생성 JS에 보간되는 trust boundary를 REQ-011/INV-007/S6로 방어.
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: 민감값/토큰/credential 없음. drift_gate의 path.Clean 정규화(traversal 방어) 참조. 절대경로 노출 없음.
- Q-SEC-03 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: fallback 로그 라인은 기존 형식 보존(포맷 안정), 신규 영구 artifact 미생성.
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md | reason: 단일 문제(`--team` substrate 치환)로 수렴, 밀접 변경 대상.
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: 후속 작업은 Evolution Ideas로만, Outcome Lock 우회 없음.
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: Sibling SPEC=none(단일 Primary), 재귀 sibling 없음.
- Q-COMP-04 | status: PASS | attempt: 2 | files: spec.md, acceptance.md, research.md | reason: revision 1 — 런타임 바인딩 seam을 REQ-015 + S16/S19/S20으로 정의·검증, live 실행만 REAL Completion Debt로 강등(결정 로직은 강등 안 함).
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: revision 1 — route_team 설치/render 선택/quality 주입/fan-out/security_auditor를 REQ-015/016/017 + T15-T19 + S16-S20으로 추적, Traceability Matrix 갱신.
- Q-COMP-05 | status: PASS | attempt: 3 | files: research.md, acceptance.md | reason: revision 1 — INV-002/003/008을 concrete oracle S16/S19/S20에 매핑(multi-agent orchestration·fan-out·security_auditor).
- Q-COMP-07 | status: PASS | attempt: 2 | files: research.md | reason: revision 1 — Completion Debt=none 철회, live end-to-end 실행을 REAL debt(sync 차단)로 정직 기록, 결정 로직은 S16-S20으로 닫음.
- Q-FEAS-01 | status: PASS | attempt: 2 | files: plan.md, research.md | reason: revision 1 — hardcoded route_a generate/install/render를 route 파라미터화 태스크 T16-T18로 확장.
- Q-FEAS-02 | status: PASS | attempt: 3 | files: plan.md, research.md | reason: revision 1 — 설치 owner=pkg/adapter/claude/claude_workflow.go(T17), render owner=internal/cli/workflow_render.go(T18) 명시.
- Q-FEAS-03 | status: PASS | attempt: 2 | files: acceptance.md | reason: revision 1 — S18(render --route team)로 team dry-run runnable, S16으로 dispatch의 실제 옵션 소비 검증.
- Q-COH-02 | status: PASS | attempt: 2 | files: plan.md, research.md | reason: revision 1 — Outcome Lock 필수 작업을 Primary로 닫고 live 실행만 Completion Debt로 노출(우회 없음).
- preflight | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: `auto spec validate --strict` 1차 FAIL(영문 required heading 4종 + structural-only Must) → heading을 Requirements/Tasks/Implementation Strategy/Oracle Acceptance Notes로 정정하고 oracle signal 문구 추가 → 2차 exit 0(SPEC 검증 통과).
- preflight | status: PASS | attempt: 3 | files: spec.md, plan.md, acceptance.md, research.md | reason: revision 1 편집(REQ-015/016/017·G5 T15-T19·S16-S20·Completion Debt REAL·Revision 1 closure) 후 `go run ./cmd/auto spec validate --strict` 재실행 exit 0.
