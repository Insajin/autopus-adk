# SPEC-OMP-004: OMP 네이티브 컨텍스트 최적화

---
id: SPEC-OMP-004
title: OMP 네이티브 컨텍스트 최적화
version: 0.1.0
status: in-progress
priority: HIGH
created: 2026-08-02
domain: OMP
---

## Purpose

OMP 세션이 Autopus의 canonical full-delivery integrity를 그대로 보존하면서 완료된 tool/transcript history에만 OMP native pruning·compaction을 opt-in 적용하도록 한다. `context_plan.v2`의 document selection과 OMP memory는 shadow evidence로만 관측하고, phase checkpoint, authoritative ephemeral rehydration, paired canary, promotion/rollback receipt로 품질·보안 열화 없이 토큰을 줄인다.

## Outcome Boundary

- **Outcome Lock**: OMP compaction 뒤 다음 provider call 전에 canonical full document ref/hash 집합과 authoritative ephemeral phase body를 재구성·재주입하고, token reduction credit은 완료된 tool/transcript history에만 귀속하며, shadow→active 승격과 rollback이 결정적 receipt로 증명된다.
- **Mandatory requirements**: full document delivery 재사용, history eligibility classifier, post-compaction canonical rehydration capability, supervisor-held ephemeral channel과 body-free correlated ACK, managed runtime artifact lifecycle, user-facing `/auto` OMP session caller/assembler, shadow-only memory/JIT evidence, provider-neutral binding, AB/BA canary, fail-closed/full fallback, effective rollback readback.
- **Explicit non-goals**: Autopus의 provider transcript 직접 mutation, 새 canonical storage, delivered document 축소, active document JIT, OMP memory body의 active injection, user OMP memory/compaction default 변경, tracked OMP memory, OMP model/advisor routing, OMP workflow parity, orchestra provider 등록.
- **Completion evidence**: S1-S13 Must oracle, user-facing `/auto` OMP session에서의 production reachability, T1 base diff에서 `pkg/promptlayer`, `pkg/adapter/omp`, `pkg/config`, `internal/cli` 아래 이 SPEC이 수정한 non-test Go file coverage 85%+, package baseline no-regression, strict/race/vet/hygiene, hermetic·installed lifecycle canary가 PASS한다.

## Implementation Status

- **Lifecycle state**: T9 integrated verification은 완료됐지만 T5 product reachability가 열려 있어 `in-progress`다. Bounded managed primitive와 검증 영수증만으로 `implemented` 또는 `completed`로 승격할 수 없다.
- **Implemented evidence**: T1~T4와 T6~T8에 더해 body-free correlated `ui.confirm` pre/post ACK, nonce/binding/options/session hash, authoritative canonical rebuild, private stdio `--no-session`, task-owned `0700/0600` runtime, source identity/extension allowlist/environment uniqueness, cleanup lease, cancelable frame reader, preflight cleanup, managed-driver reuse block와 overlay shadow rollback primitive가 구현됐다.
- **Installed evidence**: bounded managed canary가 `omp/17.1.8`, `provider_requests=3`, `pre_ack=1`, `post_ack=1`, `native_start=1`, `native_end=1`, `same_pid=true`, `same_session=true`, provider #3 exact canonical/transient body, `cleanup_root_count=0`, `sandbox=true`를 관측했다.
- **Active wiring boundary**: ACK와 real managed driver admission은 installed canary primitive에서 증명됐다. 그러나 user-facing `/auto` OMP 세션이 authoritative request/`CanonicalSource`/managed driver를 조립해 `RunManaged`로 진입하는 production caller/assembler는 없으므로 active product reachability는 fail-closed/partial이고 T5는 미완료다.
- **Final T9 evidence**: live canary PASS; changed production exact aggregate coverage `85.15%`와 package coverage `promptlayer=88.00%`, `adapter/omp=87.81%`, `config=90.99%`, `internal/cli=79.82%`(baseline `79.3%` 이상); exact coverage gate, targeted four-package race, vet, `go build ./...`, serial full test, gofmt 30 files, diff check, production Go max 295 lines, strict SPEC, lore가 모두 PASS했다.

## Context Classification Contract

