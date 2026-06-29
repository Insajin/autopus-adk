# SPEC-HARNESS-WORKFLOW-STABILITY-001 리서치

## Reviewer Brief

- **Intended scope**: route_team을 manual-pipeline-equivalent stability로 만드는 네 reinforcement
  (gate RALF retry, 85% coverage gate, review barrier, 실 WorkflowConf.team_default) + parity
  fail-closed 확장 + regression-0. 변경 표면은 route_team manifest/generator/parity, pkg/workflow
  coverage gate + remediation 결정 함수, pkg/config workflow conf, 4개 dispatcher-contract 문서.
- **Explicit non-goals (리뷰어가 새 scope로 확장 금지)**: route_a 변경, plain `/auto go` workflow
  default, Context7/Phase 3.5 추가, non-claude 플랫폼 변경, 결정적 gate JS에 LLM verdict embed.
- **Self-verified**: Traceability Matrix(REQ-001~015 ↔ task ↔ scenario ↔ INV), Semantic Invariant
  Inventory(13개 oracle 매핑, INV-001~013), oracle acceptance(S1–S15 concrete 기대값). multi-segment
  interposition(REQ-013)은 prose가 아니라 `deriveTeamWorkflowJS`의 실제 `SEGMENT==='C'`/`'D'` guard로
  보장되어 S13 launch-contract oracle로 검증되고, absent-section Load default(REQ-014)는 S14,
  schema-level coverage 검증(REQ-015)은 S15로 닫힌다. RALF/review-barrier/security-priority
  결정 로직이 prose가 아니라 `[NEW] pkg/workflow/remediation.go` 순수 함수로 추출되어 S1/S2/S5/S6
  hermetic oracle로 검증됨. coverage gate가 stdout 반환 `[NEW] CoverageRunner` seam을 씀(exit-code-only
  `CommandRunner`(stdout 없음, gate.go:21)과 구분). existing/[NEW] reference discipline,
  DisallowUnknownFields 함정과 import 경계.
- **Reviewer should focus on**: correctness(coverage 경계·exit-code 도출·결정 함수 반환값
  일관성[FixerAttempts/Aborted/AbortReason/Barrier/Reason]·security 우선순위), convergence safety
  (bounded retry + circuit-break가 무한 루프를 막는지; multi-segment guard가 coverage/review
  interposition 재진입점을 실제로 만드는지[S13]), regression risk(route_a 2-segment·non-claude·parity
  불변, absent-section이 substrate를 silent 비활성화하지 않는지[S14]), Completion Debt(라이브 실행
  residual 정직성). 새 제품 scope 제안이 아니라 이 blocker 검증에 집중한다.

