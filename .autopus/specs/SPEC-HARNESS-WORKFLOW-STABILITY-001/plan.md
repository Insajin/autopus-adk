# SPEC-HARNESS-WORKFLOW-STABILITY-001 구현 계획

## Tasks

- [ ] T1: `route_team.schema.json` 확장 — `gate_build_test`에 `retry: 2` (≤ MaxRetry=3),
      `testing` phase에 `coverage_threshold: 85` 추가, `review` phase `retry`를 0→2로 (review
      barrier 예산). schema는 `*.json`이라 300-line 제한 면제이며, `DisallowUnknownFields` 때문에
      T2의 struct field 추가가 선행/동반되어야 함.
- [ ] T2: `pkg/workflow/schema.go`의 `PhaseDef`와 `rawPhase`에 `CoverageThreshold int json:"coverage_threshold"`
      는 이미 존재하므로 유지 (ParseSchema가 `dec.DisallowUnknownFields()`를 쓰므로 필수). gate
      retry는 기존 `Retry` field로 충분 (validateDepthCaps가 이미 ≤3 enforce). coverage_threshold의
      0..100 범위 검증은 T10에서 `ParseSchema`에 추가한다 (REQ-015). `[NEW]`
      `pkg/workflow/coverage_gate.go`: 기존 exit-code-only `CommandRunner`(stdout 없음)와 **구분되는**
      `[NEW] CoverageRunner interface { RunOutput(ctx, name string, args ...string) (stdout string,
      exitCode int, err error) }` seam을 도입하고, `EvaluateCoverageGate(ctx, runner CoverageRunner,
      coverageCmd []string, threshold int) GateResult`로 stdout에서 coverage%를 파싱·비교해
      exit-code 스타일 verdict 생성. `pkg/workflow/gate.go`의 `GateResult`/`VerdictSourceExitCode`
      재사용. 결정적·LLM-free.
- [ ] T3: coverage 파서 단위 + `[NEW] pkg/workflow/coverage_gate_test.go` — Go `go test -cover`
      출력의 `total: (statements) NN.N%` 라인과 per-package `coverage: NN.N% of statements` 라인
      둘 다에서 백분율을 안전 추출 (stdlib `regexp`, NaN/빈출력 → fail-closed verdict). 84.0 → fail,
      85.0 → pass, 85.0001 → pass 경계를 명시 구현 (S3/S4 oracle).
- [ ] T4: `[NEW] pkg/workflow/remediation.go` + `[NEW] pkg/workflow/remediation_test.go` — loop/priority
      의미의 **source of truth**인 순수·결정적·LLM-free 함수. (a) `RunGateRemediation(budget int,
      evals []GateSignature) GateRemediationDecision` — bounded RALF loop를 주입된 exit signature
      시퀀스에 대해 결정 (FixerAttempts/SegmentBLaunched/Aborted/AbortReason, no-progress circuit-break).
      (b) `ConsolidateReviewVerdict(reviewerApprove, securityFail bool) ConsolidatedVerdict` —
      security FAIL이 reviewer APPROVE를 outrank (Barrier/Reason). (c) `RunReviewBarrier(budget int,
      rounds []ConsolidatedVerdict) ReviewBarrierDecision` — bounded review-fix-re-review loop. agent/LLM
      호출 없음. S1/S2/S5/S6 hermetic oracle: S1 FixerAttempts=2·SegmentBLaunched=true, S2
      Aborted=true·AbortReason=circuit_break_no_progress·FixerAttempts=1, S5 FixerAttempts=2·
      AbortReason=review_budget_exhausted·ReleaseHygieneReached=false, S6 Barrier=true·Reason=security_fail.
- [ ] T5: `[NEW]` `pkg/config/schema_workflow.go` — `WorkflowConf{ TeamDefault bool
      yaml:"team_default"; CoverageThreshold int yaml:"coverage_threshold,omitempty" }` + `Validate()`
      (threshold가 0이면 unset 허용, 1..100만 유효, 그 밖은 named error로 `workflow`+`coverage_threshold`
      명명). `pkg/config/schema.go` `HarnessConfig`에 `Workflow WorkflowConf yaml:"workflow,omitempty"`
      추가하고 `Validate()`에서 `c.Workflow.Validate()` 호출. `pkg/config/defaults.go`
      `DefaultFullConfig`에 `Workflow: WorkflowConf{TeamDefault: true, CoverageThreshold: 85}` 추가.
      **Load-path backfill (REQ-014)**: `pkg/config/loader.go::applyMissingDefaults`가 현재 `design`
      섹션만 backfill하므로(loader.go:85-94), raw map에 `workflow` 키가 없으면
      `cfg.Workflow = defaults.Workflow`를 채우는 분기를 추가한다 — 그래야 `workflow:` 섹션 없는
      autopus.yaml이 zero-value `TeamDefault=false`로 substrate를 silent 비활성화하지 않는다.
      `pkg/config/loader_test.go`에 S7(explicit false)·S14(absent section → true) 케이스를 추가한다.
