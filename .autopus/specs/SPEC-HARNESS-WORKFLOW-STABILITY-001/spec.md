# SPEC-HARNESS-WORKFLOW-STABILITY-001: route_team 안정성을 manual pipeline 수준으로 끌어올리기

**Status**: completed
**Created**: 2026-06-27
**Domain**: HARNESS

## 목적

claude-code에서 `auto workflow doctor`가 통과하면 `/auto go --team`은 결정적 `route_team`
workflow substrate(8 phase)로 서빙된다. 이 substrate는 전문 에이전트(planner+PLAN_SCHEMA,
tester, executor×N worktree fan-out, exit-code gate, annotator, tester, reviewer+security-auditor,
release_hygiene)를 충실히 dispatch하지만, manual subagent pipeline(`content/skills/agent-pipeline.md`,
auto-router Route A)보다 **얇다(thinner)**. 네 가지 안정성 격차가 있다:

1. gate_build_test 실패 시 RALF remediation loop가 없다 (`retry: 0` → 즉시 abort).
2. 85% coverage gate가 없다 (testing phase는 "tests 실행/검증"만 함).
3. review가 advisory다 — reviewer/security-auditor가 돌지만 APPROVE/REQUEST_CHANGES에 아무도 반응하지 않는다.
4. `workflow.team_default`이 phantom config key다 — 4개 문서가 `autopus.yaml` knob처럼 참조하지만
   `pkg/config/schema.go`의 `HarnessConfig`에는 `Workflow` field가 없어 실제로는 무시된다.

이 SPEC은 위 네 격차를 닫아 `route_team`을 manual pipeline과 **동등한 안정성**으로 만든다.
RALF/review-barrier/coverage 결정 로직은 `pkg/workflow`의 순수·결정적·LLM-free Go 함수에 두어
단위 테스트로 검증하고, dispatcher(main session)는 그 함수 의미를 따라 실제 fixer/reviewer agent를
spawn한다. 결정적 gate JS는 LLM verdict 없는 boundary marker로 유지한다.

## Outcome Boundary

- **Outcome Lock**: claude-code(doctor pass)에서 `/auto go --team`이 manual-pipeline-equivalent
  stability로 `route_team`을 실행한다 — gate/build/test 실패와 review REQUEST_CHANGES가
  hard abort가 아니라 bounded remediation loop를 트리거하고, 결정적 85% coverage gate가 enforce되며,
  `workflow.team_default`이 실제 validated config knob이다. 이 안정성은 prose가 아니라 **실제
  dispatcher interposition point**로 보장된다: route_team generator가 multi-segment(A/B/C/D)를
  생성해 coverage gate가 `testing`과 `review` 사이에서, review barrier가 `review`와
  `release_hygiene` 사이에서 결정적으로 끼어든다.
- **Mandatory requirements**: REQ-001 ~ REQ-010, REQ-013(multi-segment interposition),
  REQ-014(absent-section Load default), REQ-015(schema-level coverage validation).
- **Explicit non-goals**:
  - route_a(얇은 4-phase floor)를 바꾸지 않는다 (route_a는 별도 generator `deriveWorkflowJS`를
    쓰며 coverage/verify phase가 없어 2-segment 그대로 유지, regression-0).
  - `--team` 없는 plain `/auto go`가 workflow로 default되게 하지 않는다.
  - route_team에 Context7 doc-fetch나 Phase 3.5 UX-verify를 추가하지 않는다.
  - non-claude 플랫폼(codex/antigravity-cli/opencode) 동작을 바꾸지 않는다 (regression-0).
  - 결정적 gate JS에 LLM verdict를 embed하지 않는다 (segment boundary marker로만 유지).
  - `auto workflow doctor` version pin(`2.1.154`)을 바꾸지 않는다.