## Self-Verify Summary
- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 인용한 기존 경로/심볼/라인을 rg+Read로 확인했고 토큰 디스플레이 아티팩트는 Read로 교차검증함
- Q-CORR-01 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: 기존 CommandRunner가 exit code만 반환(stdout 없음, gate.go:21)임을 Read로 재확인하고 Reference Discipline·설계 결정에 반영(F-001)
- Q-CORR-02 | status: PASS | attempt: 1 | files: research.md, spec.md, plan.md | reason: 신규 파일/심볼/schema 값을 [NEW] 또는 planned addition으로 표기하고 정합성 PASS 근거에서 제외함
- Q-CORR-02 | status: PASS | attempt: 2 | files: research.md, spec.md, plan.md | reason: 신규 remediation.go/CoverageRunner를 [NEW] planned addition으로 표기하고 기존 CommandRunner와 분리
- Q-CORR-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: acceptance는 bare Given/When/Then이며 EARS는 spec.md에서 WHEN/WHILE/WHERE + SHALL 형식 사용
- Q-CORR-03 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: REQ-001~006 관측점이 runnable 함수(RunGateRemediation/EvaluateCoverageGate/RunReviewBarrier/ConsolidateReviewVerdict)를 명명하고 acceptance가 동일 함수에 대한 concrete 기대값을 검증
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline이 existing(검증됨)과 [NEW] planned addition을 분리하고 generated surface(JS)와 SoT(manifest)를 구분함
- Q-COMP-01 | status: PASS | attempt: 1 | files: all four | reason: 4파일이 각자 역할(요구/계획/검증/근거)을 갖고 상호 보완함
- Q-COMP-02 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md | reason: Traceability Matrix가 REQ-001~012를 task와 scenario에 빠짐없이 연결함
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md | reason: Traceability Matrix를 T4(remediation)/T7(dispatcher) 신설에 맞춰 renumber하고 REQ↔task↔scenario↔INV 연결 유지
- Q-COMP-02 | status: PASS | attempt: 3 | files: spec.md, acceptance.md, research.md | reason: REQ-013/014/015 ↔ T9/T5/T10 ↔ S13/S14/S15 ↔ INV-011/012/013 매핑을 Matrix·Inventory·Coverage Map에 추가해 추적성 유지
- Q-COMP-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 각 REQ에 EARS type/조건/기대결과/관측지점(GateResult, cfg field, parity error)이 명시됨
- Q-COMP-04 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: Outcome Lock의 mandatory 4 reinforcement가 모두 requirements/plan/Must acceptance로 닫히고 라이브 실행은 Completion Debt로 분리됨
- Q-COMP-04 | status: PASS | attempt: 2 | files: research.md | reason: 네 reinforcement가 runnable 결정 함수 + CoverageRunner seam으로 닫히고 라이브 실행만 Completion Debt로 유지
- Q-COMP-05 | status: PASS | attempt: 1 | files: research.md, spec.md, acceptance.md | reason: INV-001~010이 REQ/task/oracle acceptance에 추적되고 coverage(numeric)·retry(bounded)·priority가 concrete 기대값으로 검증됨
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: INV-001/002/004/005가 순수 결정 함수의 반환 필드로 매핑되고 S1/S2/S5/S6가 hermetic oracle(정확값 2/1/2/security_fail)로 검증
- Q-COMP-05 | status: PASS | attempt: 3 | files: research.md, acceptance.md, spec.md | reason: INV-011(multi-segment guard 구조)·INV-012(Load backfill)·INV-013(ParseSchema range)가 S13/S14/S15 oracle로 매핑되어 interposition이 structural-only가 아닌 실제 재진입점으로 검증됨
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix와 Reviewer Brief가 scope/non-goals/focus를 제한함
- Q-COMP-06 | status: PASS | attempt: 2 | files: research.md | reason: Reviewer Brief/Self-Verify Summary가 research.md에 이미 존재·prominent(F-005 false positive 확인) + 결정 함수 추출 반영
- Q-COMP-06 | status: PASS | attempt: 3 | files: research.md, spec.md | reason: 재발 false positive(인용 window 밖) 차단 위해 Reviewer Brief/Self-Verify Summary를 research.md 상단(인용 window 안)으로 이동, Traceability Matrix는 REQ-013/014/015·S13/S14/S15 추가
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(라이브 실행 blocker)와 Evolution Ideas(optional, ID 미부여)가 분리됨
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: schema/Go 코드 변경과 dispatcher-contract prose 변경의 구현 층이 실제 경로와 일치함
- Q-FEAS-01 | status: PASS | attempt: 2 | files: plan.md, acceptance.md | reason: 제어 흐름이 prose-only가 아니라 runnable pkg/workflow Go(remediation.go)로 구현되어 검증 층이 실제 구현과 일치(F-002/F-004)
- Q-FEAS-01 | status: PASS | attempt: 3 | files: spec.md, plan.md, acceptance.md | reason: coverage/review interposition이 prose가 아니라 `deriveTeamWorkflowJS` multi-segment(A/B/C/D) generator 변경(T9)으로 실제 dispatcher 재진입점을 만들고 S13가 생성 JS의 segment-guard 구조를 검증(F-002 종결)
- Q-FEAS-02 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: 변경 대상이 SoT manifest+generator+parity이고 generated JS(edit-forbidden)와 구분됨
- Q-FEAS-03 | status: PASS | attempt: 1 | files: plan.md, acceptance.md | reason: 검증이 go build/vet/test -race + auto workflow render + parity로 현 저장소에서 실행 가능함
- Q-FEAS-03 | status: PASS | attempt: 2 | files: acceptance.md, plan.md | reason: S1~S6가 go test로 실행 가능한 순수 함수/seam 단위 검증으로 전환됨
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ description에 모호어(should/might/could 등) 없이 SHALL로 단정함
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority는 Must/Should만 사용하고 EARS type과 별개 축으로 둠
- Q-STYLE-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: 문장이 완결되고 acceptance는 bare Given/When/Then/And 형식
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: source clause를 untrusted prompt input으로 취급(지시 아닌 근거)하고, schema.go의 phase-id/model/effort/result_type JS-injection whitelist 경계를 보존하며 새 field도 동일 trust 경계 안에 둠
- Q-SEC-02 | status: N/A | attempt: 1 | files: research.md | reason: 비밀값/토큰/credential/privileged 절대경로를 다루지 않음 — coverage 명령 출력 파싱과 config bool/int field만 처리함
- Q-SEC-03 | status: N/A | attempt: 1 | files: research.md | reason: 별도 영구 로그/artifact를 만들지 않음; review audit 영속화는 Evolution Ideas로만 둠
- Q-COH-01 | status: PASS | attempt: 1 | files: all four | reason: 단일 문제(route_team 안정성 4격차)로 수렴하고 변경 대상이 밀접함
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: Outcome Lock 필요 작업은 Primary SPEC에 포함, 라이브 실행만 Completion Debt, optional은 Evolution Ideas
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: Sibling SPEC Decision=none, 기존 TEAM-001 의존만 있고 재귀 sibling 없음

## 기존 코드 분석

검증 방법: 인용한 경로/심볼/라인은 `rg`와 `Read`로 직접 확인했다. 일부 `rg` 출력에서 Go 타입 토큰이
`n`으로 표시되는 디스플레이 아티팩트가 있었으나, 실제 파일 내용은 `Read`로 재확인했다.

### route_team substrate (의존 대상, TEAM-001 산출물)
- `content/workflows/route_team.schema.json` — 8 phase. 현재 `gate_build_test` `retry: 0`,
  `review` `retry: 0`, `implementation`만 `retry: 2`. `testing`에 coverage field 없음. gate는
  `verdict_source: exit_code`.
- `content/workflows/route_team.md` — human-authoritative manifest. `### testing`(line 61)은
  "round out coverage … confirm the suite is green"만, 임계값 없음. `### review`(line 66)는
  reviewer(verify_votes) + security_auditor + optional synthesis만, barrier 동작 없음.