- [ ] T6: `pkg/content/workflow_parity.go` 확장 — schema phase의 `coverage_threshold`(>0)와 gate
      `retry`를 derived JS block의 token으로 검증. `pkg/content/workflow_generate_team.go`가 해당
      token을 baseline comment에 emit하도록 수정 (예: testing block에 `coverage_threshold=85`,
      gate block에 기존 `retry: N`). route_a는 coverage_threshold/gate-retry 미선언이므로 token
      검사가 발화하지 않아 route_a parity 불변.
- [ ] T7: dispatcher contract prose 확장 (4파일) — `content/skills/harness-workflow.md`
      `### Segmented Dispatch Contract`에 (a) gate fail 시 fixer spawn + 실패 segment 재실행 loop
      (retry ≤ schema gate retry, ≤ MaxRetry=3) + no-progress circuit-break, (b) testing 후
      coverage gate 평가 + fail 시 동일 loop, (c) review REQUEST_CHANGES/security-FAIL → fix +
      re-review (≤ review retry) → 소진 시 abort, security > code-quality 우선순위. **dispatcher는
      이 결정들을 `pkg/workflow.RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`의
      의미를 따라** 실제 fixer/reviewer agent를 spawn한다고 명시한다 — 결정 로직은 runnable Go, agent
      dispatch는 operational layer, gate JS는 LLM verdict 없는 boundary marker. auto-router.md.tmpl의
      `Segment-launch dispatcher contract`(~1402)와 substrate-selection table(~1095)의
      `workflow.team_default` 참조를 실 field 의미로 정정. agent-teams.md + gemini SKILL mirror 동기화.
      `--no-workflow` opt-out과 `team_default=false` opt-out을 둘 다 유효 경로로 명시.
- [ ] T9: `pkg/content/workflow_generate_team.go::deriveTeamWorkflowJS`를 **multi-segment**로 확장
      (REQ-013). 현재는 `gateBuildTestID`에서 단 한 번 분할해 segment A(≤gate) / segment B(>gate)만
      만든다(workflow_generate_team.go:96-121). 이를 일반화해 "다음 phase 앞에서 결정적 dispatcher
      interposition이 필요한 phase 뒤"마다 segment를 끝낸다: (a) `gate_build_test`(기존 경계),
      (b) coverage-gated phase(`CoverageThreshold > 0`, 즉 `testing`), (c) review phase
      (`VerifyVotes > 0`, 즉 `review`). 표준 8-phase route_team은 네 segment가 된다 —
      A={planning, test_scaffold, implementation, gate_build_test}, B={annotation, testing},
      C={review}, D={release_hygiene} — 그리고 SEGMENT 비교 guard를 segment 수만큼(`SEGMENT==='A'`..
      `'D'`) emit한다. `writeTeamPhaseBlock`/baseline comment/parity token 포맷은 불변(분할 경계만
      바뀜). `pkg/content/workflow_launch_contract_test.go`의 route_team `assertSegmentGuards`를
      multi-segment(≥4 guard, `SEGMENT==='C'`/`'D'` 존재, `testing`/`review`/`release_hygiene`가
      서로 다른 guard) assertion으로 확장한다(S13). **route_a는 별도 generator `deriveWorkflowJS`를
      쓰고 coverage/verify phase가 없어 분할 경계가 발화하지 않으므로 2-segment 그대로 유지** —
      `TestLaunchContract_RouteA`의 route_a 2-segment assertion은 손대지 않는다(regression-0).