- **Completion evidence**:
  - `route_team.schema.json`에 gate retry / coverage threshold field 추가 + parity gate green.
  - route_team generator가 multi-segment(A/B/C/D)를 생성해 `testing`/`review`/`release_hygiene`이
    서로 다른 segment guard에 들어가 dispatcher가 coverage gate·review barrier를 interpose할 수 있음
    (launch-contract oracle S13).
  - dispatcher contract가 `content/skills/harness-workflow.md` + auto-router(+ gemini mirror)에서
    RALF/coverage/review-barrier로 업데이트되고 A→B→C→D launch 순서로 일관됨.
  - 새 `WorkflowConf` + 통과하는 config 테스트(`team_default=false` 실제 unmarshal, 그리고 `workflow:`
    섹션 부재 시 `applyMissingDefaults`가 `TeamDefault=true`/`CoverageThreshold=85`로 backfill).
  - `pkg/workflow`의 순수 결정 함수(`RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`)와
    `EvaluateCoverageGate` + 통과하는 단위 테스트(S1–S6 oracle).
  - `ParseSchema`가 0..100 밖 `coverage_threshold`를 named error로 거부 (schema-level oracle S15).
  - `auto workflow render --route team`이 새 gate/retry를 반영.
  - `go build/vet/gofmt/-race` green, 모든 새 `.go` ≤ 300 lines.

## Requirements

### REQ-001 — gate_build_test RALF remediation (retry budget)
WHEN the `gate_build_test` deterministic gate returns `verdict: fail` during a `route_team` run,
THE SYSTEM SHALL spawn a fixer (executor) agent and re-run the failed segment following the bounded
decision computed by the new `pkg/workflow.RunGateRemediation` function, whose retry budget is capped
by `pkg/workflow/depth.go` `MaxRetry` (3), and abort once that budget is spent.
Priority: Must

### REQ-002 — gate remediation circuit-breaker
WHILE the dispatcher is remediating a failed `gate_build_test` verdict,
THE SYSTEM SHALL abort the loop when two consecutive gate evaluations produce the same build/test
exit-code signature, as decided by `pkg/workflow.RunGateRemediation` returning `Aborted=true` with
`AbortReason="circuit_break_no_progress"`, even when the retry budget is not yet exhausted.
Priority: Must

### REQ-003 — deterministic 85% coverage gate at a real segment boundary
WHEN the `testing` phase completes during a `route_team` run,
THE SYSTEM SHALL regain dispatcher control at the segment boundary that ends after `testing` (the
coverage-gated phase) and evaluate the new deterministic `pkg/workflow.EvaluateCoverageGate`, which
parses the measured coverage percentage, compares it to a schema-declared threshold (default 85), and
yields `verdict: pass` only when the measured percentage is greater than or equal to the threshold,
reusing the `GateResult` and `VerdictSourceExitCode` exit-code adjudication from `pkg/workflow/gate.go`.
THE SYSTEM SHALL place `testing` and `review` in different segment guards (see REQ-013) so this
interposition is a real re-entry point, not a prose claim over a monolithic post-gate segment.
Priority: Must

### REQ-004 — coverage gate is LLM-free and uses a stdout-returning seam
WHERE the coverage gate runs,
THE SYSTEM SHALL derive its verdict only from a coverage percentage parsed from the stdout supplied by
the new `CoverageRunner.RunOutput` seam — distinct from the exit-code-only `pkg/workflow/gate.go`
`CommandRunner`, which returns no stdout — never from an LLM verdict.
Priority: Must

### REQ-005 — review barrier on REQUEST_CHANGES at a real segment boundary
WHEN the `review` phase reviewer returns REQUEST_CHANGES during a `route_team` run,
THE SYSTEM SHALL regain dispatcher control at the segment boundary that ends after `review`, then
spawn an executor to fix the findings and re-run the review following the bounded decision computed by
the new `pkg/workflow.RunReviewBarrier` function, whose review retry budget is capped by `MaxRetry`,
then abort with `AbortReason="review_budget_exhausted"` once the budget is spent.
THE SYSTEM SHALL place `review` and `release_hygiene` in different segment guards (see REQ-013) so the
barrier runs before `release_hygiene`, not after a monolithic segment has already reached it.
Priority: Must

### REQ-006 — security finding priority over code-quality
WHEN both the reviewer and the security-auditor produce findings during the `review` phase,
THE SYSTEM SHALL consolidate the verdict through the new `pkg/workflow.ConsolidateReviewVerdict`
function so a security FAIL yields `Barrier=true` with `Reason="security_fail"` and outranks a
code-quality REQUEST_CHANGES (security > code-quality).
Priority: Must

### REQ-007 — real WorkflowConf.team_default field
THE SYSTEM SHALL define a `WorkflowConf` struct on `HarnessConfig` with a `TeamDefault bool`
field tagged `yaml:"team_default"`, so `workflow.team_default` in `autopus.yaml` unmarshals into a
real Go field instead of being silently ignored.
Priority: Must

