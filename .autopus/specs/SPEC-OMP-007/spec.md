# SPEC-OMP-007: OMP context promotion oracle — 18.x에서도 유효한 compaction 효과 측정

---
id: SPEC-OMP-007
title: OMP context promotion oracle — 18.x에서도 유효한 compaction 효과 측정
version: 0.2.0
status: approved
priority: HIGH
created: 2026-09-08
domain: OMP
---

## Purpose

릴리즈 핀은 omp/17.2.7에 머물러 있고, 그 이유는 하네스 결함이 아니라 측정 결과다. 같은 plan generator, 같은 20 task pairs, 같은 gateway와 모델(`openai-codex/gpt-5.6-sol`)로 바이너리만 바꿔 돌린 cohort는 omp/17.2.7에서 compaction 8회·per-pair median 2309–2335 bp(A23–A28 서명 evidence 5건), omp/18.1.5에서 2회·per-pair median 0 bp, omp/18.1.13에서 2회·per-pair median 895 bp를 냈고 나머지 13개 gate는 모두 통과했다(`docs/runbooks/omp-pin-advance.md` "Measured 2026-09-07", `scripts/release-tools/advance-omp-pin.sh:75-97`). 이 세 숫자는 전부 v1 oracle의 per-pair median이다. 18.x 두 버전의 segment 누적값은 evidence에 보존되지 않았고, median 0은 "모든 pair가 0"도 "누적 감축이 0"도 함의하지 않는다 — 같은 산식에서 median 0이면서 누적 3552 bp인 궤적이 존재한다(acceptance.md의 B‴ fixture).