- `.claude/workflows/route_team.workflow.js` — 설치된 generated surface (edit-forbidden).
- `templates/claude/workflows/route_team.workflow.js.tmpl` — 템플릿 generated surface (edit-forbidden).

### 생성 + parity (SoT → JS)
- `pkg/content/workflow_generate_team.go` — `deriveTeamWorkflowJS(schema)`가 결정적으로 JS를 생성.
  `writeTeamBaselineComment`(line 244)이 parity가 읽는 `model=/effort=/depth` token 라인을 emit
  (포맷이 계약). `writeTeamPhaseBlock`(line 133)에서 phase별 extra token(`retry: N budget: N`,
  `fan_out_cap=N`, `verify_votes=N synthesis=t`)을 구성.
- `pkg/content/workflow_parity.go` — `checkWorkflowParity`(line 32)가 phase-id/retry/budget/
  result-type token을 schema↔JS↔markdown에서 비교, fail-closed. `checkPerPhaseQualityParity`
  (line 101)가 phase block 안의 model/effort/depth token을 검증. `phaseJSBlock`(line 140)이 phase
  block을 슬라이스.

### schema 파싱 (확장 위험 지점)
- `pkg/workflow/schema.go` — `PhaseDef`(line 18)와 `rawPhase`(line 38) struct. **`ParseSchema`
  (line 57)가 `dec.DisallowUnknownFields()`(line 60)를 사용** → schema JSON에 새 key를 넣으려면
  반드시 `PhaseDef`+`rawPhase`에 대응 field를 먼저 추가해야 한다. `validateDepthCaps`(depth.go:40)가
  retry/fan_out/verify_votes를 cap에 대해 fail-closed로 검증.

### 결정적 gate (재사용 패턴 + stdout seam 한계)
- `pkg/workflow/gate.go` — `EvaluateGate`(line 39)가 `CommandRunner`(line 17) seam으로 build/test를
  돌려 exit-code에서 verdict 도출. **`CommandRunner.Run(ctx, name string, args ...string) (exitCode
  int, err error)`은 exit code만 반환하고 stdout을 반환하지 않는다(line 21)** → coverage%를 파싱할
  수 없으므로 별도의 stdout 반환 seam이 필요하다(리뷰 finding F-001). `GateResult`(line 26) struct.
  `VerdictSourceExitCode = "exit_code"`(line 11). coverage gate는 verdict 도출 패턴(GateResult/
  VerdictSourceExitCode)은 재사용하되 stdout seam(`[NEW] CoverageRunner`)을 새로 둔다.
- `internal/cli/workflow_gate.go`, `internal/cli/workflow_render.go` — CLI wiring. 테스트
  `internal/cli/workflow_test.go`, `internal/cli/workflow_render_test.go`(hermetic `renderLines`
  헬퍼 존재).

### depth caps + binding (재사용)
- `pkg/workflow/depth.go` — `MaxVerifyVotes=3`, `MaxFanOut=5`, `MaxRetry=3`(line 19-21).
  `ResolveDepth`(line 27). gate/review retry는 기존 `MaxRetry`로 이미 capped. remediation 함수의
  budget 인자도 이 cap에서 유래한다.
- `pkg/workflow/binding.go` — `QualityBinding`(line 20), `OverlayPhases`(line 30). quality→
  (model,effort)는 `internal/cli/workflow_quality_binding.go::resolveTeamQualityBinding`(line 33)에서
  주입. `pkg/workflow`는 `internal/cli`를 import하지 않음(schema.go:5).
- 리졸버: `pkg/cost/pricing.go::ModelForAgent`(line 80), `internal/cli/effort_resolve.go::ResolveEffort`
  (line 39).

### config (REQ-007~009 핵심)
- `pkg/config/schema.go` — `HarnessConfig`(line 59-87)에 **`Workflow` field 없음**. `Quality QualityConf`
  (line 75), `Spec SpecConf`(line 69)는 실존. `Validate`(line 231)에서 `Quality.Default`(line 246)와
  `Features.CC21.TaskCreatedMode`(line 251)를 검증.
- `pkg/config/schema_spec.go` — `SpecConf.ReviewGate ReviewGateConf`, `ReviewGateConf.Enabled`
  (line 11)는 실존 validated field.
- `pkg/config/defaults.go` — `DefaultFullConfig`(line 38). `"workflow": 10`(line 159)은
  `Skills.CategoryWeights`의 token-limit 값으로 `team_default`과 무관.
- `pkg/config/loader_test.go` — `Load`(line 19, 40 등)로 YAML을 실제 unmarshal하는 테스트 패턴.
  `TestLoad_ExplicitDesignDisabledIsPreserved`(line 49)가 explicit `false`가 보존되는지 검증하는
  정확한 선례 — S7/S8의 oracle 패턴.

### phantom key team_default 참조 (REQ-010, 정확 4파일)
`rg`로 확정한 `team_default` 참조 전체 집합 (코드 0건, 문서 4건):
- `content/skills/harness-workflow.md:145`
- `content/skills/agent-teams.md`
- `templates/claude/commands/auto-router.md.tmpl` (substrate-selection table ~1095/1096)
- `templates/gemini/skills/agent-teams/SKILL.md.tmpl`