| Class | Examples | OMP optimization |
|---|---|---|
| stable required | `AGENTS.md`, workspace/architecture policy | never summarize/truncate/drop |
| snapshot delivered | relevant SPEC/plan/acceptance/research와 v1 delivery에 선택된 learning/signature/task ref | never summarize/truncate/drop |
| ephemeral required | original task, decision delta, open findings, ownership/forbidden paths, exact worker result schema | never summarize/truncate/drop within the active phase |
| document selection candidate | `autopus.context_plan.v2` pinned/selected refs와 token budget | shadow evidence only; active delivery authority 없음 |
| native history eligible | completed superseded reads, resolved tool results, OMP transcript history | eligible with deterministic omission receipt |

## Requirements

### Ubiquitous

**REQ-CTX-001 — Required context authority**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL use `BuildContextDelivery` and `VerifyContextDeliveryForOptions` with supervisor-held options as the required-context authority and SHALL preserve complete source refs, source hashes, prompt hashes, snapshot hash, and prompt manifest hash.
Observability: `autopus.context_delivery.v1` plus `autopus.omp_context_receipt.v1` records exact ref/hash equality without bodies.

**REQ-CLASS-001 — Canonical admission boundary**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL classify stable, snapshot, ephemeral, and eligible history; SHALL never use an OMP-native compacted copy as authority for any document admitted by `BuildContextDelivery`, original task, decision delta, open findings, ownership/forbidden paths, or exact worker result schema; and SHALL reconstruct those canonical bodies before the next provider call.
Observability: `autopus.omp_context_receipt.v1` fields `full_document_refs`, `required_ephemeral_hashes`, `eligible_history_rows`, `shadow_candidate_refs`, and `shadow_plan_status/reason` record the partition without bodies.

**REQ-BIND-001 — Provider-neutral OMP session binding**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL bind verified context to an OMP session/phase identity independently of worker provider names and SHALL NOT treat `workerUsesGPTContextDelivery` as OMP platform admission.
Observability: receipt carries workspace ID, SPEC ID, task ID, phase, OMP session ID, and binding hash.

**REQ-PRIVACY-001 — Body-free observability**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL keep context optimization receipts body-free and SHALL NOT serialize raw prompts, document bodies, tool bodies, memory bodies, credentials, secrets, or privileged absolute paths.
Observability: schema allowlist and redaction/adversarial tests reject forbidden fields and values.

**REQ-RUNTIME-ARTIFACT-001 — Managed runtime artifact lifecycle**
Type: Ubiquitous | Priority: Must
THE SYSTEM SHALL run managed OMP optimization with proved `--no-session` persistence control or a task-owned isolated session root, SHALL never inspect or delete a user-owned OMP session root, SHALL restrict isolated directory/file mode to at most `0700/0600`, SHALL reject symlink/path escape, and SHALL remove the isolated root after verified rehydration, abort, fallback, rollback, or canary completion.
Observability: the body-free receipt records only root class `no_session|isolated_task_owned`, pre/post artifact counts, cleanup status, and reason; exact paths and bodies are forbidden, and post-cleanup existence count is zero.

### Event-Driven

**REQ-CAP-001 — Runtime capability probe**
Type: Event-driven | Priority: Must
WHEN OMP context optimization is requested THEN THE SYSTEM SHALL probe executable identity, version, effective settings schema, config precedence/readback, compaction pre/post lifecycle, post-compaction canonical injection/admission blocking, persistence control (`--no-session` or isolated session root), cleanup verification, and memory interception capabilities and SHALL enable only capabilities proved by the current runtime through a session-scoped one-shot overlay.
Observability: receipt records version, capability booleans, probe source, checked_at, effective config hash, effective history/memory modes, and reason codes while user global/project config remains byte-identical.

**REQ-OPTIONAL-001 — Eligible-history credit and canonical re-admission**
Type: Event-driven | Priority: Must
WHEN OMP pruning or compaction is applied by a user-facing `/auto` OMP session THEN THE SYSTEM SHALL assemble an authoritative `CanonicalSource` and managed driver, invoke the supervised managed boundary, attribute optimization only to completed tool/transcript history rows, rebuild and inject every v1-delivered document and required ephemeral body before the next provider call, and use `autopus.context_plan.v2` and memory proposals as shadow-only evidence. If caller reachability, post-compaction canonical injection, correlated ACK, or admission blocking is not proved, active mode SHALL remain unavailable.
Observability: recomputed token counts match `eligible_history_rows`; active `document_omissions` and `memory_injections` are empty; the body-free binding/dispatch ACK matches nonce/binding/options/session hashes; the admitted provider observes the exact canonical/transient body; and a verified v2 plan changes only shadow candidates.