서명 evidence가 주장하는 것은 정확히 이것이다: "`compaction.methodOrder: [snapcompact]` 오버레이 아래에서 reused call마다 manual `compact`를 호출하면, provider가 청구한 primary input tokens가 20 pair의 median에서 2000 bp 이상 준다"(`pkg/promptlayer/omp_context_promotion_report_cohort.go:26-32`, `omp_context_canary_reduce.go:59,117-123`, `internal/cli/workflow_context_runtime_product_overlay.go:151`). 정적 분석(research.md)은 세 가지를 확정했다. (1) 측정원은 이미 provider 청구량이다 — `get_session_stats.tokens.input+cacheRead+cacheWrite`의 prompt 전후 delta(`pipeline_omp_context_active_usage.go:44-57`, `_rpc_execute.go:68-121`)이며 두 바이너리 모두 assistant message `usage`를 합산한다. 따라서 원격(provider-native) 감축이 B 세션에서 일어났다면 지금 metric에도 보였을 것이고, "local 측정이 remote 감축을 못 본다"는 가설은 성립하지 않는다. (2) 대신 하네스가 방법을 강제한다: 오버레이가 `remote`를 제거하고 18.1.13의 `compact()`는 `methodOrder`에 있는 방법만 고르며(binary line 1311157-1311183), 기본 순서는 `[remote, snapcompact, handoff, shake, soft]`이고 `openai`/`openai-codex`는 remote 대상이다(line 1038206, 986204). 17.2.7에는 `methodOrder`가 없고 `compaction.strategy`(기본 `snapcompact`)로 고르며 `methodOrder` 키는 무시된다(line 1121697-1121760). (3) 18.0.9의 "skip inflating snapcompact results"(PR #10024)가 projection 거부(`ue >= pe` → `snapcompact would not reduce context locally.`, line 1311284-1311296)를 추가했고, 18.1.x는 같은 workload에서 18회 시도 중 2회만 compaction한다(17.2.7은 8회; 나머지 16회 거부의 클래스 분포는 probe가 확정한다). 이 거부는 provider 청구량이 아니라 OMP 로컬 tokenizer 추정치끼리의 비교다.

원인에 대한 세 진술은 hypothesis로 남는다: (1) 18.1.5의 median 0에서 18.1.13의 median 895로 바뀐 원인(릴리즈 노트 v18.1.6·18.1.8·18.1.10 중 무엇이 기여했는지 증거 없음), (2) "오버레이의 `snapcompact` 강제가 18.x median 저하의 원인"이라는 설명, (3) 18.x 두 버전의 segment 누적 감축값. 측정이 보여주는 것은 median 세 값과 거부 횟수뿐이고 원인을 지목하지 않는다 — probe(T0)와 측정 cohort(T7)가 판정한다.

동시에 현재 oracle 자체의 취약점이 드러났다. A28 evidence의 20 pair 값은 `[0,0,438,338,2533,2137,4030,3550,5172,4674]×2`이고 median 2335는 각 segment의 5·6번째 pair가 결정한다. 처음 네 pair는 history가 너무 작아 어떤 버전도 compaction을 못 하므로 구조적으로 0–438 bp에 묶이고, 다섯 cohort의 floor 여유는 309–335 bp뿐이다. compaction 시점이 한 call만 늦어도 실제 효과와 무관하게 floor를 놓친다. 반면 같은 evidence의 segment 누적 청구량은 A 1,524,105 대 B 1,040,415 → 3174 bp다. 같은 omp/17.2.7로 만든 23개 evidence tag의 median은 v0.50.93–97에서 7084–7305 bp, v0.50.98부터 2556, v0.50.101–108에서 3308–3310, v0.50.110부터 2309–2335 bp로 움직였다 — OMP는 그대로였고 harness 구현과 workload prompt 크기만 바뀌었다(research.md 실측 표).

이 SPEC은 floor(2000 bp)를 낮추지 않는다. 대신 (a) evidence가 무엇을 주장하는지를 "OMP가 유효 compaction chain으로 낸 유효(provider 청구) 감축"으로 바꾸고, (b) 그 감축을 segment 누적 청구량(compaction 유지비 포함, C2)의 segment별 최솟값(C4)으로 집계하며, (c) 방법·거부 사유·유지비를 관측 사실로 남기고, (d) 과거 A22–A28 v1 evidence 검증을 바이트 단위로 보존하고, (e) oracle 코드를 바꾸기 전에 한 번의 probe cohort로 가설을 판정한다.

## Outcome Boundary

- **Outcome Lock**: promotion evidence v2가 "OMP의 유효 compaction chain(policy `effective_method_order`: 이 저장소가 바이너리로 읽은 major만 — omp/17.x는 `[snapcompact]`, omp/18.x는 `[remote, snapcompact]`, 그 외 major는 fail-closed; 오버레이 body는 두 경우 모두 `methodOrder: [remote, snapcompact]`)이 10-call segment의 provider 청구 primary input을 관측된 compaction 유지비까지 계상해 20% 이상 줄였다"를 attest하고, 유지비가 관측 불가한 compaction은 통과가 아니라 fail-closed이며, 이 oracle 아래에서 floor를 넘는 버전은 통과하고 넘지 못하는 버전은 실패하며, 과거 v1 evidence 23건은 변경 전후 동일하게 검증되고, `auto doctor`·`advance-omp-pin.sh`·canary gate verdict가 하나의 Verdict State Table에서 같은 metric 이름을 말한다. 오라클 변경 전에 probe cohort 1회가 H1′–H4를 판정한다. 이 SPEC은 어떤 버전이 v2 floor를 넘는다고 주장하지 않는다 — 17.2.7의 v2 값도 미측정이고, 측정값이 floor 미만이면 결과는 아래 Completion evidence의 `blocked` outcome이다.
- **Mandatory requirements**: REQ-MEASURE-001/002, REQ-METHOD-001/002, REQ-ATTEST-001, REQ-COMPAT-001/002, REQ-DIAG-001, REQ-PIN-001, REQ-DOCTOR-001, REQ-PROBE-001, REQ-PRODUCT-001.
- **Explicit non-goals**: `min_reduction_basis_points` 2000의 수치 완화, 20 pair/40 call cohort 형태·AB/BA 균형·serial 실행 변경, attestation v2 envelope·서명키·lineage 변경(SPEC-ADK-RELEASE-SIGNING-001 영역), product runtime의 threshold 자동 compaction 임계값 변경, upstream snapcompact projection 수정, `Already compacted` 거부의 no-op 승격, 통화 단위 비용 계산, tools allowlist·sandbox 변경, 리뷰 backend(SPEC-OMP-006) 변경.
- **Completion evidence**: AC-001–AC-012 검증과 historical v1 동일 결과를 남긴다. T7의 실행 결과는 `passed`, `blocked_below_floor`, `blocked_unobservable`로 구분한다. `passed`만 v2 서명·발행을 허용한다. 완전한 cohort가 floor 미만이면 숫자·oracle·cohort digest를 `refused`로 기록한다. 관측 불가나 안전 경계 오류로 중단되면 감축률을 만들지 않고 `unmeasured`와 body-free `last_attempt_reason`을 남긴다. 후보가 어느 blocked 결과든 실제 release pin을 복원하고 선택된 핀에서도 같은 절차로 한 번 측정한다. 그 핀도 실패하면 v2 발행은 blocked로 남고 사용자에게 알린다. 기존 v1 서명 이력은 별도 보존하며, 실패값을 verified 값으로 덮어쓰거나 floor 2000을 완화하지 않는다.

## Requirements

### Ubiquitous

**REQ-MEASURE-001 — 측정원은 provider 청구량 (primary와 maintenance)**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL measure primary `input_tokens` as the same-session delta of `get_session_stats.tokens.input + tokens.cacheRead + tokens.cacheWrite` immediately around the primary prompt, and SHALL attribute compaction maintenance to the B observation that requested it. The `compact` response `data.preserveData.openaiRemoteCompaction.usage.inputTokens/outputTokens` is a [STATIC] candidate source for the final remote attempt, not proof of all billed attempts.
THE SYSTEM SHALL classify maintenance as `observed_remote` only when correlated transport/provider evidence proves either exactly one provider attempt or complete input/output usage for every attempt (including retries, stream failures and failed methods before a fallback). A final success, a first-chain method, absence of retry events, and `retry-off` alone SHALL NOT establish this proof. omp/18.1.13's internal `Toe` retry loop exposes final-attempt usage only; without additional verified attempt coverage its remote completions SHALL be `unobserved`, not admitted as C2.
THE SYSTEM SHALL classify `local_render` only for a verified no-provider local image-render path with no preceding or hidden provider attempt; zero maintenance is allowed only with that proof or proof that no provider request occurred. Failed, declined and non-completing attempts SHALL NOT silently become zero. Missing attempt coverage SHALL abort promotion as `blocked_unobservable`, retain a body-free reason, and emit no numeric v2 evidence. When coverage is complete, maintenance input/output SHALL equal the sum across all attempts. Tokenizer estimates remain unattested diagnostics.
Observability: `OMPContextPromotionObservationV2.input_tokens`/`maintenance_input_tokens`/`maintenance_output_tokens`/`maintenance_observability`; receipt `MaintenanceInputTokens`/`MaintenanceOutputTokens`/`MaintenanceObservability`/`CompactionImages`; probe `stats_before`/`stats_after`/`maintenance_usage_present`/`maintenance_usage_keys`/`compaction_images`/`ui_request_methods`.

**REQ-MEASURE-002 — 유효 감축 metric과 floor**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL compute for each 10-call session segment `effective_reduction_bp = round_half_away_from_zero((sum_A_input - (sum_B_input + sum_B_maintenance_input)) * 10000 / sum_A_input)` over the segment's ten A and ten B observations, where `sum_B_maintenance_input` is the sum of the segment's B `maintenance_input_tokens`, SHALL keep that value signed and unclamped so that a segment whose observed maintenance exceeds its saving reports a negative value (the existing `ompContextReductionBasisPointsV1` rounding already rounds both signs away from zero), SHALL attest the minimum over segments as gate `effective-reduction` with required value `2000` and fail the cohort whenever that minimum is below `2000`, SHALL keep the 2000 bp floor as a hard constant that the verifier rejects when the policy states any other value, and SHALL keep the v1 per-pair median (`ReduceOMPContextCanaryPairsV1`) as a recorded diagnostic value (`median_reduction_basis_points`) that is not a gate and that implies nothing about the cumulative value, since median `0` admits both a cohort that saved nothing and the late-compaction cohort whose segment cumulative is 3552 bp.
Observability: gate row `effective-reduction` observed/required; `gate_diagnostic` `effective_reduction_bp=<v>/2000 maint_input_tokens=<n>`.

**REQ-METHOD-001 — 유효 chain·방법·거부 관측**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL run the optimized (B) session under the overlay `compaction.methodOrder: [remote, snapcompact]` on every OMP version, SHALL derive `effective_method_order` from the `omp --version` probe already recorded as `runtime.omp_version` through the `[NEW]` verified-chain table `pipelineOMPActiveEffectiveChains` whose only rows are the majors this repository read out of a binary — major `17` yields `["snapcompact"]` (omp/17.2.7 binary 1121697-1121760: selection is `compaction.strategy` with default `snapcompact`, `methodOrder` is ignored, and `compactionSummary` carries no `method`/`tokensAfter`) and major `18` yields `["remote","snapcompact"]` (omp/18.1.13 binary 1311127-1311183 plus the upstream v18.0.0 `compaction.methodOrder` note) — SHALL fail the run closed with `managed active OMP effective compaction chain is unverified: <version>` for every other major so that no unread binary's behaviour is assumed, SHALL accept a manual `compact` completion whichever effective-chain method OMP used, SHALL record per B observation `compaction_method` in `{none, remote, snapcompact}` read from `compactionSummary.method` of the post-compaction `get_messages_page` transcript when the field is present (its value is required to be in `effective_method_order`), derived as the sole entry when the field is absent and `effective_method_order` has exactly one entry, and failed closed as `managed active OMP compaction method is unobservable` when the field is absent and the chain has two entries, SHALL require `auto_compaction_start.action`/`auto_compaction_end.action` — when those legacy lifecycle frames are present — to equal the derived method, and SHALL record `compaction_refusal` in `{none, too_small, no_messages, would_not_reduce}` mapped exactly from `pipelineOMPActiveCompactionNoopMessages` (`Nothing to compact (session too small)` → `too_small`, `Nothing to compact (no messages yet)` → `no_messages`, `snapcompact would not reduce context locally.` → `would_not_reduce`), failing the call closed on any other refusal text (including `Already compacted`, which stays an invalid response exactly as today) or any other method value.
Observability: `OMPContextPromotionObservationV2.compaction_method`/`compaction_refusal`; policy `effective_method_order`; `[NEW] pipelineOMPActiveEffectiveChains` row test; receipt `Method`/`Refusal`; gate `compaction-method` observed `remote=<n>,snapcompact=<n>`.

**REQ-ATTEST-001 — report schema v2, gate projection, evidence 이름**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL emit new promotion reports with `schema_version` `autopus.omp_context_promotion_report.v2` whose observation rows carry every v1 field plus `compaction_method`, `compaction_refusal`, `maintenance_input_tokens`, `maintenance_output_tokens`, `maintenance_observability`, whose policy replaces `min_reduction_basis_points` with `min_effective_reduction_basis_points: 2000` and adds `compaction_method_order: ["remote","snapcompact"]` and `effective_method_order` (exactly the `pipelineOMPActiveEffectiveChains` row for `runtime.omp_version`'s major — `["snapcompact"]` for 17, `["remote","snapcompact"]` for 18 — with no default for an unread major), whose gate list replaces `token-reduction` with `effective-reduction` (`observed_value` the minimum segment bp rendered as a signed decimal, `required_value` `2000`) and adds `compaction-method` (`observed_value` `remote=<n>,snapcompact=<n>`, `required_value` `chain=<effective_method_order joined by ,>`, status `passed` only when `remote + snapcompact` equals the compaction count, every nonzero method is in `effective_method_order`, and every counted compaction's `maintenance_observability` is `observed_remote` or `local_render`), SHALL project all fifteen gate rows deterministically from the aggregate and policy through `[NEW] expectedOMPContextPromotionGatesV2` and reject any other list as `OMP context promotion gate projection mismatch`, SHALL sign it with the unchanged `autopus.omp_context_promotion_attestation.v2` envelope, and SHALL publish it as `omp-context-promotion-report.v2.json` (runtime path `.autopus/runtime/omp-context/promotion-report-v2.json`) while keeping the two-entry evidence tree and the fifteen-asset release contract.
Observability: report JSON; evidence tree listing; `release_contract_test.go` asset list; `validate_canary` gate count 15.

**REQ-COMPAT-001 — v1 검증 바이트 동일**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL keep `OMPContextPromotionReportV1`, `decodeOMPContextPromotionReportV1`, `validateOMPContextPromotionReportMetadataV1`, `validateOMPContextPromotionCohortV1`, and `expectedOMPContextPromotionGatesV1` behaviourally unchanged, SHALL select the strict decoder by peeking `schema_version` after the v1 pre-decode checks, and SHALL verify each of the 23 local evidence tags `omp-context-evidence-v0.50.93`…`v0.50.117` (104 and 109 absent) through `ompcontextverify --mode historical` with byte-identical inputs and identical outcomes before and after the change.
Observability: `[NEW] pkg/promptlayer/omp_context_promotion_historical_tags_test.go` comparing pre/post outcomes; `scripts/companion-release/ompcontextverify` output.

**REQ-PRODUCT-001 — product 경로 동등성**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL write the same overlay body (`compaction.methodOrder: [remote, snapcompact]`) for `workflowContextProductOverlayBody` (product runtime) and the manual-compaction overlay (evidence producer) on every OMP version, SHALL treat `effective_method_order` as a value derived from `runtime.omp_version` through `pipelineOMPActiveEffectiveChains` and never as a second overlay, SHALL accept `auto_compaction_start`/`auto_compaction_end` with `action` in `effective_method_order` on the product-path threshold validators and on `manualCompact`'s legacy start/end checks, SHALL fail those validators closed with `managed OMP native compaction start is invalid` when `runtime.omp_version`'s major has no verified chain row, and SHALL replace the policy identity label `snapcompact-image-schema=omp-v<pin>` with `compaction-method-chain=remote,snapcompact;compaction-oracle=effective-reduction-v2;omp-pin=omp-v<pin>` so that `production_path_equivalent: true` stays truthful and `TestOMPPinAgreesEverywhere` still reads the pin.
Observability: overlay body test; `pipelineOMPActivePolicyIdentity`; pin agreement test.

### Event-driven

**REQ-PROBE-001 — oracle 변경 전 probe cohort**
Type: Event-driven | Priority: Must
WHEN `observe-session` runs with the [NEW] `--probe-dir` flag, THEN THE SYSTEM SHALL execute the identical 20-pair probe workload without producing a report, evidence store, attestation, tag or verdict-table measurement. Call records SHALL retain sequence/variant/session, stats before/after, turn usages, identity delta and elapsed time; compaction records SHALL retain outcome/method/refusal, estimates, usage presence/keys, attempt-coverage status, image count and role/type/UI-method histograms. Only metadata identifiers matching `^[A-Za-z_][A-Za-z0-9_]{0,63}$` SHALL be preserved verbatim, case-sensitively, in histogram keys and identifier arrays. Out-of-range identifiers SHALL increment a rejection count, never collide into an `unprintable` allowlist entry; stderr redaction remains separate. Probe metadata SHALL never automatically grant a production allowlist entry without T1 semantic review.
THE SYSTEM SHALL write bounded records under isolated `$TMPDIR` with directory 0700/file 0600 and SHALL stop and verify the entire canary UID process set is absent before retaining records. A [NEW] descriptor-based export helper SHALL traverse from a trusted directory descriptor with no symlink following at any component, open the source once, verify that same descriptor with fstat (regular file, expected UID, single link, bounded size), and read only that descriptor. It SHALL validate/sanitize records in memory, create a new runner-owned 0600 destination exclusively under a trusted retained directory, and publish only validated serialized records. Path re-open, `sudo install` of untrusted original bytes, overwrite, and hard-link/symlink sources are forbidden. Export or cleanup failure SHALL publish no artifact and no approval. These controls precede receipt output and isolation-root removal.
THE SYSTEM SHALL retain `.autopus/runtime/omp-context/probe-<nonce>.jsonl` outside cleanup only after successful export, report accepted/rejected counts, and end a full probe with `error_code=probe_completed`. An interrupted probe SHALL instead retain its partial record count and body-free abort reason without calling it completed or measured.
Observability: retained `probe-<nonce>.jsonl`; final frame `error_code=probe_completed`; receipt line `probe summary`; absence of `promotion-report-v*.json`; `[NEW] advance-omp-pin.sh --probe`.

**REQ-PIN-001 — verdict table 의미 보존**
Type: Event-driven | Priority: Must
WHEN `advance-omp-pin.sh` evaluates a target version, THEN THE SYSTEM SHALL classify it by the Verdict State Table below into exactly one of `in_use`, `refused`, `unmeasured`, SHALL key every `refused` row by the oracle version, the metric name, the measured value, and the cohort manifest digest that measured it, SHALL classify a version measured only under an older oracle as `unmeasured` under the current oracle (today 18.1.2, 18.1.5 and 18.1.13 are `refused` under `oracle=v1` and `unmeasured` under `oracle=v2`) so that `--measure` is admitted exactly once per oracle version while a `refused` row under the current oracle rejects `--measure`, SHALL refuse `--measure` and the `[NEW]` flag `--probe` for a target whose major has no `pipelineOMPActiveEffectiveChains` row with `omp/<v>: effective compaction chain is unverified; read the binary and add a verified row first`, and SHALL print the current metric name `effective_reduction_bp` — plus the history line `refused under oracle v1 median_reduction_bp=<n>/2000` when one exists — in every refusal and unmeasured message.
Observability: script stdout/stderr; `[NEW] scripts/release-tools/tests/advance-omp-pin-verdict-test.sh`.

**REQ-DOCTOR-001 — doctor 행 정렬**
Type: Event-driven | Priority: Must
WHEN `auto doctor` projects `compaction.measured_reduction`, THEN THE SYSTEM SHALL derive the reason from the same Go Verdict State Table (`[NEW] internal/cli/doctor_omp_context_reduction_table.go`) that the pin-script test reads back against the script's case arms — `in_use` → `measured_reduction_verified`, `refused` → `measured_reduction_below_floor`, `unmeasured` → `reduction_unmeasured`, empty version → `version_unavailable` — SHALL retire the reason token `measured_zero_reduction` everywhere it is enumerated (`doctor_omp_context_reduction.go`, the projection allowlist in `doctor_omp_context_projection.go:122-124`, `docs/runbooks/omp-pin-advance.md`) because a measured 895 bp or 1300 bp is below the floor and is not zero, SHALL name the measured value and its oracle in the row detail (`oracle=v2 effective_reduction_bp=<n>/2000`), SHALL report `measured_reduction_verified` only for the pinned version whose latest signed release evidence cleared its own oracle's reduction gate (`token-reduction` 2335 bp under oracle v1 until the first v2 release canary, then `effective-reduction` under v2), naming that oracle and metric in the row detail, SHALL replace the `omp/18.1.` prefix rule with exact-version rows so that an unlisted 18.1.x is `reduction_unmeasured`, and SHALL keep the row advisory (`Required: false`).
THE SYSTEM SHALL prioritize current-oracle completed below-floor evidence over historical success, and current-oracle aborted/unobservable attempts over historical success as `unmeasured`, SHALL keep `pinned` and `historical_verified` as independent metadata rather than state overrides, and SHALL preserve signed historical values with their original oracle without replacing them with failed or partial v2 results.
Observability: doctor row reason and detail; `[NEW] internal/cli/doctor_omp_context_reduction_table_test.go`; `doctor_omp_context_projection.go` reason allowlist.

### Unwanted

**REQ-METHOD-002 — remote compaction 경계 fail-closed**
Type: Unwanted | Priority: Must
IF a compaction emits `agent_start`, `turn_start`, `turn_end`, `agent_end` or `prompt_result` inside the compaction barrier, or `compaction_method` is `remote` and the `compact` response lacks `preserveData.openaiRemoteCompaction.usage.inputTokens` (the V1 remote fallback returns `{provider, replacementHistory, compactionItem}` without usage), or the completed compaction's `maintenance_observability` is `unobserved` — its method is not the first entry of `effective_method_order` (omp/18.1.13 recurses to the next configured method after an attempt that already reached the provider, binary 1311387-1311388, so that attempt is billed and never reported), or the derived method is `snapcompact` and the post-compaction transcript added no image digest (the provider LLM summary path) — or the post-compaction transcript fails the existing structural walk of `validatePipelineOMPActiveMessageValue` (sanitizer exact-pass on every string and key, `type: image` maps only as PNG with compaction provenance, `image`-named keys only as `images` on `compactionSummary`), or the post-compaction transcript's set of `role` values or content `type` values contains a token that is neither in the same call's pre-compaction transcript nor in `{compactionSummary}` nor in the `[NEW]` allowlist `pipelineOMPActiveCompactionContentTypes` (empty until T0 fixes it from the probe histogram), THEN THE SYSTEM SHALL fail the call closed with a body-free reason (`managed active OMP provider activity crossed the compaction barrier`, `managed active OMP remote compaction usage is unobservable`, `managed active OMP compaction maintenance cost is unobservable`, the existing walk errors, or `managed active OMP transcript introduced unclassified content type: <token>` where `<token>` is printed only when it matches `^[a-z_]{1,32}$` and is `unprintable` otherwise), and SHALL NOT count the attempt in `compaction_provider_requests` or in the maintenance token fields; the existing `validatePipelineOMPActiveBridgeFrame` rejection of every `extension_ui_request` method other than `confirm` (`managed active OMP emitted unsupported UI activity`) stays unchanged and covers OMP's own `notify` frames.
THE SYSTEM SHALL apply the same fail-closed rule when final remote usage exists but complete provider-attempt coverage is absent, including an internal same-method retry that eventually succeeds, SHALL emit no numeric reduction report after such an abort, and SHALL handle it in T7 as `blocked_unobservable`, never `refused` with a fabricated zero.
Observability: error text; receipt `CompactionCycles`/`MaintenanceObservability`; `compaction_provider_requests`; probe `roles`/`content_types`/`ui_request_methods` histograms.

**REQ-COMPAT-002 — schema 혼합 거부**
Type: Unwanted | Priority: Must
IF a report declares v2 but omits any v2 field, states a policy floor other than 2000, or states an `effective_method_order` that is not the `pipelineOMPActiveEffectiveChains` row for `runtime.omp_version`'s major (including a major with no verified row), or states `maintenance_observability: local_render` for an observation whose `compaction_method` is not the first entry of `effective_method_order`, or declares any `schema_version` other than v1 or v2, THEN THE SYSTEM SHALL reject it with `OMP context promotion report schema is invalid` and SHALL NOT retry with another decoder; IF a report declares v1, THEN THE SYSTEM SHALL hand it to the unchanged v1 decoder whose existing errors (including the strict unknown-field error for any v2 field) stand as before; and the `[NEW]` dispatcher SHALL run the v1 pre-decode checks in the v1 order (size bound, UTF-8 validity, `rejectDuplicateOMPContextEvidenceKeysV1`) before peeking `schema_version`, so that a body with a duplicated `schema_version` key fails with the unchanged text `decode OMP context promotion report: OMP context evidence contains invalid or duplicate key` before any decoder is selected.
Observability: verifier error text.

**REQ-DIAG-001 — body-free gate diagnostic**
Type: Unwanted | Priority: Must
IF the v2 cohort gate fails, THEN THE SYSTEM SHALL emit `gate_diagnostic` exactly of the form `pairs=<n>/20 compactions=<n>/2 effective_reduction_bp=<v>/2000 maint_input_tokens=<n> median_reduction_bp=<v> chain=<snapcompact|remote_snapcompact> methods=remote/<n> snapcompact/<n> refusals=too_small/<n> no_messages/<n> would_not_reduce/<n>` where every `<n>` is a count and every `<v>` is either digits or `neg` followed by digits — a negative basis-point value renders as `neg<digits>` because the receipt filter (`prepare-release-runtime-lib.sh:124`) drops any verdict string containing `-`, which is why v1's signed `median_reduction_bp=-<n>` never reaches the receipt — matching `^[a-z_=/0-9[:space:]]*$` and at most 400 bytes, and the canary failure receipt SHALL print it unchanged as `production canary gate verdict`.
THE SYSTEM SHALL compute diagnostic `maint_input_tokens` as the cohort-wide sum over both segments, not the selected segment's sum, while effective reduction remains the minimum segment value; maintenance 20000 per segment therefore yields `maint_input_tokens=40000`.
Observability: observe-session error frame; `prepare-release-runtime-lib.sh` receipt line.

## Verdict State Table

`advance-omp-pin.sh`(T5), `[NEW] internal/cli/doctor_omp_context_reduction_table.go`, AC-009는 이 표 하나에서 파생된다. 현재 oracle은 v2이고, 한 버전은 정확히 한 state를 가진다.

| state | 조건 | `advance-omp-pin.sh` | doctor reason |
|---|---|---|---|
| `in_use` | 현재 oracle 실패·중단 기록이 없고 현재 핀의 최신 서명 evidence가 자기 oracle의 gate를 통과 | `already at omp/<v>`, exit 0; v1 bootstrap 이력은 oracle를 표시 | `measured_reduction_verified` — 서명된 값만 표시 |
| `refused` | 현재 oracle의 완전한 cohort가 floor 미만; 핀 여부나 과거 서명 성공보다 우선 | `--measure`에도 실패; 실제 값·oracle·cohort 출력 | `measured_reduction_below_floor`; 과거 서명 값은 별도 detail |
| `unmeasured` | 현재 oracle에서 완전한 측정이 없거나 마지막 시도가 관측 불가로 중단; 중단은 역사적 성공보다 우선 | 측정·probe 허용, 중단 사유와 필요한 관측원 명시; 수치 없음 | `reduction_unmeasured`; `last_attempt_reason` 별도 표시 |
| — | 버전 문자열 없음 | — | `version_unavailable` |

초기 행(T5 시점): omp/17.2.7 `in_use`(`oracle=v1 median_reduction_bp=2335/2000 compactions=8`; v2 값은 첫 v2 release canary 또는 T7이 기록), omp/18.1.2·18.1.5 `unmeasured`(v1 median 0 bp, 2026-09-03), omp/18.1.13 `unmeasured`(v1 median 895 bp, 2026-09-07), 그 외 모든 버전 `unmeasured`. v1 median은 v2 metric의 값을 함의하지 않으므로 어떤 v1 이력도 `refused`(oracle=v2)를 만들지 못한다. T7은 측정한 버전을 `refused`(oracle=v2, 숫자·cohort digest 기록) 또는 `in_use`로 옮긴다.
Precedence fixture: pinned 17.2.7 with historical v1=2335 and complete v2=1300 is `refused`, not `in_use`; pinned 17.2.7 with the same historical evidence but an unobservable v2 abort is `unmeasured`, not verified at v2. Both keep historical v1=2335 separately. T7 never writes a failed v2 value into an `in_use` row.

## Decision Needed

네 옵션 모두 floor 2000 bp는 그대로 둔다. 차이는 evidence가 무엇을 주장하느냐다.

| 옵션 | 서명 evidence의 주장 | 무엇이 바뀌나 | 통과/실패 성질 | v1 evidence 호환 | 비용·위험 |
|---|---|---|---|---|---|
| **A. 유효 감축, 방법 무관, metric 불변** | "유효 chain의 manual compaction이 20 pair median에서 청구 primary input을 20% 줄였다" | B 세션·product overlay `methodOrder: [remote, snapcompact]`, 방법·거부 관측 필드, report v2 | remote가 gateway를 통과하고 줄이면 통과; 안 줄이면 실패. 그러나 compaction 시점이 한 call 늦은 버전은 실제 효과와 무관하게 실패(median 취약성) | v2 struct 분리, v1 불변 | 1 cohort; remote 경로 미검증(H1′) |
| **B. 이벤트 궤적 attest** | "수행된 compaction 각각이 다음 turn의 청구 input을 줄였고 ≥2회 수행됐다" | 방법 chain은 그대로 `[snapcompact]`, pair median 대신 compaction 전후 궤적을 gate | 이벤트 2회만 효과적이면 통과 → 총 효과가 작은 버전도 통과(약한 주장). 18.1.13은 통과할 수 있음 | v2 struct | 주장이 약해져 "줄이지 않는 버전은 실패" 조건을 만족 못 함 |
| **C. 유효 감축 + 누적 집계 (채택)** | "유효 chain(17.x `[snapcompact]`, 18.x `[remote, snapcompact]`)의 manual compaction이 각 10-call segment의 청구 primary input(+관측된 compaction 유지비)을 20% 줄였다" | A의 변경 + segment 누적 metric(`effective-reduction`, min over segments) + median은 진단값, 방법·거부·유지비 관측성 attest, effective chain을 검증된 major 표에서 policy로 기록 | 감축 시점에 좌우되지 않는다(A23–A28의 v1 evidence를 누적으로 재계산하면 3131–3174 bp지만, net 산식·새 cohort에서의 값은 [INFERENCE]이고 미측정). 18.1.5·18.1.13은 v2에서 `unmeasured`이며 median 0·895는 누적값을 함의하지 않는다 — 통과/실패는 T7 측정이 결정한다 | v2 struct, v1 불변 | 1 cohort(probe) + 1 cohort(측정); release lane 파일명 15개소 이동 |
| **D. upstream 이슈만** | 불변("snapcompact-only, median") | 없음 | 18.x는 upstream이 projection을 바꾸기 전까지 영구 refused | 불변 | 핀 정체가 계속됨 |

하위 결정(C 안): **C1** 유지비 gross(무시) vs **C2** net(`sum_B_maintenance_input` 포함) — C2(operator가 실제 지불하는 컨텍스트). C2의 "유지비 0"은 가정이 아니라 관측이다: `local_render`(chain의 첫 방법으로 완료 + 이미지 digest ≥1 → 로컬 렌더, provider 요청 없음)만 0이며, remote는 응답 usage로 계상하고 관측 불가한 경우(usage 부재, chain의 뒤 방법으로 완료, 이미지 없는 snapcompact 완료)는 fail-closed라 C1으로 퇴화하지 않는다. **C3** 전체 20 pair 누적 vs **C4** segment별 min — C4(segment는 독립 세션 쌍이고 한 segment의 성공이 다른 segment의 실패를 가리지 않음).

**정적으로 검증 불가한 항목**: 18.1.13의 `remote` compaction이 release gateway(`provider-bound-endpoint`)를 통과하는지, remote 경로가 compaction barrier 안에서 agent/turn frame을 내는지, remote 결과(`providerPayload.items`의 `compaction` item)가 transcript 규칙을 통과하는지, V2 remote가 V1 remote로 fallback하는 빈도(usage 부재), remote 실패 후 snapcompact로 내려가는 빈도, 18.1.13의 segment 누적 값. 이들은 plan.md의 probe cohort 1회(REQ-PROBE-001)가 결정한다. remote가 완료되지 않으면 18.x의 compaction은 chain의 뒤 방법으로 끝나고, 그 시도의 provider 비용은 어떤 RPC 응답에도 없으므로 해당 call은 fail-closed다 — 즉 18.x는 remote가 usage와 함께 완료될 때에만 v2 evidence를 만들 수 있고, 그렇지 않으면 핀은 17.2.7에 남는다. 17.2.7은 chain `[snapcompact]`로 v2 evidence를 **생성**할 수 있지만(method 유도, 유지비 `local_render`), 그 cohort가 floor를 넘는지는 미측정이다. 넘지 못하면 Completion evidence의 `blocked` outcome이 적용되고, floor·oracle 재검토는 사용자 결정 사항이다.

**결정 (2026-09-08, 사용자): C 채택, 하위 결정 C2(유지비 net)·C4(segment별 min), floor 2000 bp 불변.** probe cohort(REQ-PROBE-001)는 spec review 게이트 통과 후 `/auto go` 첫 단계(T0)로 실행한다.

## Related SPECs

- SPEC-OMP-004: managed RPC 런타임·product overlay·pre/post compaction bridge — 이 SPEC은 overlay `methodOrder`와 product validator의 `action` 허용 범위를 확장한다.
- SPEC-OMP-005: 모델 역할 투영 — 변경 없음(모델 scope digest는 그대로 attest).
- SPEC-OMP-006: RPC 리뷰 backend, 17.x/18.1.x prompt ack 이중 수락 — 같은 `pipelineOMPRPCProtocol`을 공유하며 이 SPEC은 compaction 경로만 건드린다.
- SPEC-ADK-RELEASE-SIGNING-001: attestation v2 envelope·키·lineage — 변경 없음(non-goal). report 파일명 이동은 release contract 테스트로 고정한다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-MEASURE-001 | T0, T1, T2 | AC-001, AC-004, AC-010 | INV-001, INV-010 |
| REQ-MEASURE-002 | T2, T3 | AC-001, AC-002, AC-003, AC-008 | INV-002, INV-003 |
| REQ-METHOD-001 | T0, T1, T2 | AC-004, AC-010 | INV-004 |
| REQ-METHOD-002 | T1, T2 | AC-004, AC-005 | INV-010, INV-011 |
| REQ-ATTEST-001 | T3, T6 | AC-001, AC-003, AC-006 | INV-004, INV-005 |
| REQ-COMPAT-001 | T3 | AC-006, AC-007 | INV-005, INV-006 |
| REQ-COMPAT-002 | T3 | AC-007 | INV-006 |
| REQ-DIAG-001 | T2, T4 | AC-008 | INV-007 |
| REQ-PIN-001 | T5 | AC-009 | INV-008 |
| REQ-DOCTOR-001 | T5 | AC-009 | INV-008 |
| REQ-PROBE-001 | T0 | AC-005, AC-010 | INV-001, INV-009, INV-011, INV-012 |
| REQ-PRODUCT-001 | T1, T5 | AC-011 | INV-004 |
| Completion evidence | T6, T7 | AC-012 | INV-002, INV-005 |

## Risks

| Risk | Mitigation |
|---|---|
| remote compaction이 실패해 snapcompact로 fallback | 선행·내부 시도의 유지비를 모두 관측하지 못하면 `blocked_unobservable`; snapcompact 성공만으로 수치나 evidence를 만들지 않는다 |
| 17.2.7에서 v2 evidence 또는 floor 통과 불가 | 단일 chain method 유도는 유지하되 실제 관측·측정 결과에 따라 passed 또는 두 blocked 결과로 기록한다 |
| remote compaction 유지비가 관측되지 않아 C2가 C1으로 퇴화 | 측정원을 `compact` 응답 `preserveData.openaiRemoteCompaction.usage`로 고정하고 부재 시 fail-closed(REQ-MEASURE-001/REQ-METHOD-002) |
| remote 결과의 `compaction` item·`openaiResponsesHistory` payload가 transcript 규칙에 걸림 | novelty 규칙은 probe 모드에서 기록만 하고, T1이 probe histogram으로 allowlist를 고정(REQ-METHOD-002) |
| 누적 metric이 17.2.7에 유리해 "완화"로 읽힘 | floor 수치·의미(청구 컨텍스트 20%) 불변, median은 진단값으로 계속 기록, 알려진 나쁜 버전의 실패를 AC-002로 고정 |
| report 파일명 `.v2.json` 이동이 release lane 15개소를 흔듦 | 한 스크립트 이동 + `release_contract_test.go`·hardening 테스트가 누락을 잡음; 이전 tag는 `.v1.json` 그대로 |
| verdict table의 "오래된 oracle로 refused" 상태가 재측정 남용을 부름 | oracle version당 `--measure` 1회, 재측정 결과는 표에 기록 필수 |
| probe 파일/상위 디렉터리 교체로 root가 다른 파일을 복사 | canary UID 종료 확인 후 no-follow descriptor 기반 읽기·fstat·검증된 레코드만 배타적 생성; raw 원본 복사 금지 |
| 검증되지 않은 major(19.x 등)로 핀을 옮기며 chain을 추측 | `pipelineOMPActiveEffectiveChains`에 없는 major는 런타임·verifier·pin script 모두 fail-closed; 행 추가는 바이너리 읽기 후에만(REQ-METHOD-001/REQ-PIN-001, AC-004) |
| 18.x에서 remote 시도가 provider에 닿은 뒤 실패해 snapcompact가 완료되고 청구된 유지비가 보고되지 않음 | chain 위치 규칙(첫 방법이 아닌 완료 = `unobserved`)과 이미지 digest로 `maintenance_observability`를 판정하고 `unobserved`는 fail-closed(REQ-METHOD-002, AC-004); probe가 빈도를 먼저 기록하고, 그 결과 18.x가 evidence를 못 만들면 핀은 17.2.7에 남는다 |
| 17.2.7의 v2 cohort가 floor를 넘지 못해 release lane에 v2 evidence가 없음 | Completion evidence의 `blocked` outcome으로 정직하게 기록(측정값·cohort digest·runbook·verdict row) 후 사용자 결정으로 escalate; floor 완화는 이 SPEC 밖 |
| 음수 effective reduction이 receipt 필터에 걸려 verdict가 사라짐 | diagnostic 문법이 `neg<digits>`로 렌더(REQ-DIAG-001, AC-008); 400바이트·charset 불변 |

## Out of Scope

Outcome Boundary의 explicit non-goals와 동일하다.

## Review Revision 1

리뷰(rev 0, 2026-09-08) finding별 해결. 코드 인용은 모두 재확인했다(research.md "현재 측정 경로"·"바이너리 정적 증거").

- F-001 → REQ-METHOD-001/REQ-ATTEST-001/REQ-PRODUCT-001, Outcome Lock, AC-004, plan T1: `effective_method_order`를 `runtime.omp_version`에서 유도(17.x `[snapcompact]`, 18.x `[remote, snapcompact]`), 17.2.7은 method 부재 시 `snapcompact`로 유도되어 v2 evidence를 만든다; Decision C 행과 Risks에 반영.
- F-002 → REQ-MEASURE-001/REQ-MEASURE-002/REQ-METHOD-002, INV-010, AC-004: 유지비 측정원을 `compact` RPC 응답 `data.preserveData.openaiRemoteCompaction.usage.inputTokens/outputTokens`(18.1.13 `woe()` line 985980-985990, `JJr()` 985823-985845, `oB()` 982799)로 고정; `get_session_stats`는 compaction usage를 합산하지 않음(`appendModelUsage`는 auto-thinking에서만 호출, line 1308337); remote인데 usage 부재면 fail-closed.
- F-003 → REQ-METHOD-002, INV-011, AC-005, plan T1: "분류기" 서술을 `validatePipelineOMPActiveMessageValue`의 실제 구조 walk로 바꾸고, 미지 content는 pre-compaction 대비 신규 `role`/`type` 토큰으로 정의(allowlist는 probe histogram으로 T1이 고정).
- F-004 → `## Verdict State Table`, REQ-PIN-001/REQ-DOCTOR-001, AC-009: state→script→doctor reason 1:1 매핑, 17.2.7 `in_use`(v1 evidence로 verified, oracle 명시), 18.1.13 `unmeasured`(v1 refused 이력 출력), `omp/18.1.` prefix 규칙 제거.
- F-005 → plan T2/T3/T6, Ownership, REQ-ATTEST-001: `BuildOMPContextPromotionStaticPolicyV3`·`companion_omp_context_static_policy.go`·`matchesOMPContextPromotionStaticPolicyV3`·`computeOMPContextPromotionEvidenceIDV1`·authority digest·`pipeline_omp_context_observation.go:115`·runtime 파일명 4개소·`validate_canary` gate 수 14→15 추가; v2 policy는 `min_reduction_basis_points`를 제거한다고 명시.
- F-006 → REQ-METHOD-001/REQ-DIAG-001, AC-008: text→class 매핑 표(`too_small`/`no_messages`/`would_not_reduce`), `already_compacted` 제거(`Already compacted`는 두 바이너리 모두 throw하지만 no-op 목록에 없어 오늘처럼 invalid response), diagnostic 문법에 `no_messages`·`chain` 추가.
- F-007 → REQ-ATTEST-001, AC-001: `compaction-method` gate의 `required_value: chain=<effective chain>`과 pass 규칙(합계=compaction 수, 방법 ⊆ chain), `[NEW] expectedOMPContextPromotionGatesV2` projection + DeepEqual.
- F-008 → research "현재 측정 경로" 방법 강제 행, plan T1: 현 두 바이너리는 manual `compact`에서 `auto_compaction_start/end`를 내지 않지만 하네스(`_lifecycle.go:69-73,129-133`)와 legacy fixture(`_rpc_process_fixture_test.go:183-186`)는 받으므로 T1이 line 70 start 검사와 `validPipelineOMPActiveNativeEnd`를 effective chain으로 확장.
- F-009 → REQ-PROBE-001, plan T0, AC-010: `--probe-dir`는 canary isolated `$TMPDIR` 아래, `run_canary` 실패 분기에서 receipt 직전 retained 경로 `.autopus/runtime/omp-context/probe-<nonce>.jsonl`로 반출(cleanup·`temp_dir` 삭제 밖); tolerance-0 등식은 `usage_identity_delta` 기록 항목으로 강등하고 Must는 `probe_completed`·무서명·record 수·enum 값으로 한정.
- F-010 (deferred by reviewer) → 변경 없음; Evolution Ideas에 testdata vendoring을 advisory로 유지.
- F-011 → REQ-COMPAT-002, AC-007(e): dispatcher는 v1 순서의 pre-decode 검사(size, UTF-8, duplicate-key) 뒤에 `schema_version`을 peek; 중복 `schema_version`은 v1과 글자 같은 duplicate-key 오류.

## Review Revision 2

rev 1(위 `## Review Revision 1`) 이후 남아 있던 정직성·근거 문제를 목표별로 해결했다. 코드·바이너리 인용은 모두 이 리비전에서 다시 읽었다.

- P-1 doctor reason이 895/1300 bp를 `measured_zero_reduction`으로 말함 → REQ-DOCTOR-001, Verdict State Table `refused` 행, AC-009: reason을 `measured_reduction_below_floor`로 바꾸고 detail에 측정값·oracle을 넣으며, 열거 지점 3곳(`doctor_omp_context_reduction.go`, `doctor_omp_context_projection.go:122-124` allowlist, `docs/runbooks/omp-pin-advance.md`)에서 `measured_zero_reduction`을 제거한다(plan T5).
- P-2 음수 effective reduction이 diagnostic charset(`^[a-z_=/0-9[:space:]]*$`, `prepare-release-runtime-lib.sh:124`)에 걸림 → REQ-MEASURE-002(부호 유지·clamp 금지), REQ-DIAG-001(`neg<digits>` 렌더), AC-008(음수 fixture `effective_reduction_bp=neg131/2000`, 220바이트).
- P-3 median 0을 "모든 pair 0" 또는 "누적 0"으로 읽던 서술 → Purpose, Decision C 행, Verdict State Table 초기 행, REQ-MEASURE-002: 세 숫자를 per-pair median으로 명시하고 18.1.5·18.1.13을 v2 `unmeasured`로 고정(AC-002 fixture는 합성값으로 남는다).
- P-4 "upstream 18.1.8이 개선을 냈다 / snapcompact 강제가 원인" → Purpose 뒤 단락과 research H1′·H3: 세 진술을 hypothesis로 강등하고 판정 수단(probe T0, 측정 cohort T7)을 남긴다.
- P-5 거부·실패한 compaction 요청의 provider 비용을 0으로 가정 → REQ-MEASURE-001(`maintenance_observability` 3-값 분류), REQ-METHOD-002(`unobserved` fail-closed), REQ-ATTEST-001·REQ-COMPAT-002(gate pass 조건과 `local_render` 정합성 검사), AC-004: 0은 `local_render` 관측(chain 첫 방법 완료 + 이미지 digest ≥1; 17.2.7 binary 1121795-1121835 / 18.1.13 1311265-1311300)에만 허용하고, 이미지 없는 snapcompact 완료(17.2.7 LLM 요약 경로 1121762·1121838)와 chain의 뒤 방법으로 끝난 완료(18.1.13 1311387-1311388 재귀)는 fail-closed다. OMP notice는 RPC 모드에서 `extension_ui_request{method: notify}`(18.1.13 1441896-1441902)로 오고 `validatePipelineOMPActiveBridgeFrame`(line 266-272)이 이미 거부하므로 새 frame arm을 만들지 않는다.
- P-6 remote usage 필드를 실측처럼 서술 → REQ-MEASURE-001에 `[STATIC]` 표기(바이너리 읽기만; 런타임 존재는 T0 probe까지 미확인)와 probe의 `maintenance_usage_keys` 기록.
- P-7 major `>= 18`을 미래 major까지 일반화 → REQ-METHOD-001/REQ-ATTEST-001/REQ-PRODUCT-001/REQ-COMPAT-002/REQ-PIN-001: `[NEW] pipelineOMPActiveEffectiveChains`를 읽은 바이너리(17, 18)만으로 한정하고 그 외 major는 런타임·verifier·pin script에서 fail-closed(AC-004의 `omp/19.0.0` 케이스).
- P-8 AC-012가 핀 유지 시 17.2.7의 v2 통과를 보장 → Outcome Boundary Completion evidence에 `passed`/`blocked` 두 terminal outcome 정의, Decision 마지막 단락, Risks 새 행, AC-012: 통과는 측정으로만 주장하고 실패는 `blocked`로 기록·escalate한다(floor 2000 불변).
- P-9 미구현 flag를 기존 것처럼 서술 → REQ-PROBE-001·REQ-PIN-001·plan A1/T0·AC-010에서 `[NEW] --probe-dir`, `[NEW] OMP_CONTEXT_PROBE_DIR`, `[NEW] advance-omp-pin.sh --probe`로 표기.

## Review Revision 3 — pending re-review

- 2026-09-09 재리뷰의 F-001/F-004: AC-012 chain을 선택된 runtime에서 유도하고 현재 oracle 실패·중단을 과거 성공보다 우선시한다. 핀 여부와 historical evidence는 별도 보존한다.
- F-002: remote 내부 재시도를 포함한 모든 provider 시도의 비용 관측을 요구한다. 최종 성공 usage만 있는 현재 RPC 응답은 C2 증거로 부족하며, 관측원 없이는 승격 blocked다.
- F-003: probe metadata identifier는 camelCase를 보존한다. stderr redaction과 allowlist 근거 데이터를 분리한다.
- F-009/F-012(security): UID 종료·process-free 확인, descriptor 기반 no-follow 읽기, fstat, 검증 후 새 retained 파일 생성으로 검사-사용 경쟁을 차단한다.
- Provider F-012(completeness)/F-013: partial abort와 full-cohort below-floor를 구분하고, 진단 유지비는 cohort 전체 합으로 명시한다.
- 마지막 review receipt는 이 수정 이전 입력에서 REJECT다. Claude reviewer와 judge가 계정 429로 실행되지 못했으므로 이 문서는 draft를 유지하며 재리뷰가 필요하다. revision 1·2의 기록은 변경 이력이며 충돌 시 현재 REQ와 이 revision이 우선한다.