확인: codex agent-teams 미러와 gemini auto-router는 `team_default`을 **포함하지 않음** → 미러
업데이트는 위 4파일에 한정 (없는 토큰을 새로 만들지 않는다).

## Outcome Lock

- **User-visible outcome**: claude-code(doctor pass)에서 `/auto go --team`이 manual-pipeline-equivalent
  stability로 `route_team`을 실행한다 — gate/build/test 실패와 review REQUEST_CHANGES가 hard abort가
  아니라 bounded remediation loop를 트리거하고, 결정적 85% coverage gate가 enforce되며,
  `workflow.team_default`이 실 validated config knob이다.
- **Mandatory requirements**: REQ-001~010 (네 reinforcement). REQ-011(parity fail-closed)·
  REQ-012(regression-0)는 안전 불변.
- **Explicit non-goals**: route_a 불변 / plain `/auto go` workflow default 금지 / route_team에
  Context7·Phase 3.5 추가 금지 / non-claude 플랫폼 불변 / 결정적 gate JS에 LLM verdict embed 금지.
- **Completion evidence**: schema retry/coverage field + parity green / dispatcher contract 4파일
  업데이트 / 새 `WorkflowConf` + 통과 config 테스트 / `pkg/workflow` 순수 결정 함수
  (RunGateRemediation/RunReviewBarrier/ConsolidateReviewVerdict)·EvaluateCoverageGate + 통과 단위
  테스트(S1–S6) / `auto workflow render --route team` 반영 / `go build/vet/gofmt/-race` green / 새
  `.go` ≤ 300 lines.

## Visual Planning Brief

작업 성격은 CLI/Go backend + dispatcher contract prose이므로 sequence/command-flow로 설명한다
(plan.md `## Visual Planning Brief` 참조). 핵심: 결정적 gate는 exit-code로 유지하고, RALF/coverage/
review-barrier **결정 로직은 `pkg/workflow`의 순수 함수(`RunGateRemediation`/`EvaluateCoverageGate`/
`RunReviewBarrier`/`ConsolidateReviewVerdict`)가 source of truth**이며 단위 테스트로 검증된다.
dispatcher는 그 결정값을 따라 실제 fixer/reviewer agent를 spawn하고, JS는 LLM verdict 없는 boundary
marker만 emit한다.

## 설계 결정

- **왜 RALF/review-barrier 결정 로직을 순수 Go(`[NEW] pkg/workflow/remediation.go`)에 두는가**: loop
  count, no-progress circuit-break, security>code-quality 우선순위를 prose로만 두면 hermetic oracle로
  검증할 수 없다(리뷰 finding F-002/F-004 — prose-only 동작을 실행 가능 oracle로 취급한 문제). 그래서
  결정 로직을 LLM-free 순수 함수(`RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`)로
  추출해 source of truth로 삼고 단위 테스트로 S1/S2/S5/S6를 닫는다. dispatcher contract(prose)는 그
  함수 의미를 따라 실제 fixer/reviewer agent를 spawn하는 operational layer이며, 결정적 gate JS에는
  여전히 LLM verdict를 embed하지 않는다(하드 제약 유지). 기존 segmented dispatch가 이미 segment 사이에서
  `auto workflow gate`를 돌리므로(harness-workflow.md:218), 그 사이에 결정 함수 의미를 따른 loop를
  추가하는 것이 최소 변경이다.
- **왜 coverage gate를 별도 `[NEW]` 파일 + 새 seam으로**: `pkg/workflow/gate.go`의 verdict 도출
  패턴(GateResult/VerdictSourceExitCode)을 재사용하되, **exit-code-only `CommandRunner`는 stdout이
  없어 coverage%를 줄 수 없으므로**(gate.go:21) stdout을 반환하는 `[NEW] CoverageRunner.RunOutput`
  seam을 도입한다. gate.go에 합치면 300-line 압박 + concern 혼합 + seam 혼동. 별도 파일이 분할
  전략(by concern)에 부합.
- **왜 coverage_threshold를 schema field로**: parity gate가 schema를 authority로 삼아 derived JS와
  비교하므로, 임계값을 schema에 두면 SoT 단일화 + parity로 drift 차단이 동시에 된다. config의
  `WorkflowConf.CoverageThreshold`는 사용자 override용 보조(omitempty)이며 schema 기본이 권위.
- **왜 team_default 기본 true**: 현행 동작(doctor pass 시 --team이 substrate로 감)을 보존. false 기본은
  silent behavior change이며 Outcome Lock 위반.
- **왜 review barrier에서 security > code-quality**: manual consolidation 규칙(security 우선)과 일치.
  reviewer APPROVE라도 security FAIL이면 barrier로 처리해 데이터/보안 무결성을 지킨다. 이 우선순위는
  `ConsolidateReviewVerdict`의 결정값(Barrier=true, Reason="security_fail")으로 runnable화된다.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go (existing module) | go 1.26 / toolchain go1.26.4 | `autopus-adk/go.mod` (인용) | 2026-06-27 | 신규 의존성 없음 — brownfield, stdlib `regexp`/`encoding/json`만 사용 |