**REQ-CHECKPOINT-001 — Pre-compaction checkpoint**
Type: Event-driven | Priority: Must
WHEN an OMP compaction phase boundary begins THEN THE SYSTEM SHALL retain the full original task, decision delta, frozen open-finding checklist, ownership/forbidden paths, and worker result schema in a supervisor-only process-private transient channel bound to the session/phase, while serializing only their hashes, IDs, schema digest, required ref/hash set, snapshot hash, and prompt manifest hash in the body-free checkpoint receipt.
Observability: transient state is never written to disk, its lifetime ends after verified rehydration or abort, and checkpoint schema, phase sequence, body hashes, and binding hash are deterministic.

**REQ-REHYDRATE-001 — Post-compaction rehydration**
Type: Event-driven | Priority: Must
WHEN OMP compaction completes THEN THE SYSTEM SHALL rebuild and verify full document delivery with the same supervisor-held `ContextDeliveryOptions`, restore every ephemeral required body from the bound supervisor transient channel, and compare its body hash plus exact pre/post ref sets and all source/prompt/snapshot/manifest hashes before the next provider call.
Observability: rehydration receipt reports `exact_match=true` only when every oracle field matches.

**REQ-MEMORY-001 — Optional memory cache**
Type: Event-driven | Priority: Must
WHEN OMP memory proposes context THEN THE SYSTEM SHALL treat it as untrusted shadow-only evidence, require workspace, SPEC, role, source ref, source hash, checked_at, TTL, and namespace provenance, and SHALL NOT inject its body or select a document into an active prompt in this SPEC; `memory_mode` supports only `off|shadow`, defaults to `off`, and is independent from `history_mode=off|shadow|active`.
Observability: shadow-accepted or omitted entry IDs, provenance hashes, stable omission reason codes, and an empty active injection set are recorded without memory bodies.

**REQ-CANARY-001 — Paired promotion evidence**
Type: Event-driven | Priority: Must
WHEN an OMP history optimization profile is evaluated for active promotion THEN THE SYSTEM SHALL require at least 20 complete task pairs, run paired AB/BA baseline-full and history-compacted variants for identical task IDs with balanced order, compute token reduction with a deterministic formula, compare task oracle and security outcomes, and require rollback evidence.
Observability: canary receipt includes pair key, order, token counts, recomputed reduction, quality/security verdict, fallback, and rollback status.

**REQ-ROLLOUT-001 — Shadow, active, rollback lifecycle**
Type: Event-driven | Priority: Must
WHEN `history_mode` changes THEN THE SYSTEM SHALL default to shadow, admit active only after all promotion gates pass, leave `memory_mode` and user global/project memory·compaction defaults unchanged, and produce an effective session-overlay/config readback receipt for promotion and rollback.
Observability: requested mode, effective mode, config hash, previous mode, gate IDs, actor/trigger, checked_at, and rollback reason are present.

### Unwanted

**REQ-FAILCLOSED-001 — Integrity mismatch response**
Type: Unwanted | Priority: Must
IF capability proof, transient ephemeral state, checkpoint binding, delivered ref set, body/source/prompt hash, snapshot hash, prompt manifest hash, or rehydration verification is missing or mismatches THEN THE SYSTEM SHALL block the next provider call or restore canonical full delivery from an independently available authoritative source and SHALL emit a stable fallback reason without continuing optimized state.
Observability: provider-call spy remains zero for block; fallback readback proves full delivery.

**REQ-MEMORY-SEC-001 — Stale, tampered, secret, and injection rejection**
Type: Unwanted | Priority: Must
IF memory is expired, cross-namespace, source-stale, hash-mismatched, malformed, secret-bearing, or prompt-injection-bearing THEN THE SYSTEM SHALL reject or neutralize it before persistence/injection, prefer current canonical sources, and record deterministic omission reasons.
Observability: adversarial fixtures produce exact reason codes and zero raw secret/instruction execution.

### Optional