### REQ-008 — team_default default true preserves behavior
THE SYSTEM SHALL default `WorkflowConf.TeamDefault` to `true` in `DefaultFullConfig`, so existing
installs that do not set `workflow.team_default` retain the current `--team` substrate behavior.
Priority: Must

### REQ-009 — WorkflowConf validation
WHEN `HarnessConfig.Validate` runs,
THE SYSTEM SHALL validate the `Workflow` section so an out-of-range coverage threshold (outside
0..100) is rejected with a named error.
Priority: Should

### REQ-010 — substrate-selection prose reads the real field
THE SYSTEM SHALL update the substrate-selection prose in `content/skills/harness-workflow.md`,
`content/skills/agent-teams.md`, `templates/claude/commands/auto-router.md.tmpl`, and
`templates/gemini/skills/agent-teams/SKILL.md.tmpl` so the documented `workflow.team_default=false`
opt-out maps to the real `WorkflowConf.TeamDefault` field.
Priority: Must

### REQ-011 — parity gate extends to new schema fields, fail-closed
WHEN `route_team.schema.json` declares new gate retry or coverage-threshold fields,
THE SYSTEM SHALL extend the parity gate (`pkg/content/workflow_parity.go`) to compare those values
as tokens in the derived JS and fail closed (naming the diverging element) on any drift.
Priority: Must

### REQ-012 — regression-0 for non-claude platforms and route_a
THE SYSTEM SHALL leave route_a phase order, the `auto workflow doctor` version pin (`2.1.154`), the
required/advisory primitive classification, and codex/antigravity-cli/opencode behavior unchanged.
Priority: Must

### REQ-013 — route_team generator is multi-segment so gates have real interposition points
WHEN `deriveTeamWorkflowJS` generates the `route_team` workflow JS,
THE SYSTEM SHALL end a segment after every phase that needs a deterministic dispatcher interposition
before the next phase (the build/test gate `gate_build_test`, the coverage-gated phase where
`coverage_threshold > 0` which is `testing`, and the review phase where `verify_votes > 0` which is
`review`) and emit one `SEGMENT === '<letter>'` guard per segment, so that for the standard 8-phase
`route_team` schema the generator produces four segments with `testing`, `review`, and
`release_hygiene` each in a different guard (including `SEGMENT === 'C'` and `SEGMENT === 'D'` guards),
giving the dispatcher real re-entry points to interpose the coverage gate and the review barrier.
The four segments are A (planning, test_scaffold, implementation, gate_build_test), B (annotation,
testing), C (review), and D (release_hygiene).
Priority: Must

### REQ-014 — absent or partial workflow section backfills team_default via the Load path
WHEN `config.Load` reads an `autopus.yaml` whose `workflow:` section is absent, or is present but omits the `team_default` field, THE SYSTEM SHALL backfill `team_default` to `true` through `pkg/config/loader.go::applyMissingDefaults`.
An absent section backfills the whole `WorkflowConf{TeamDefault: true, CoverageThreshold: 85}` (the same missing-key mechanism that already backfills `design`); a present section that omits `team_default` backfills only that field to `true` while preserving its other explicitly-set fields. Neither an absent nor a partial section may yield a zero-value `false` that would silently disable the `--team` substrate.
Priority: Must

### REQ-015 — schema-level coverage_threshold validation in ParseSchema
WHEN `pkg/workflow/schema.go::ParseSchema` parses a phase that declares `coverage_threshold`,
THE SYSTEM SHALL reject a value outside the 0..100 range with a named error identifying
`coverage_threshold`, alongside the existing `validateDepthCaps` fail-closed checks, so an
out-of-range schema value cannot reach the derived JS or the parity gate.
Priority: Must

## 생성 파일 상세