신규 외부 의존성 없음(brownfield Go). techstack-freshness greenfield 요건 미적용. coverage 파싱은
stdlib `regexp`로 충분하고 remediation 결정 함수는 stdlib만 쓰므로 새 라이브러리를 도입하지 않는다.

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "on gate_build_test failure … spawn a fixer and re-run the failed segment, bounded by … MaxRetry=3" | bounded retry loop | `RunGateRemediation` FixerAttempts / SegmentBLaunched | S1 |
| INV-002 | "plus a circuit-breaker on no-progress, before aborting" | no-progress termination | `RunGateRemediation` Aborted / AbortReason="circuit_break_no_progress" | S2 |
| INV-003 | "coverage 84% → gate FAIL / 85% → PASS" | numeric threshold (>=) | `EvaluateCoverageGate` GateResult.Verdict / VerdictSource | S3, S4 |
| INV-004 | "REQUEST_CHANGES triggers … fix + re-review within bounded retries, then abort" | bounded review loop | `RunReviewBarrier` FixerAttempts / Aborted / AbortReason="review_budget_exhausted" / ReleaseHygieneReached | S5 |
| INV-005 | "Security findings priority > code-quality" | priority ordering | `ConsolidateReviewVerdict` Barrier / Reason="security_fail" | S6 |
| INV-006 | "workflow.team_default=false in autopus.yaml → Go config parses field = false" | config unmarshal fidelity | cfg.Workflow.TeamDefault | S7, S10 |
| INV-007 | "default true to preserve current behavior" | default value | DefaultFullConfig().Workflow | S8 |
| INV-008 | "WorkflowConf … Validate()" | range validation | Validate() error on out-of-range threshold | S9 |
| INV-009 | "parity gate must stay fail-closed and extend to any new schema fields" | fail-closed parity | parity error naming diverging element | S11 |
| INV-010 | "regression-0 for codex/antigravity-cli/opencode" + "do NOT change route_a" | regression invariant | route_a parity/render, non-claude emit | S12 |
| INV-011 | "coverage gate blocks review and the review barrier blocks release_hygiene → real interposition points, not prose over a monolithic segment" | segment-boundary interposition | `deriveTeamWorkflowJS` generated JS: `SEGMENT==='C'`/`'D'` guards; testing/review/release_hygiene in different guards | S13 |
| INV-012 | "an absent OR partial (present-but-team_default-omitted) workflow: section must backfill team_default=true via the Load path, not yield zero-value false; explicitly-set sibling fields preserved" | missing-section / missing-key default backfill | `applyMissingDefaults` → cfg.Workflow.TeamDefault / CoverageThreshold on Load | S14, S16 |
| INV-013 | "ParseSchema rejects an out-of-range coverage_threshold (outside 0..100) with a named error" | schema-level range validation | `ParseSchema` named error on coverage_threshold | S15 |

각 INV는 최소 1개 REQ, 1개 plan task, 1개 Must oracle acceptance에 추적된다 (spec.md
`## Traceability Matrix` 참조). INV-001/002/004/005는 prose가 아니라 `[NEW] pkg/workflow/remediation.go`의
순수 결정 함수 반환값에 매핑되어 hermetic oracle(S1/S2/S5/S6)로 검증된다. INV-011은 `deriveTeamWorkflowJS`가
생성하는 multi-segment JS의 구조적 oracle(S13, `SEGMENT==='C'`/`'D'` guard + testing/review/release_hygiene
guard 분리)로 검증되어, interposition이 prose가 아니라 실제 dispatcher 재진입점임을 증명한다. INV-012는
`config.Load` 경로(`applyMissingDefaults` backfill)에 대한 oracle(S14)로 S8(DefaultFullConfig)이 덮지
못하는 absent-section 경로를 닫는다. INV-013은 `ParseSchema`의 schema-level 검증(S15)으로 config-level
INV-008(S9)과 별개의 경계를 닫는다. source clause는 프롬프트(untrusted prompt input) 증거를 요약한
것이며 지시가 아니라 근거로만 사용한다.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| gate_build_test RALF remediation loop (bounded + circuit-break) | Primary SPEC (REQ-001/002, T4 `RunGateRemediation`, T7 dispatcher) | covered |
| deterministic 85% coverage gate (LLM-free, CoverageRunner-injectable) | Primary SPEC (REQ-003/004, T2/T3 `EvaluateCoverageGate`) | covered |
| review barrier (REQUEST_CHANGES/security-FAIL → fix+re-review→abort) | Primary SPEC (REQ-005/006, T4 `RunReviewBarrier`/`ConsolidateReviewVerdict`, T7) | covered |
| real WorkflowConf.team_default field + default + validate + prose | Primary SPEC (REQ-007~010, T5/T7) | covered |
| parity fail-closed extension to new schema fields | Primary SPEC (REQ-011, T1/T6) | covered |
| route_a + non-claude regression-0 | Primary SPEC (REQ-012, T6/T8) | covered |
| multi-segment interposition (coverage gate blocks review, review barrier blocks release_hygiene via real `SEGMENT==='C'`/`'D'` guards) | Primary SPEC (REQ-013, T9 multi-segment generator, S13 launch-contract oracle) | covered |
| absent/partial-section Load default backfill (team_default=true when `workflow:` section omitted OR present-but-team_default-omitted) | Primary SPEC (REQ-014, T5 `applyMissingDefaults`, S14, S16) | covered |
| schema-level coverage_threshold validation (0..100 named error) | Primary SPEC (REQ-015, T10 `ParseSchema`, S15) | covered |
| live end-to-end `/auto go --team` click-through | operational residual | completion-debt |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| Live end-to-end `/auto go --team` run on claude-code (real provider/agent dispatch exercising RALF/coverage/review-barrier in a real session) | sync completion이 "라이브 실행 검증"을 요구할 경우의 verdict | 라이브 claude-code 세션에서 수동 클릭스루. hermetic하게 증명 불가(TEAM-001과 동일 operational residual). 로직 자체는 S1–S12 hermetic oracle(순수 결정 함수 + CoverageRunner seam + config 단위 테스트)로 검증됨 — 라이브 실행은 통합 확인용이지 로직 정확성 게이트가 아니다. |