- [ ] T10: `pkg/workflow/schema.go::ParseSchema`에 `coverage_threshold` 범위 검증을 추가(REQ-015).
      현재 `ParseSchema`(schema.go:59)는 `isSafePhaseID`/`isSafeAgentModel`/`isSafeEffort`/
      `validateDepthCaps`/`isSafeResultType`로 fail-closed하지만 `CoverageThreshold`는 검사하지
      않는다. `rp.CoverageThreshold`가 0..100 밖이면 `coverage_threshold`를 명명한 named error를
      반환하는 분기를 추가한다(예: `phase %q coverage_threshold %d out of range [0,100]`). 0은 unset로
      허용. `pkg/workflow`의 schema 파싱 테스트에 coverage_threshold=150 → error, =85 → ok 케이스를
      추가한다(S15).
- [ ] T8: 검증 — `go build ./... && go vet ./... && gofmt -l pkg/ internal/ && go test -race
      ./pkg/workflow/... ./pkg/config/... ./pkg/content/... ./internal/cli/...`. 신규/확장 oracle:
      `pkg/content/workflow_launch_contract_test.go`(S13 multi-segment), `pkg/config/loader_test.go`
      (S14 absent-section Load default), `pkg/workflow` schema 파싱(S15 coverage range). `auto workflow
      render --route team`이 새 gate/retry와 multi-segment guard를 반영하는지 확인. 모든 새 `.go`
      ≤ 300 lines 확인. `auto generate-templates` 후 parity green 확인.

## Implementation Strategy

- **결정성 경계 + 결정 로직의 source of truth**: coverage gate는 stdout을 반환하는 `[NEW]
  CoverageRunner` seam으로 LLM-free·주입 가능하게 만든다. RALF/review-barrier/security-priority
  **결정 로직은 prose가 아니라 `[NEW] pkg/workflow/remediation.go`의 순수 함수**(`RunGateRemediation`/
  `RunReviewBarrier`/`ConsolidateReviewVerdict`)가 source of truth이며 단위 테스트(S1/S2/S5/S6)로
  검증한다. dispatcher contract(prose)는 그 함수 의미를 따라 실제 agent를 spawn하는 operational
  layer이고, 결정적 gate JS에는 LLM verdict를 embed하지 않는다(하드 제약 유지).
- **새 seam vs 기존 seam**: 기존 `pkg/workflow/gate.go`의 `CommandRunner.Run(ctx, name, args...)
  (exitCode int, err error)`은 **exit code만 반환하고 stdout이 없다** → coverage% 파싱에 쓸 수 없다.
  그래서 stdout을 반환하는 별도 `[NEW] CoverageRunner.RunOutput`을 도입하고 기존 `CommandRunner`/
  `EvaluateGate`는 그대로 둔다(regression-0).
- **DisallowUnknownFields 함정**: `ParseSchema`(schema.go:60)가 unknown field를 거부하므로 schema
  JSON에 새 key를 넣기 전에 `PhaseDef`+`rawPhase` struct field를 반드시 먼저 추가한다. 이 순서를
  어기면 모든 workflow 명령이 깨진다. T1/T2를 같은 변경 단위로 묶는다.
- **import 경계**: `pkg/workflow`는 `internal/cli`를 import하지 않는다(schema.go:5). coverage gate와
  remediation 결정 함수는 모두 `pkg/workflow`에 두고(순수 Go), CLI wiring은 `internal/cli`에서
  주입한다. quality→(model,effort) binding은 기존 `pkg/workflow/binding.go::QualityBinding`로 주입
  (변경 없음).
- **기존 cap 재사용**: gate/review retry는 `validateDepthCaps`가 이미 `MaxRetry=3`으로 enforce.
  fan_out≤5, votes≤3 불변. 새 cap을 만들지 않는다. remediation 함수의 budget 인자도 schema gate/review
  retry(≤MaxRetry)에서 유래한다.
- **parity fail-closed 우선**: 새 schema field는 반드시 parity token 검사에 추가한다. 추가 없이
  schema만 바꾸면 derived JS drift가 silent로 통과할 수 있다.
- **파일 분할**: coverage gate 로직, remediation 결정 로직, config workflow 로직을 각각 별도 `[NEW]`
  파일로 분리해 300-line 제한과 concern 분리를 동시에 만족.

## Visual Planning Brief

### Command-flow — `/auto go SPEC-ID --team` substrate selection + RALF loop (after this SPEC)

