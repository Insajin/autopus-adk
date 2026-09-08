# SPEC-OMP-007: OMP context promotion oracle — 18.x에서도 유효한 compaction 효과 측정

---
id: SPEC-OMP-007
title: OMP context promotion oracle — 18.x에서도 유효한 compaction 효과 측정
version: 0.1.0
status: draft
priority: HIGH
created: 2026-09-08
domain: OMP
---

## Purpose

릴리즈 핀은 omp/17.2.7에 머물러 있고, 그 이유는 하네스 결함이 아니라 측정 결과다. 같은 plan generator, 같은 20 task pairs, 같은 gateway와 모델(`openai-codex/gpt-5.6-sol`)로 바이너리만 바꿔 돌린 cohort는 omp/17.2.7에서 compaction 8회·median 2309–2335 bp(A23–A28 서명 evidence 5건), omp/18.1.5에서 2회·0 bp, omp/18.1.13에서 2회·895 bp를 냈고 나머지 13개 gate는 모두 통과했다(`docs/runbooks/omp-pin-advance.md` "Measured 2026-09-07", `scripts/release-tools/advance-omp-pin.sh:75-97`).

서명 evidence가 주장하는 것은 정확히 이것이다: "`compaction.methodOrder: [snapcompact]` 오버레이 아래에서 reused call마다 manual `compact`를 호출하면, provider가 청구한 primary input tokens가 20 pair의 median에서 2000 bp 이상 준다"(`pkg/promptlayer/omp_context_promotion_report_cohort.go:26-32`, `omp_context_canary_reduce.go:59,117-123`, `internal/cli/workflow_context_runtime_product_overlay.go:151`). 정적 분석(research.md)은 세 가지를 확정했다. (1) 측정원은 이미 provider 청구량이다 — `get_session_stats.tokens.input+cacheRead+cacheWrite`의 prompt 전후 delta(`pipeline_omp_context_active_usage.go:44-57`, `_rpc_execute.go:68-121`)이며 두 바이너리 모두 assistant message `usage`를 합산한다. 따라서 원격(provider-native) 감축이 B 세션에서 일어났다면 지금 metric에도 보였을 것이고, "local 측정이 remote 감축을 못 본다"는 가설은 성립하지 않는다. (2) 대신 하네스가 방법을 강제한다: 오버레이가 `remote`를 제거하고 18.1.13의 `compact()`는 `methodOrder`에 있는 방법만 고르며(binary line 1311157-1311183), 기본 순서는 `[remote, snapcompact, handoff, shake, soft]`이고 `openai`/`openai-codex`는 remote 대상이다(line 1038206, 986204). (3) 18.0.9의 "skip inflating snapcompact results"(PR #10024)가 projection 거부(`ue >= pe` → `snapcompact would not reduce context locally.`, line 1311284-1311296)를 추가했고, 18.1.x는 같은 workload에서 18회 시도 중 2회만 compaction한다(17.2.7은 8회; 나머지 16회 거부의 클래스 분포는 probe가 확정한다). 이 거부는 provider 청구량이 아니라 OMP 로컬 tokenizer 추정치끼리의 비교다.

동시에 현재 oracle 자체의 취약점이 드러났다. A28 evidence의 20 pair 값은 `[0,0,438,338,2533,2137,4030,3550,5172,4674]×2`이고 median 2335는 각 segment의 5·6번째 pair가 결정한다. 처음 네 pair는 history가 너무 작아 어떤 버전도 compaction을 못 하므로 구조적으로 0–438 bp에 묶이고, 다섯 cohort의 floor 여유는 309–335 bp뿐이다. compaction 시점이 한 call만 늦어도 실제 효과와 무관하게 floor를 놓친다. 반면 같은 evidence의 segment 누적 청구량은 A 1,524,105 대 B 1,040,415 → 3174 bp다. 더 넓게 보면 같은 omp/17.2.7로 만든 23개 evidence tag의 median은 v0.50.93–97에서 7084–7305 bp, v0.50.98부터 2556, v0.50.101–108에서 3308–3310, v0.50.110부터 2309–2335 bp로 움직였다 — OMP는 그대로였고 harness 구현과 workload prompt 크기만 바뀌었다(research.md 실측 표). floor 여유가 3점대로 준 것은 OMP가 아니라 harness·workload 변화의 결과다.

이 SPEC은 floor(2000 bp)를 낮추지 않는다. 대신 (a) evidence가 무엇을 주장하는지를 "OMP가 설정된 compaction chain으로 낸 유효(provider 청구) 감축"으로 바꾸고, (b) 그 감축을 segment 누적 청구량으로 집계하며, (c) 방법·거부 사유를 관측 사실로 남기고, (d) 과거 A22–A28 v1 evidence 검증을 바이트 단위로 보존하고, (e) oracle 코드를 바꾸기 전에 한 번의 probe cohort로 가설을 판정한다.

## Outcome Boundary

- **Outcome Lock**: promotion evidence v2가 "OMP의 설정된 compaction chain(`[remote, snapcompact]`)이 10-call segment의 provider 청구 primary input을 20% 이상 줄였다"를 attest하고, 이 oracle 아래에서 실제로 줄이는 버전은 통과하고 줄이지 않는 버전은 여전히 실패하며, 과거 v1 evidence 23건은 변경 전후 동일하게 검증되고, `auto doctor`·`advance-omp-pin.sh`·canary gate verdict가 같은 metric 이름을 말한다. 오라클 변경 전에 probe cohort 1회가 H1–H4를 판정한다.
- **Mandatory requirements**: REQ-MEASURE-001/002, REQ-METHOD-001/002, REQ-ATTEST-001, REQ-COMPAT-001/002, REQ-DIAG-001, REQ-PIN-001, REQ-DOCTOR-001, REQ-PROBE-001, REQ-PRODUCT-001.
- **Explicit non-goals**: `min_reduction_basis_points` 2000의 수치 완화, 20 pair/40 call cohort 형태·AB/BA 균형·serial 실행 변경, attestation v2 envelope·서명키·lineage 변경(SPEC-ADK-RELEASE-SIGNING-001 영역), product runtime의 threshold 자동 compaction 임계값 변경, upstream snapcompact projection 수정, 통화 단위 비용 계산, tools allowlist·sandbox 변경, 리뷰 backend(SPEC-OMP-006) 변경.
- **Completion evidence**: AC-001–AC-012 통과, probe 수신 `probe.jsonl` 요약(숫자만)이 research.md에 기록, 23개 로컬 evidence tag가 변경 전후 동일한 historical proof 결과, 선택된 핀에서 v2 cohort가 `effective-reduction` gate를 통과한 서명 evidence, runbook·verdict table·doctor 표 갱신.

## Requirements

### Ubiquitous

**REQ-MEASURE-001 — 측정원은 provider 청구량**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL measure every observation's `input_tokens` as the delta of `get_session_stats.tokens.input + tokens.cacheRead + tokens.cacheWrite` taken on the same session immediately before the primary `prompt` and after its terminal `agent_end` (the existing `sessionStats`/`executeManaged` path), SHALL keep that delta the only attested token source, and SHALL record OMP-local estimates (`contextUsage.tokens`, `compactionSummary.tokensBefore`, `compactionSummary.tokensAfter`) only as unattested diagnostics because they are tokenizer-scoped and changed between omp/17.2.7 and omp/18.1.13.
Observability: `OMPContextPromotionObservationV2.input_tokens`; probe `probe.jsonl` `stats_before`/`stats_after`.

**REQ-MEASURE-002 — 유효 감축 metric과 floor**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL compute for each 10-call session segment `effective_reduction_bp = round_half_away_from_zero((sum_A_input - (sum_B_input + sum_B_maintenance_input)) * 10000 / sum_A_input)` over the segment's ten A and ten B observations, SHALL attest the minimum over segments as gate `effective-reduction` with required value `2000`, SHALL keep the 2000 bp floor as a hard constant that the verifier rejects when the policy states any other value, and SHALL keep the v1 per-pair median (`ReduceOMPContextCanaryPairsV1`) as a recorded diagnostic value (`median_reduction_basis_points`) that is not a gate.
Observability: gate row `effective-reduction` observed/required; `gate_diagnostic` `effective_reduction_bp=<n>/2000`.

**REQ-METHOD-001 — 방법 chain과 방법·거부 관측**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL run the optimized (B) session under `compaction.methodOrder: [remote, snapcompact]`, SHALL accept a manual `compact` completion whichever chain method OMP used, SHALL record per B observation `compaction_method` in `{none, remote, snapcompact}` read from the `compactionSummary.method` field of the post-compaction `get_messages_page` transcript — deriving it from the sole entry instead when the effective `methodOrder` has exactly one entry — and `compaction_refusal` in `{none, too_small, would_not_reduce, already_compacted}` mapped from the exact refusal texts already listed in `pipelineOMPActiveCompactionNoopMessages`, and SHALL fail the call closed on any other refusal text or an unrecognized method value.
Observability: `OMPContextPromotionObservationV2.compaction_method`/`compaction_refusal`; gate `compaction-method` observed `remote=<n>,snapcompact=<n>`.

**REQ-ATTEST-001 — report schema v2와 evidence 이름**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL emit new promotion reports with `schema_version` `autopus.omp_context_promotion_report.v2` whose observation rows carry every v1 field plus `compaction_method`, `compaction_refusal`, `maintenance_input_tokens`, `maintenance_output_tokens`, whose policy carries `min_effective_reduction_basis_points: 2000` and `compaction_method_order: ["remote","snapcompact"]`, whose gate list replaces `token-reduction` with `effective-reduction` and adds `compaction-method`, SHALL sign it with the unchanged `autopus.omp_context_promotion_attestation.v2` envelope, and SHALL publish it as `omp-context-promotion-report.v2.json` while keeping the two-entry evidence tree and the fifteen-asset release contract.
Observability: report JSON; evidence tree listing; `release_contract_test.go` asset list.

**REQ-COMPAT-001 — v1 검증 바이트 동일**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL keep `OMPContextPromotionReportV1`, `decodeOMPContextPromotionReportV1`, `validateOMPContextPromotionReportMetadataV1`, `validateOMPContextPromotionCohortV1`, and `expectedOMPContextPromotionGatesV1` behaviourally unchanged, SHALL select the strict decoder by peeking `schema_version` before decoding, and SHALL verify each of the 23 local evidence tags `omp-context-evidence-v0.50.93`…`v0.50.117` (104 and 109 absent) through `ompcontextverify --mode historical` with byte-identical inputs and identical outcomes before and after the change.
Observability: `[NEW] pkg/promptlayer/omp_context_promotion_historical_tags_test.go` comparing pre/post outcomes; `scripts/companion-release/ompcontextverify` output.

**REQ-PRODUCT-001 — product 경로 동등성**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL apply the same `methodOrder` to `workflowContextProductOverlayBody` (product runtime) and to the manual-compaction overlay (evidence producer), SHALL accept `auto_compaction_start`/`auto_compaction_end` with `action` in `{remote, snapcompact}` on the product-path threshold validators, and SHALL replace the policy identity label `snapcompact-image-schema=omp-v<pin>` with `compaction-method-chain=remote,snapcompact;compaction-oracle=effective-reduction-v2;omp-pin=omp-v<pin>` so that `production_path_equivalent: true` stays truthful and `TestOMPPinAgreesEverywhere` still reads the pin.
Observability: overlay body test; `pipelineOMPActivePolicyIdentity`; pin agreement test.

### Event-driven

**REQ-PROBE-001 — oracle 변경 전 probe cohort**
Type: Event-driven | Priority: Must
WHEN `observe-session` runs with the `[NEW]` flag `--probe-dir <dir>`, THEN THE SYSTEM SHALL run the identical 20-pair cohort with the B session under `methodOrder: [remote, snapcompact]`, SHALL write one body-free JSONL record per call and per compaction attempt (sequence, variant, session_sequence, `get_session_stats` triples before/after, each `turn_end.message.usage` quadruple, compaction outcome, method, refusal class, `tokensBefore`, `tokensAfter`, elapsed ms) to `<dir>/probe.jsonl` with mode 0600, SHALL scrub the same forbidden strings the evidence writer scrubs, and SHALL end the run with `error_code=probe_completed` so that no report, evidence store, attestation, tag, or verdict-table entry is produced.
Observability: `probe.jsonl`; final frame `error_code=probe_completed`; absence of `promotion-report-v*.json`.

**REQ-PIN-001 — verdict table 의미 보존**
Type: Event-driven | Priority: Must
WHEN `advance-omp-pin.sh` evaluates a target version, THEN THE SYSTEM SHALL keep exactly the three verdict states (in use, refused with measured numbers, unmeasured), SHALL key every refused row by the oracle version and the cohort manifest (workload) digest that measured it (`oracle=v1` for 18.1.2, 18.1.5 and 18.1.13 today), SHALL treat a version refused only under an older oracle as unmeasured under the current oracle so that `--measure` is admitted exactly once per oracle version, and SHALL print the current metric name `effective_reduction_bp` in every refusal and unmeasured message.
Observability: script stdout/stderr; `[NEW] scripts/release-tools/tests/advance-omp-pin-verdict-test.sh`.

**REQ-DOCTOR-001 — doctor 행 정렬**
Type: Event-driven | Priority: Must
WHEN `auto doctor` projects `compaction.measured_reduction`, THEN THE SYSTEM SHALL derive `measured_reduction_verified`, `measured_zero_reduction`, `reduction_unmeasured` and `version_unavailable` from one Go verdict table that the pin-script test reads back against the script's case arms, SHALL report `measured_reduction_verified` only for a version whose v2 evidence cleared `effective-reduction`, and SHALL keep the row advisory (`Required: false`).
Observability: doctor row reason; `[NEW] internal/cli/doctor_omp_context_reduction_table_test.go`.

### Unwanted

**REQ-METHOD-002 — remote compaction 경계 fail-closed**
Type: Unwanted | Priority: Must
IF a compaction emits `agent_start`, `turn_start`, `turn_end`, `agent_end` or `prompt_result` inside the compaction barrier, or the post-compaction transcript carries content the transcript validator does not classify (anything other than text, validated images, and the `compactionSummary` message), THEN THE SYSTEM SHALL fail the call closed with a body-free reason naming the frame type or content kind, and SHALL count the compaction's own provider request in `maintenance_input_tokens`/`maintenance_output_tokens` only when it completed.
Observability: error text; `compaction_provider_requests`; maintenance token fields.

**REQ-COMPAT-002 — schema 혼합 거부**
Type: Unwanted | Priority: Must
IF a report declares v2 but omits any v2 field or states a policy floor other than 2000, or declares any `schema_version` other than v1 or v2, THEN THE SYSTEM SHALL reject it with `OMP context promotion report schema is invalid` and SHALL NOT retry with another decoder; IF a report declares v1, THEN THE SYSTEM SHALL hand it to the unchanged v1 decoder whose existing errors (including the strict unknown-field error for any v2 field) stand as before.
Observability: verifier error text.

**REQ-DIAG-001 — body-free gate diagnostic**
Type: Unwanted | Priority: Must
IF the v2 cohort gate fails, THEN THE SYSTEM SHALL emit `gate_diagnostic` exactly of the form `pairs=<n>/20 compactions=<n>/2 effective_reduction_bp=<n>/2000 median_reduction_bp=<n> methods=remote/<n> snapcompact/<n> refusals=too_small/<n> would_not_reduce/<n>` where every value is a count or basis points, matching `^[a-z_=/0-9[:space:]]*$` and at most 400 bytes, and the canary failure receipt SHALL print it unchanged as `production canary gate verdict`.
Observability: observe-session error frame; `prepare-release-runtime-lib.sh` receipt line.

## Decision Needed

네 옵션 모두 floor 2000 bp는 그대로 둔다. 차이는 evidence가 무엇을 주장하느냐다.

| 옵션 | 서명 evidence의 주장 | 무엇이 바뀌나 | 통과/실패 성질 | v1 evidence 호환 | 비용·위험 |
|---|---|---|---|---|---|
| **A. 유효 감축, 방법 무관, metric 불변** | "설정된 chain(remote→snapcompact)의 manual compaction이 20 pair median에서 청구 primary input을 20% 줄였다" | B 세션·product overlay `methodOrder: [remote, snapcompact]`, 방법·거부 관측 필드, report v2 | remote가 gateway를 통과하고 줄이면 통과; 안 줄이면 실패. 그러나 compaction 시점이 한 call 늦은 버전은 실제 효과와 무관하게 실패(median 취약성) | v2 struct 분리, v1 불변 | 1 cohort; remote 경로 미검증(H1) |
| **B. 이벤트 궤적 attest** | "수행된 compaction 각각이 다음 turn의 청구 input을 줄였고 ≥2회 수행됐다" | 방법 chain은 그대로 `[snapcompact]`, pair median 대신 compaction 전후 궤적을 gate | 이벤트 2회만 효과적이면 통과 → 총 효과가 작은 버전도 통과(약한 주장). 18.1.13은 통과할 수 있음 | v2 struct | 주장이 약해져 "줄이지 않는 버전은 실패" 조건을 만족 못 함 |
| **C. 유효 감축 + 누적 집계 (권장)** | "설정된 chain의 manual compaction이 각 10-call segment의 청구 primary input(+compaction 유지비)을 20% 줄였다" | A의 변경 + segment 누적 metric(`effective-reduction`, min over segments) + median은 진단값, 방법·거부·유지 토큰 attest | 실제로 줄이는 버전은 시점과 무관하게 통과(17.2.7: 3131–3174 bp); 18.1.5(0 bp)는 실패; 18.1.13은 [INFERENCE] ≤ 약 1300 bp로 실패 예상, probe가 확정 | v2 struct, v1 불변 | 1 cohort(probe) + 1 cohort(측정); release lane 파일명 15개소 이동 |
| **D. upstream 이슈만** | 불변("snapcompact-only, median") | 없음 | 18.x는 upstream이 projection을 바꾸기 전까지 영구 refused | 불변 | 핀 정체가 계속됨; doctor의 18.x `measured_zero_reduction` 유지 |

하위 결정(C 안): **C1** 유지비 gross(무시) vs **C2** net(`sum_B_maintenance_input` 포함) — C2 권장(operator가 실제 지불하는 컨텍스트). **C3** 전체 20 pair 누적 vs **C4** segment별 min — C4 권장(segment는 독립 세션 쌍이고 한 segment의 성공이 다른 segment의 실패를 가리지 않음).

**권장: C (C2·C4), confidence: medium.** 근거: (1) 측정원(`get_session_stats` delta = provider `input_tokens`)은 두 바이너리에서 동일하게 verified(binary line 1123806-1123850 vs 1313859-1313940; usage 변환 652010-652050 vs 918934-918975)이므로 method-agnostic 유효 감축을 지금의 frame으로 뒷받침할 수 있다. (2) median의 취약성은 서명 evidence 5건에서 실측됐다(여유 309–335 bp, 8/20 pair 구조적 0–438 bp). (3) 알려진 나쁜 버전은 새 metric으로도 실패한다(18.1.5: pair 전부 0 bp면 누적도 0). **정적으로 검증 불가한 항목**: 18.1.13의 `remote` compaction이 release gateway(`provider-bound-endpoint`)를 통과하는지, remote 경로가 compaction barrier 안에서 agent/turn frame을 내는지, remote 결과(opaque item)가 transcript validator를 통과하는지, 18.1.13의 segment 누적 값. 이 넷은 plan.md의 probe cohort 1회(REQ-PROBE-001)가 결정하며, probe가 remote 불가로 판정하면 C는 chain 변경 없이 누적 metric·방법·거부 관측만 도입하는 축소형(C-min)으로 줄고, 18.1.13은 [INFERENCE]대로 실패한 채 핀이 17.2.7에 남는다 — 그것이 정직한 결과다.

## Related SPECs

- SPEC-OMP-004: managed RPC 런타임·product overlay·pre/post compaction bridge — 이 SPEC은 overlay `methodOrder`와 product validator의 `action` 허용 범위를 확장한다.
- SPEC-OMP-005: 모델 역할 투영 — 변경 없음(모델 scope digest는 그대로 attest).
- SPEC-OMP-006: RPC 리뷰 backend, 17.x/18.1.x prompt ack 이중 수락 — 같은 `pipelineOMPRPCProtocol`을 공유하며 이 SPEC은 compaction 경로만 건드린다.
- SPEC-ADK-RELEASE-SIGNING-001: attestation v2 envelope·키·lineage — 변경 없음(non-goal). report 파일명 이동은 release contract 테스트로 고정한다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-MEASURE-001 | T0, T2 | AC-001, AC-010 | INV-001 |
| REQ-MEASURE-002 | T2, T3 | AC-001, AC-002, AC-003 | INV-002, INV-003 |
| REQ-METHOD-001 | T0, T1, T2 | AC-004, AC-010 | INV-004 |
| REQ-METHOD-002 | T1, T2 | AC-005 | INV-004, INV-009 |
| REQ-ATTEST-001 | T3, T6 | AC-003, AC-006 | INV-005 |
| REQ-COMPAT-001 | T3 | AC-006, AC-007 | INV-005, INV-006 |
| REQ-COMPAT-002 | T3 | AC-007 | INV-006 |
| REQ-DIAG-001 | T2, T4 | AC-008 | INV-007 |
| REQ-PIN-001 | T5 | AC-009 | INV-008 |
| REQ-DOCTOR-001 | T5 | AC-009 | INV-008 |
| REQ-PROBE-001 | T0 | AC-010 | INV-001, INV-004, INV-009 |
| REQ-PRODUCT-001 | T1, T5 | AC-011 | INV-004 |
| Completion evidence | T6, T7 | AC-012 | INV-002, INV-005 |

## Risks

| Risk | Mitigation |
|---|---|
| remote compaction이 release gateway를 못 통과해 chain이 항상 snapcompact로 떨어짐 | probe가 먼저 판정; 그 경우 C는 누적 metric만 남기고 chain은 `[snapcompact]`로 유지(Decision Needed) |
| remote compaction 결과의 opaque item이 transcript validator를 통과하지 못함 | REQ-METHOD-002 fail-closed; probe에서 content kind만(본문 없이) 기록해 허용 규칙을 정한다 |
| 누적 metric이 17.2.7에 유리해 "완화"로 읽힘 | floor 수치·의미(청구 컨텍스트 20%) 불변, median은 진단값으로 계속 기록, 알려진 나쁜 버전의 실패를 AC-002로 고정 |
| report 파일명 `.v2.json` 이동이 release lane 15개소를 흔듦 | 한 스크립트 이동 + `release_contract_test.go`·hardening 테스트가 누락을 잡음; 이전 tag는 `.v1.json` 그대로 |
| verdict table의 "오래된 oracle로 refused" 상태가 재측정 남용을 부름 | oracle version당 `--measure` 1회, 재측정 결과는 표에 기록 필수 |
| probe 실행이 서명 evidence를 만들어 실수로 태그됨 | `error_code=probe_completed`로 lane fail-closed, report/evidence store 미기록 |

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.