이 SPEC은 라이브 의존을 줄이기 위해 gate/retry/coverage/review/config를 `[NEW] CoverageRunner` seam,
순수 결정 함수(`RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`), config 단위
테스트로 hermetic oracle화했다. 위 항목 외 Completion Debt는 없다.

## Evolution Ideas

These are optional improvements and do not block sync completion. SPEC ID/task ID/acceptance ID를
부여하지 않으며, 사용자가 명시적으로 요청하기 전까지 follow-up SPEC이나 sibling SPEC을 추천하지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| coverage threshold를 phase별이 아니라 per-package로 세분화 | Outcome Lock은 단일 85% 게이트만 요구 | 사용자가 per-package 게이트를 명시 요청 |
| RALF fixer에 실패 diff 컨텍스트를 자동 주입해 수렴 가속 | bounded loop + circuit-break로 안정성은 이미 충족 | 사용자가 수렴 품질 개선을 요청 |
| route_a에도 coverage gate 도입 | non-goal(route_a floor 유지)에 명시적으로 배제됨 | 사용자가 route_a 강화를 별도 요청 |
| review barrier 결과를 구조화 audit artifact로 영속화 | 현 SPEC은 dispatcher in-session 동작만 요구 | 사용자가 review audit 영속화를 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC가 단일 cohesive change로 Outcome Lock을 닫는다. 의존은 기존 `SPEC-HARNESS-WORKFLOW-TEAM-001`(완료된 substrate) 하나이며 새 sibling 불필요 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `content/workflows/route_team.schema.json` | existing | Read 확인 (8 phase, retry 값) |
| `content/workflows/route_team.md` | existing | Read 확인 (testing line 61, review line 66) |
| `pkg/workflow/schema.go::PhaseDef` / `rawPhase` / `ParseSchema` `DisallowUnknownFields` | existing (edit: ParseSchema에 coverage_threshold 0..100 named-error 검증 추가, REQ-015/T10) | Read 확인 (line 18/38/60); `CoverageThreshold int json:"coverage_threshold"`는 PhaseDef/rawPhase에 이미 존재(line 28/50)하나 `ParseSchema`는 아직 범위 검사 없음(`validateDepthCaps`만 호출, line 92) |
| `pkg/workflow/gate.go::EvaluateGate` / `CommandRunner` / `GateResult` / `VerdictSourceExitCode` | existing | Read 확인 (line 39/17/26/11); `CommandRunner.Run`은 exit code만 반환·**stdout 없음**(line 21) |
| `pkg/workflow/depth.go::MaxRetry/MaxFanOut/MaxVerifyVotes/ResolveDepth/validateDepthCaps` | existing | Read 확인 (line 19-21/27/40) |
| `pkg/workflow/binding.go::QualityBinding/OverlayPhases` | existing | Read 확인 (line 20/30) |
| `pkg/content/workflow_parity.go::checkWorkflowParity/checkPerPhaseQualityParity/phaseJSBlock` | existing | Read 확인 (line 32/101/140) |
| `pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS/writeTeamBaselineComment/writeTeamPhaseBlock` | existing (edit: `deriveTeamWorkflowJS`를 단일 gate-경계 2-segment에서 multi-segment(A/B/C/D)로 확장, REQ-013/T9) | Read 확인 (line 55/244/133); 현재 segment 분할은 `gateBuildTestID`에서 1회만 발생(segA/segB, line 96-121), `writeTeamPhaseBlock`은 `coverage_threshold=%d` token을 이미 emit(line 136-138) |
| `pkg/config/schema.go::HarnessConfig` (no Workflow field) / `Validate` / `Quality.Default` | existing | Read 확인 (line 59-87/231/246) |
| `pkg/config/schema_spec.go::ReviewGateConf.Enabled` | existing | Read 확인 (line 11) |
| `pkg/config/defaults.go::DefaultFullConfig` / `"workflow":10` (CategoryWeights) | existing | Read 확인 (line 38/159) |
| `pkg/config/loader.go::loadConfig` / `applyMissingDefaults` | existing (edit: `applyMissingDefaults`가 `workflow` 섹션 부재 시 `cfg.Workflow = defaults.Workflow`를, 섹션 present-but-team_default-omitted 시 `cfg.Workflow.TeamDefault = true`만 backfill, REQ-014/T5) | Read 확인 (loadConfig line 51→applyMissingDefaults 호출 line 68; applyMissingDefaults는 현재 `design` 섹션만 backfill, line 85-94) |
| `pkg/config/loader_test.go::TestLoad_ExplicitDesignDisabledIsPreserved` + 누락 design 섹션 default 테스트 | existing (edit: S7 explicit-false unmarshal·S14 absent-section backfill·S16 present-but-team_default-omitted backfill 케이스 추가) | rg 확인 (explicit-false line 49; missing-design-default assert line 46) — S7 oracle 선례이자 S14 backfill 패턴의 직접 선례 |
| `pkg/cost/pricing.go::ModelForAgent` / `internal/cli/effort_resolve.go::ResolveEffort` | existing | rg 확인 (line 80/39) |
| `team_default` 참조 4파일 (harness-workflow.md, agent-teams.md, auto-router.md.tmpl, gemini SKILL) | existing | rg 확인 (코드 0건, 문서 4건) |
| `[NEW] pkg/workflow/coverage_gate.go::CoverageRunner (RunOutput → stdout) / EvaluateCoverageGate` | planned addition | 신규 — exit-code-only `CommandRunner`와 구분되는 stdout seam, 정합성 검증 대상 아님 |
| `[NEW] pkg/workflow/remediation.go::RunGateRemediation / ConsolidateReviewVerdict / RunReviewBarrier` | planned addition | 신규 — loop/priority 결정 로직 source of truth |
| `[NEW] pkg/config/schema_workflow.go::WorkflowConf` (TeamDefault, CoverageThreshold, Validate) | planned addition | 신규 |
| `[NEW] pkg/workflow/coverage_gate_test.go`(S3/S4), `pkg/workflow/remediation_test.go`(S1/S2/S5/S6), `pkg/config/schema_workflow_test.go`, `pkg/content/workflow_parity_stability_test.go` | planned addition | 신규 테스트 |
| `route_team.schema.json` `coverage_threshold` field, gate `retry: 2`, review `retry: 2` | planned addition | 신규 schema 값 (DisallowUnknownFields 때문에 struct field 동반 필수) |
| `pkg/content/workflow_launch_contract_test.go::assertSegmentGuards` (route_team) | existing (edit: route_team segment assertion을 multi-segment ≥4 guard·`SEGMENT==='C'`/`'D'`·testing/review/release_hygiene guard 분리로 확장, S13; route_a 2-segment assertion 불변) | Read 확인 (현재 route_team은 정확히 1개 A-guard·1개 B-guard, B 시작=annotation을 단언, line 77-82/183) |