| 파일 | 역할 | New/Existing |
|------|------|--------------|
| `content/workflows/route_team.schema.json` | gate retry + coverage_threshold field 추가 | Existing (edit) |
| `content/workflows/route_team.md` | testing/review/gate prose에 RALF/coverage/barrier 계약 추가 | Existing (edit) |
| `pkg/workflow/schema.go` | `PhaseDef`/`rawPhase`의 `coverage_threshold` field(이미 존재) + `ParseSchema`에 0..100 범위 검증 추가 (REQ-015, named error) | Existing (edit) |
| `pkg/workflow/coverage_gate.go` | `CoverageRunner.RunOutput`(stdout 반환) seam + coverage% 파싱 + threshold 비교 → `GateResult` exit-code verdict | `[NEW]` |
| `pkg/workflow/remediation.go` | RALF/review-barrier/security-priority 순수 결정 함수 (`RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`) — loop/priority 의미의 source of truth | `[NEW]` |
| `pkg/content/workflow_parity.go` | 새 schema field token parity 확장 | Existing (edit) |
| `pkg/content/workflow_generate_team.go` | `deriveTeamWorkflowJS`를 multi-segment(A/B/C/D)로 확장 — coverage 경계(testing 후)·review 경계(review 후)에서 segment를 끝내 `SEGMENT==='C'`/`'D'` guard 생성 (REQ-013); 새 token을 derived JS comment에 emit (coverage_threshold token은 이미 emit) | Existing (edit) |
| `pkg/config/schema_workflow.go` | `WorkflowConf` struct + `Validate` | `[NEW]` |
| `pkg/config/schema.go` | `HarnessConfig`에 `Workflow WorkflowConf` field + Validate 호출 | Existing (edit) |
| `pkg/config/defaults.go` | `DefaultFullConfig`에 `Workflow: WorkflowConf{TeamDefault: true, CoverageThreshold: 85}` | Existing (edit) |
| `pkg/config/loader.go` | `applyMissingDefaults`가 `workflow:` 섹션 부재 시 `cfg.Workflow = defaults.Workflow`로 backfill (REQ-014, design 섹션 backfill와 동일 패턴) | Existing (edit) |
| `content/skills/harness-workflow.md` | dispatcher contract: A→B→C→D launch 순서 + RALF loop, coverage gate(B 후), review barrier(C 후) — `segment` enum과 dispatcher sequence prose 정합화 (remediation 함수 의미 준수) | Existing (edit) |
| `content/skills/agent-teams.md` | team_default → 실 field 참조 | Existing (edit) |
| `templates/claude/commands/auto-router.md.tmpl` | segmented dispatch contract 동일 확장 | Existing (edit) |
| `templates/gemini/skills/agent-teams/SKILL.md.tmpl` | gemini mirror 동기화 | Existing (edit) |

테스트 파일(모두 `[NEW]`):
`pkg/workflow/coverage_gate_test.go`(S3/S4), `pkg/workflow/remediation_test.go`(S1/S2/S5/S6),
`pkg/config/schema_workflow_test.go`, `pkg/content/workflow_parity_stability_test.go`. 기존
테스트 확장: `pkg/config/loader_test.go`에 `team_default` unmarshal 케이스(S7)와 `workflow:` 섹션
부재 시 Load-path backfill 케이스(S14)를 추가하고, `pkg/content/workflow_launch_contract_test.go`의
route_team segment assertion을 multi-segment(`SEGMENT==='C'`/`'D'` guard, S13)로 확장한다 (route_a
2-segment assertion은 불변). `pkg/workflow/schema.go`의 0..100 coverage_threshold 검증(S15)은
`pkg/workflow`의 schema 파싱 테스트에서 검증한다.

## Related SPECs

이것은 sibling lineage WORKFLOW-001 / TEAM-001 / RUNTIME-001 / FIDELITY-001의 **follow-up**이며,
`SPEC-HARNESS-WORKFLOW-TEAM-001`에 **hard-depend**한다 (route_team substrate + segmented dispatch
contract가 TEAM-001 산출물). 이 SPEC은 그 substrate를 확장만 하며 새 sibling SPEC을 만들지 않는다.