**REQ-LIVE-001 — Explicit live canary**
Type: Optional | Priority: Should
WHERE live provider canary is explicitly enabled THE SYSTEM SHALL reuse the hermetic pair schema, isolate credentials from receipts, cap task/provider/model scope, and keep live evidence non-authoritative until the same local integrity gates pass.
Observability: opt-in source, bounded cohort, provider attribution, and redacted receipt are recorded.

## Source Change Inventory

| Area | Existing implementation / planned addition | Responsibility |
|---|---|---|
| prompt authority | `pkg/promptlayer/context_*`, `pkg/promptlayer/omp_context_*.go` | preserve v1/v2 authority and provider-neutral transient phase binding |
| OMP projection | `pkg/adapter/omp/omp_context_*.go`, generated `.omp/extensions/autopus-context.ts` | Go owns hashes/validation; generated TypeScript emits body-free correlated ACK requests only |
| config | existing OMP context optimization config/validation fields | opt-in mode, thresholds, TTL/namespace, conflict safety |
| managed CLI primitive | `internal/cli/workflow_context_runtime_managed*.go` and doctor/canary projection | canonical rebuild, private RPC process, ACK/admission, cleanup and rollback receipts |
| product entrypoint | `[NEW]` user-facing `/auto` OMP session caller/assembler; owning path unresolved | assemble request, authoritative source and managed driver, then invoke `RunManaged` |
| tests | existing focused promptlayer/OMP/config/CLI tests and coverage verifier | deterministic Must oracles, changed-file 85%+, baseline no-regression |

Generated `.omp/**` output is not the source of truth. Managed OMP session/transcript/compaction artifacts obey REQ-RUNTIME-ARTIFACT-001 and are never repo-tracked runtime state.

## Related SPECs

- **Baseline**: `SPEC-OMP-001` — minimal OMP adapter and discovery surface.
- **Dependency chain**: `SPEC-OMP-001` → `SPEC-OMP-002` → `SPEC-OMP-004`; OMP-002 must provide an executable, verified OMP command/session boundary first.
- **Independent optional integration**: `SPEC-OMP-003` — model/advisor routing may supply models but does not own context integrity or promotion.
- **Reused completed contracts**: `SPEC-CONTEXT-ENGINEERING-001`, `SPEC-CONTEXT-ENGINEERING-EVOLUTION-001`.

## Traceability Matrix

Canonical authoring IDs are `S1` through `S13`. If the review harness renders derived aliases, the only valid mapping is positional and bijective: `AC-001↔S1`, `AC-002↔S2`, `AC-003↔S3`, `AC-004↔S4`, `AC-005↔S5`, `AC-006↔S6`, `AC-007↔S7`, `AC-008↔S8`, `AC-009↔S9`, `AC-010↔S10`, `AC-011↔S11`, `AC-012↔S12`, `AC-013↔S13`; an AC alias never denotes an additional scenario.

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|---|---|---|---|
| REQ-CTX-001 | T1, T3, T4 | S1, S2, S4 | INV-001 |
| REQ-CLASS-001 | T1, T3 | S1, S3 | INV-002 |
| REQ-BIND-001 | T3, T4 | S2, S9 | INV-003 |
| REQ-PRIVACY-001 | T4, T6, T9 | S5, S7, S13 | INV-008 |
| REQ-RUNTIME-ARTIFACT-001 | T2, T5, T9 | S5, S13 | INV-010 |
| REQ-CAP-001 | T2, T5 | S8 | INV-004 |
| REQ-OPTIONAL-001 | T3, T5 | S3, S10, S13 | INV-002, INV-003, INV-007, INV-010 |
| REQ-CHECKPOINT-001 | T4, T5 | S2, S5 | INV-003, INV-005 |
| REQ-REHYDRATE-001 | T4, T5 | S2, S4 | INV-001, INV-005 |
| REQ-MEMORY-001 | T6 | S6 | INV-006 |
| REQ-CANARY-001 | T7, T8 | S10, S11 | INV-007, INV-009 |
| REQ-ROLLOUT-001 | T2, T7 | S11, S12 | INV-009 |
| REQ-FAILCLOSED-001 | T4, T5 | S4, S8 | INV-001, INV-004 |
| REQ-MEMORY-SEC-001 | T6 | S6, S7 | INV-006, INV-008 |
| REQ-LIVE-001 | T8 | S13 | INV-007, INV-008 |