## Revision 1 closure

| Finding | Category | How closed | File(s) where the change landed |
|---------|----------|------------|---------------------------------|
| F-001 | correctness/feasibility (Q-CORR-01/03, Q-FEAS-01/03) | coverage gate가 exit-code-only `CommandRunner`(stdout 없음, gate.go:21) 대신 stdout을 반환하는 `[NEW] CoverageRunner.RunOutput` seam에서 coverage%를 파싱하도록 명시; S3/S4가 `EvaluateCoverageGate(ctx, runner, coverageCmd, 85)` 호출로 새 seam을 검증 | spec.md (REQ-003/004, 생성 파일 상세, Matrix), plan.md (T2/T3, Implementation Strategy), acceptance.md (S3/S4, Oracle Notes), research.md (기존 코드 분석, 설계 결정, Reference Discipline) |
| F-002 | completeness/feasibility (Q-COMP-02/04/05, Q-FEAS-01/03) | RALF loop·circuit-break 제어 흐름을 prose에서 `[NEW] pkg/workflow/remediation.go`의 순수 결정 함수 `RunGateRemediation`로 이동, S1/S2가 그 함수에 대한 hermetic oracle(FixerAttempts 2/1, circuit_break_no_progress) | spec.md (REQ-001/002, Matrix), plan.md (T4/T7), acceptance.md (S1/S2), research.md (INV-001/002, Coverage Map) |
| F-004 | completeness/feasibility (Q-COMP-04/05, Q-FEAS-01/03) | review-barrier·security>code-quality 우선순위를 `[NEW] RunReviewBarrier`/`ConsolidateReviewVerdict` 결정 함수로 runnable화, S5/S6가 정확값(FixerAttempts 2/review_budget_exhausted, Barrier/security_fail) oracle | spec.md (REQ-005/006, Matrix), plan.md (T4/T7), acceptance.md (S5/S6), research.md (INV-004/005, Coverage Map) |
| F-005 | completeness (Q-COMP-06) | FALSE POSITIVE — `## Reviewer Brief`와 `## Self-Verify Summary`가 research.md에 이미 존재·prominent. 재작성 없이 존재만 확인(리뷰어 hedge "downgrade to PASS if present" 충족) | research.md (변경 없음, 존재 확인만) |

## Revision 2 closure