- `SPEC-HARNESS-WORKFLOW-TEAM-001` — route_team substrate, segmented dispatch, parity gate (의존 대상)
- `SPEC-HARNESS-WORKFLOW-FIDELITY-001` — fan-out degenerate floor, planner ownership
- `SPEC-HARNESS-WORKFLOW-RUNTIME-001` — workflow runtime/doctor
- `SPEC-HARNESS-WORKFLOW-001` — route_a floor

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 (gate RALF retry) | T1, T4, T7 | S1 | INV-001 |
| REQ-002 (circuit-breaker) | T4, T7 | S2 | INV-002 |
| REQ-003 (85% coverage gate at segment boundary) | T2, T3, T9 | S3, S4, S13 | INV-003, INV-011 |
| REQ-004 (coverage LLM-free, CoverageRunner seam) | T2 | S3, S4 | INV-003 |
| REQ-005 (review barrier at segment boundary) | T4, T7, T9 | S5, S13 | INV-004, INV-011 |
| REQ-006 (security priority) | T4, T7 | S6 | INV-005 |
| REQ-007 (WorkflowConf field) | T5 | S7 | INV-006 |
| REQ-008 (default true) | T5 | S8 | INV-007 |
| REQ-009 (config-level validation) | T5 | S9 | INV-008 |
| REQ-010 (prose reads field) | T7 | S10 | INV-006 |
| REQ-011 (parity fail-closed) | T1, T6 | S11 | INV-009 |
| REQ-012 (regression-0) | T6, T8 | S12 | INV-010 |
| REQ-013 (multi-segment interposition) | T9 | S13 | INV-011 |
| REQ-014 (absent/partial-section Load default) | T5 | S14, S16 | INV-012 |
| REQ-015 (schema-level coverage validation) | T2, T10 | S15 | INV-013 |

## Completion Verdict

_Synced 2026-06-29 via `/auto sync --auto --loop`. The go pipeline's self-report
was not trusted; the sync re-ran the build/test/format gates directly._

- **Outcome Lock**: satisfied — `route_team` gains manual-pipeline-equivalent stability
  through real dispatcher interposition points (multi-segment A/B/C/D generator), not prose:
  bounded RALF gate remediation + circuit-break, deterministic 85% coverage gate, security>code
  review barrier, and a real validated `WorkflowConf.TeamDefault` config knob.
- **Mandatory requirements**: 13/13 (REQ-001~010, REQ-013, REQ-014, REQ-015) covered per the
  research.md `## Feature Coverage Map` (all `covered`). Safety invariants REQ-011 (parity
  fail-closed) and REQ-012 (regression-0) held.
- **Must acceptance**: 16/16 (S1~S16) — hermetic. S1/S2/S5/S6 close on the pure decision
  functions (`RunGateRemediation`/`RunReviewBarrier`/`ConsolidateReviewVerdict`), S3/S4 on
  `EvaluateCoverageGate` via the `CoverageRunner.RunOutput` stdout seam, S7~S10/S14/S16 on the
  config unmarshal + `applyMissingDefaults` Load-path backfill, S11 on the extended parity gate,
  S13 on the multi-segment generator (`SEGMENT==='C'`/`'D'` guards), S12 regression-0, S15 on
  `ParseSchema` range validation.
- **Completion Debt**: none blocking. The single residual — a live end-to-end `/auto go --team`
  click-through on a real claude-code session exercising RALF/coverage/review-barrier with real
  provider dispatch — is an **operational residual** (same class as TEAM-001), not a logic gate.
  The decision logic is fully hermetically verified by S1~S12; the live run is integration
  confirmation only and is NOT claimed as having been performed.
- **Evolution Ideas**: surfaced as optional, not scheduled (per-package coverage, fixer diff
  injection, route_a coverage gate, review-audit persistence). No follow-up/sibling SPEC created.

### Sync evidence (re-run, not trusted from go)

- `go build ./...` → 0, `go vet ./...` → 0, `gofmt -l <changed .go>` → clean.
- `go test -race -cover ./pkg/workflow/... ./pkg/config/... ./pkg/content/...` → all `ok`,
  coverage pkg/workflow **89.2%** / pkg/config **87.8%** / pkg/content **94.4%** (all ≥ the 85%
  gate this SPEC introduces).
- Full module `go test ./...`: only `pkg/qa/run::TestGUIPolicyRuntimeBlocksStoppedActions` fails,
  on a missing `journey_graph` GUI-harness artifact — pre-existing and unrelated (pkg/qa/run
  references no STABILITY-001 symbol; not in this change set). Regression-0 holds.
- `review.md` Verdict **PASS**; all 6 review findings (F-001~F-006) `resolved`.
- All changed/new `.go` ≤ 300 lines (max `pkg/config/schema.go` = 298). `@AX: no-op`
  (small pure-function files, fan-in < 3 → no mandatory ANCHOR/WARN trigger).