```
auto go --team (claude-code)
        │
        ▼
auto workflow doctor ──fail──▶ fallback-class=fail-fast → Route A (unchanged)
        │ pass
        ▼
read autopus.yaml → WorkflowConf.TeamDefault   ◀── REQ-007/008/010 (was phantom key)
        │ team_default=false OR --no-workflow ──▶ Agent Teams (pre-route opt-out, unchanged)
        │ team_default=true (default)
        ▼
route_team substrate (8 phases, multi-segment dispatch A/B/C/D)
```

### Sequence — A→B→C→D segment launches with deterministic interposition between them

The generator (T9, REQ-013) splits route_team into four segment guards so the dispatcher REGAINS
control between segments and interposes each gate. A monolithic post-gate segment would give the prose
no re-entry point — that is the F-002 defect this fixes.

```
Dispatcher (main session)         Decision logic (pkg/workflow, pure Go)        Fixer/Reviewer (agents)
   │ launch SEGMENT 'A' ──────────▶  planning..gate_build_test (boundary marker)      │
   │ auto workflow gate ──────────▶  EvaluateGate → GateSignature{BuildExit,TestExit} │
   │ RunGateRemediation(budget,evals) → {FixerAttempts, SegmentBLaunched, Aborted}    │
   │   verdict fail + progress ─────────────────────────────────────────▶ spawn fixer  (REQ-001)
   │   same signature twice ──▶ Aborted=true, AbortReason=circuit_break_no_progress     (REQ-002)
   │ ── gate pass ──
   │ launch SEGMENT 'B' ──────────▶  annotation, testing                               │
   │ coverage gate ───────────────▶  EvaluateCoverageGate(CoverageRunner stdout, 85)  │
   │   84% → verdict fail → fixer loop ; 85% → verdict pass  (REQ-003/004)            │
   │ ── coverage pass ── (interposition because testing/review are in DIFFERENT guards, S13)
   │ launch SEGMENT 'C' ──────────▶  review (reviewer + security-auditor)             │
   │   ConsolidateReviewVerdict(approve, securityFail) → {Barrier, Reason}  (REQ-006) │
   │     security FAIL ⇒ Reason=security_fail outranks reviewer APPROVE                │
   │   RunReviewBarrier(budget,rounds) → {FixerAttempts, Aborted, ReleaseHygieneReached}
   │     barrier ─────────────────────────────────────────────────────▶ spawn fixer  (REQ-005)
   │     budget exhausted ──▶ Aborted=true, AbortReason=review_budget_exhausted        │
   │ ── review barrier clear ── (interposition because review/release_hygiene differ, S13)
   │ launch SEGMENT 'D' ──────────▶  release_hygiene marker                            │
   │ auto check --hygiene --arch --quiet --staged                                     │
```

## Feature Completion Scope

Primary SPEC가 Outcome Lock을 단일 cohesive change로 닫는다: 네 reinforcement(RALF retry,
coverage gate, review barrier, 실 config field)가 모두 이 SPEC의 task T1–T10에 들어 있고, 각각
oracle acceptance로 검증된다. **coverage gate와 review barrier의 interposition은 prose가 아니라
T9의 multi-segment generator가 만드는 실제 segment 경계(`SEGMENT==='C'`/`'D'`)로 보장되며 S13
launch-contract oracle로 닫힌다** — 이것이 F-002(monolithic post-gate segment라 dispatcher가
재진입할 수 없었던 결함)를 종결한다. RALF/review-barrier/security-priority 결정 로직은 T4의 순수
함수가 source of truth이며 S1/S2/S5/S6 단위 테스트로 닫힌다. config substrate는 explicit
unmarshal(S7)·DefaultFullConfig 기본값(S8)·Load-path backfill(S14, T5)·config-level 검증(S9)·
schema-level 검증(S15, T10)으로 모두 닫는다. 승인된 sibling 의존성은
`SPEC-HARNESS-WORKFLOW-TEAM-001`(route_team substrate 제공) 하나뿐이며 새 sibling을 만들지 않는다.

**Completion Debt (sync blocker 후보)**: 실 end-to-end `/auto go --team` 라이브 실행은 라이브
provider/agent 호출이 필요해 hermetic하게 증명할 수 없다(TEAM-001과 동일한 operational residual).
이 SPEC은 gate/retry/coverage/review/config 로직을 **CoverageRunner seam·순수 결정 함수·config
단위 테스트로 hermetic oracle화**(S1–S15)하여 라이브 실행 의존을 제거했다. 라이브 클릭스루는
operational residual로 남으며 research.md `## Completion Debt`에 기록한다.