| Finding | Category | How closed | File(s) where the change landed |
|---------|----------|------------|---------------------------------|
| F-002 (multi-segment interposition) | feasibility/completeness/cohesion (Q-FEAS-01, Q-COMP-04, Q-COH-02) | dispatcher contract가 coverage gate가 review를, review barrier가 release_hygiene을 block한다고 약속했으나 generator는 `gate_build_test` 한 경계에서만 분할(segment B가 annotation→testing→review→release_hygiene를 monolithic 실행)해 dispatcher가 testing/review 후 재진입할 수 없었음. `deriveTeamWorkflowJS`를 multi-segment(A/B/C/D)로 확장하도록 REQ-013/T9 추가 — coverage 경계(testing 후)·review 경계(review 후)마다 segment를 끝내 `SEGMENT==='C'`/`'D'` guard 생성, testing/review/release_hygiene가 서로 다른 guard에 위치. S13 launch-contract oracle이 생성 JS의 segment-index 순서(testing<review<release_hygiene)와 C/D guard 존재를 검증. route_a는 별도 generator `deriveWorkflowJS`+coverage/verify phase 부재로 2-segment 유지(regression-0). 결정적 gate JS는 여전히 LLM verdict 없는 boundary marker. | spec.md (REQ-003/005 interposition clause, REQ-013, Outcome Boundary, 생성 파일 상세, Matrix), plan.md (T9, Visual Planning Brief A→B→C→D sequence, Feature Completion Scope), acceptance.md (S13, Oracle Notes), research.md (INV-011, Coverage Map, Reference Discipline) |
| REQ-008 Load-path | completeness (Q-COMP-02, Q-COMP-05) | S8은 `DefaultFullConfig`만 검증하고 `workflow:` 섹션 부재 시의 `config.Load` 경로는 미검증이었음 — `loader.go::loadConfig`가 `applyMissingDefaults`(loader.go:68)로 누락 섹션 default를 채우는데(design 선례) workflow는 backfill되지 않아 absent section이 zero-value `false`로 substrate를 silent 비활성화할 위험. REQ-014/T5로 `applyMissingDefaults`가 `workflow` 부재 시 `cfg.Workflow = defaults.Workflow`를 채우게 명시하고, S14 Load-path oracle(absent section → `cfg.Workflow.TeamDefault==true`/`CoverageThreshold==85`, explicit false는 보존)을 추가. | spec.md (REQ-014, Outcome Boundary, 생성 파일 상세 loader.go row, Matrix), plan.md (T5 Load-path backfill), acceptance.md (S14, Oracle Notes), research.md (INV-012, Coverage Map, Reference Discipline loader.go/loader_test.go rows) |
| Schema-level coverage validation | completeness (Q-COMP-05) | coverage_threshold 범위 검증이 `WorkflowConf.Validate`(config level, S9)에만 있고 schema level에는 없었음 — `ParseSchema`는 `validateDepthCaps`만 호출(schema.go:92)하고 coverage_threshold를 검사하지 않아 out-of-range 값이 derived JS/parity까지 도달 가능. REQ-015/T10으로 `ParseSchema`가 0..100 밖 coverage_threshold를 `coverage_threshold` 명명 named error로 거부하게 추가하고, S15 schema-level oracle(coverage_threshold=150 → error naming coverage_threshold; =85 → ok)을 추가. | spec.md (REQ-015, Outcome Boundary, 생성 파일 상세 schema.go row, Matrix), plan.md (T10 ParseSchema validation), acceptance.md (S15, Oracle Notes), research.md (INV-013, Coverage Map, Reference Discipline schema.go row) |
| Q-COMP-06 recurring | completeness (Q-COMP-06) | provider가 인용한 research.md 발췌(~41줄)가 trim되어 `## Reviewer Brief`/`## Self-Verify Summary`를 반복적으로 missing으로 오탐(실제로는 존재). 두 섹션을 삭제 없이 research.md 최상단(title/intro 직후, `## 기존 코드 분석` 앞)으로 이동해 인용 window 안에 들어오게 함. Traceability Matrix는 REQ-013/014/015·S13/S14/S15·INV-011/012/013을 포함하도록 확장. | research.md (Reviewer Brief·Self-Verify Summary를 상단으로 이동, Self-Verify에 attempt:3 항목 추가), spec.md (Traceability Matrix) |

note: 본 revision은 SPEC document 단계 수정이다. multi-segment generator·`applyMissingDefaults` backfill·`ParseSchema` 검증은 기존 심볼에 대한 edit으로 계획되며 Reference Discipline에 existing-symbol edit으로 기록했다. route_team launch-contract test(`assertSegmentGuards`)의 multi-segment 확장은 TEAM-001 산출물에 대한 후속 구현 작업이고, route_a 2-segment assertion과 doctor version pin(2.1.154)은 불변(regression-0). Status는 draft 유지(review verdict만 승격).

## Revision 3 closure

| Finding | Category | How closed | File |
|---------|----------|-----------|------|
| Partial workflow section (present but team_default omitted) | completeness (Q-COMP-02/04/05, Q-COH-02) | REQ-008/014 default-true invariant was traced only to absent-section (S14) and `DefaultFullConfig` (S8); the present-but-team_default-omitted edge had no oracle. Code already handles it (`applyMissingDefaults` sets `TeamDefault=true` when the present `workflow` map lacks the `team_default` key). Added S16 oracle (`workflow:` present with only `coverage_threshold: 90` → `TeamDefault==true`, `CoverageThreshold==90` preserved), extended REQ-014 to cover absent OR partial sections, and mapped S16 into Traceability Matrix, INV-012, Coverage Map, and Reference Discipline. | spec.md (REQ-014, Matrix), acceptance.md (S16, Oracle Notes), research.md (INV-012, Coverage Map, Reference Discipline loader rows), pkg/config/loader_test.go (S16 oracle strengthened to non-default 90) |
