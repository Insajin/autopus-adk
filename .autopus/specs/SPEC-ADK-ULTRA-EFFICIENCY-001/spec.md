# SPEC-ADK-ULTRA-EFFICIENCY-001: Token-Efficient Ultra Quality Allocation

---
id: SPEC-ADK-ULTRA-EFFICIENCY-001
title: Token-Efficient Ultra Quality Allocation
version: 0.2.1
status: completed
priority: HIGH
---

**Created**: 2026-07-11  
**Updated**: 2026-07-15
**Source**: `BS-052`  
**Target module**: `autopus-adk/`  
**Operational live provider scope**: GPT through the Codex execution path only
**Lifecycle implementation**: `true` for the completed SPEC/document lifecycle; the full-evaluation terminal artifact retains `implemented=false` as an experiment rollout boundary
**Depends On**: `SPEC-BUDGET-001`, `SPEC-COMPRESS-001`, `SPEC-CODEXQUAL-001`, `SPEC-HARNESS-WORKFLOW-TEAM-001`, `SPEC-ACCGATE-002`  
**Narrowly revises**: `SPEC-COMPRESS-001` below-threshold pass-through, `SPEC-HARNESS-WORKFLOW-TEAM-001` Ultra review depth  

## Purpose

Make Ultra the highest-confidence Autopus mode without treating every task as if it requires the maximum prompt, context, reviewer count, and synthesis pass. The system first measures provider usage, removes repeated fixed context, and then uses conservative pre-dispatch risk evidence to choose either a compact premium review lane or the current full Ultra review profile. Model, effort, implementation fan-out, security, and deterministic quality gates remain unchanged.

## Background

At initial approval, the implementation baseline had the following gaps. This historical list does not describe the completed v0.2.0 state:

- `pkg/telemetry/types.go::AgentRun` stores one `EstimatedTokens` value.
- `pkg/worker/adapter/interface.go::TaskResult` and `pkg/orchestra/types.go::ProviderResponse` do not retain input, output, reasoning, cache, or tool-token usage.
- `pkg/promptlayer/layer.go::Render` provides stable ordering, hashes, token estimates, and cache eligibility, but does not prove provider cache use.
- `pkg/worker/compress.DefaultCompressor` preserves structured context, but hard compression starts after 50% of the provider window and the live worker pipeline does not consistently install the compressor.
- `templates/claude/commands/auto-router.md.tmpl` and `templates/gemini/commands/auto-router.md.tmpl` embed broad routed guidance and unconditional project-context loading.
- `pkg/workflow.ResolveDepth("ultra")` defines the canonical three-vote plus synthesis tuple. The generated team workflow has a one-vote/no-synthesis schema baseline and accepts runtime depth overrides, but only dry-run rendering currently applies the resolved quality binding deterministically; live route-team dispatch lacks that binding bridge.
- `internal/cli/review_risk_tier.go` already classifies changed paths and reduces generic review provider fan-out for low/medium risk, but route-team review does not reuse this evidence.

Official provider guidance supports just-in-time context, exact stable-prefix ordering, dynamic effort based on evaluated task complexity, and multi-agent fan-out only where work is genuinely independent. Prompt caching remains a billable-cost and latency optimization, not a raw-token reduction.

## Outcome Boundary

- **Outcome Lock**: Ultra preserves its premium capability and high/critical quality contract while measuring actual usage, removing unrelated fixed prompt/context, pruning stale repeated handoff content, and selecting review depth from conservative pre-dispatch risk evidence.
- **Mandatory requirements**: normalized usage with null-safe semantics; accepted-task accounting; thin routing; bounded context receipts; typed pruning; conservative risk eligibility; mandatory security; fail-open full Ultra; paired promotion evidence; generated-surface parity; Balanced and user-config compatibility.
- **Explicit non-goals**: a new quality mode; Balanced changes; public 25% promise; provider pricing overhaul; fixed response truncation; security or deterministic-gate removal; direct generated-surface edits; mandatory provider cache activation; adaptive implementation fan-out; within-run review-call expansion; model or effort downshift.
- **Completion evidence**: strict SPEC validation, focused Go tests with 85%+ coverage in changed packages or an explicit review-approved exception, rendered-surface size/parity oracles, A/A measurement evidence, a GPT/Codex-only paired accepted-task report, high/critical zero-regression report, full-depth audit sample, and applied rollback receipt. Claude/Gemini live calls, Claude `route_team` proof, and multi-provider consensus are not required for the v0.2.0 completion decision; their existing hermetic compatibility tests remain mandatory.

## Definitions

- **UsageEnvelope**: versioned normalized usage for one provider call, with identity, provenance, nullable actual components, estimates, cost, and subset semantics.
- **Usage status**: `actual`, `cost_only`, `estimated`, or `unavailable`.
- **Actual-complete call**: a call whose provider-reported inclusive input and output semantics are sufficient to compute `raw_total_tokens` without guessing.
- **Raw input tokens**: inclusive provider input after normalization. Anthropic-style uncached input plus cache creation plus cache read is normalized to one inclusive value.
- **Raw total tokens**: inclusive input plus inclusive output, plus a reasoning or tool component only when provider metadata states that component is separate from both totals.
- **Billable cost**: provider-reported actual cost when available; otherwise a separately labeled estimate with a pricing version.
- **Accepted task**: a distinct task whose deterministic acceptance and final pipeline status are PASS.
- **Minimum premium review lane**: an eligible low/medium Ultra path selected before dispatch with one reviewer, mandatory security, no synthesis, and the existing implementation fan-out, retry, model, and effort profile unchanged.
- **Full Ultra review profile**: the existing high/critical, sensitive, unknown, audit, or binding-failure path with three review votes, mandatory security, and synthesis while keeping the current fan-out, retry, model, and effort profile.
- **Context receipt**: bounded task-specific metadata and optional-recall envelope containing outcome, constraints, ownership, acceptance, references, decision delta, and manifest hashes. Complete required-document bodies are a separate verified snapshot outside the 800–2,000-token envelope; optional artifacts remain available by stable reference.
- **Policy promotion**: movement from shadow to a larger canary stage after deterministic usage and quality gates pass.

## v0.2.0 Operational GPT/Codex Evidence Contract

The product implementation remains provider-compatible, but the live evidence boundary for this revision is deliberately narrower. New operational calls use `gpt-5.6-sol` through the canonical `auto agent run` to worker Codex adapter path. Existing non-Codex fixtures and rendered-surface parity tests remain required and must not be removed or weakened. Claude/Gemini live calls, live Claude `route_team` proof, and multi-provider consensus are explicitly outside the current completion boundary.

`evidence/live-canary-preflight-v1.json` is immutable historical evidence of the earlier Claude `route_team` assumption. It is not rewritten or deleted. New GPT/Codex evidence supersedes that file only for the current operational scope and must identify its own provider, model, effort, config, corpus, and policy hashes.

The frozen corpus hash is `sha256:a3454f01b734d3f72060bc9b93972032b908f88940960e7f7b0953ab7356958a`. Its deterministic target oracle preflight covers all 12 tasks and is required to report 12 of 12 PASS before any live comparison is eligible. The authorized live cohort is intentionally bounded to seven tasks:

| Task | Risk | Pair order | Baseline | Candidate |
|---|---|---|---:|---:|
| `ute-corpus-v1-001` | low | AB | full 5 calls | compact 2 calls |
| `ute-corpus-v1-004` | medium | BA | full 5 calls | compact 2 calls |
| `ute-corpus-v1-005` | medium audit | AB | full 5 calls | full 5 calls |
| `ute-corpus-v1-011` | low | BA | full 5 calls | compact 2 calls |
| `ute-corpus-v1-012` | medium | AB | full 5 calls | compact 2 calls |
| `ute-corpus-v1-006` | high sentinel | BA | full 5 calls | full 5 calls |
| `ute-corpus-v1-009` | critical sentinel | AB | full 5 calls | full 5 calls |

The full tuple is three reviewer calls at `xhigh`, one mandatory security call at `max`, and one consolidator call at `xhigh`. The compact tuple is one reviewer call at `xhigh` plus one mandatory security call at `max`. Child calls do not use `ultra`; the supervisor/orchestra Ultra contract remains protected by static parity tests. Baseline therefore uses 35 calls, candidate uses 23 calls, and the primary paired run uses 58 calls: 44 at `xhigh` and 14 at `max`.

Each `xhigh` call has a 22,000 raw-token rollout budget and each `max` call has a 26,000 raw-token rollout budget. The primary worst-case admission is therefore `44 × 22,000 + 14 × 26,000 = 1,332,000` raw tokens. The five-call full-profile applied rollback replay reserves another `4 × 22,000 + 1 × 26,000 = 114,000` raw tokens. The complete hard envelope is 63 calls and 1,446,000 raw tokens, leaving a 54,000-token margin under the authorization ceiling of 64 calls and 1,500,000 raw tokens. No 64th call is planned or admitted. Concurrency is one and retries are zero. The replay executes inside this pre-admitted envelope only after every prior gate passes and the circuit breaker remains closed; it is not conditioned on observed underspend.

Live evidence runs read-only and ephemeral, ignores user config and rules, skips repository discovery, disables tools and optional features, uses a strict output schema, and never persists raw prompts, responses, or provider JSONL. Only allowlisted numeric, hash, enum, identity, and normalized actual-usage fields may persist. Any usage, tool, identity, model, effort, config, or schema ambiguity, any tool event, or any non-PASS verdict opens the circuit with no retry.

Deterministic patch-hash, test-command, and security receipts are authoritative. A GPT verdict is supplementary and cannot convert a deterministic failure to PASS. Promotion requires 14 complete paired trials, seven complete quality rows, mandatory security for every task, full depth for audit task `005`, high sentinel `006`, and critical sentinel `009`, exact audit/security linkage, zero regressions, and a reported provisional median raw-token reduction of at least 25 percent. Fault injection must produce `ROLLBACK`, restore an isolated binding to `full_ultra`, and pass atomic state readback. No user config or repository activation is authorized by this SPEC.

The independent diff-only review passed with zero open P0, P1, or P2 findings. The historical attempts below retain their time-local admission and rollout decisions. The later full-evaluation v2 terminal chain passed the remaining live completion gates and moved the SPEC/document lifecycle to `completed` with lifecycle `implemented=true`; it did not promote or activate the adaptive policy.

## GPT/Codex Live Attempt Terminal State

This section and the task006 diagnosis sections through the v8 terminal state are chronological historical snapshots. Their time-local `approved`, `implemented=false`, and open-debt statements are preserved as evidence history and are superseded only for the current SPEC lifecycle by the later **GPT/Codex Full-Evaluation v2 Terminal State** section.

The first frozen v0.2.0 primary attempt terminated fail-closed at call 39 of 58. Calls 1 through 38 returned `PASS`; call 39 was task `ute-corpus-v1-006`, high-risk sentinel, arm `B`, full-profile reviewer 1 at `xhigh`, and returned `FAIL` with `finding_count=1`. The circuit opened immediately. All 39 calls reported actual usage, total observed raw usage was 523,811 tokens, tool calls and retries were zero, and no later primary or replay call was admitted.

The deterministic task `006` patch-hash and verification-command recheck passed. That deterministic result does not invalidate or override the supplementary GPT failure for promotion admission. The strict evaluator rejected the incomplete ledger, and the rollback replay admission check rejected the same ledger without making a provider call. This is positive evidence for the S25 fail-closed branch, not evidence that the 58-call comparison, S24, complete S26 replay path, S27 quality ledger, or applied rollback replay passed.

The terminal decision is `BLOCKED_NO_PROMOTION`. Candidate activation, user configuration mutation, and repository policy activation remain false; the effective safe profile remains `full_ultra`. The cumulative-authorization P1 is resolved by requiring canonical output and an ignored atomic runtime claim. Reconciliation marks policy hash `sha256:1640281825b184f9ffbb92dc36a9afac27a5b55a9ff9d2632aadfa2dcce9430b` as `CONSUMED_ON_RECONCILIATION`; canonical and noncanonical reuse probes both rejected reuse with zero provider calls. The final diff-only review passed with zero open P0, P1, or P2 findings.

The SPEC therefore remains version `0.2.0` with status `approved` and `implemented=false`, Completion Debt remains open only for the incomplete live gates, and the attempt is not resumable. Any later live attempt requires a newly frozen policy hash and explicit authorization.

### Terminal evidence index

| Evidence | SHA-256 | Meaning |
|---|---|---|
| `evidence/gpt-primary-call-ledger-v1.partial-fail.json` | `sha256:f1b2fc2171af84464c6e5e7f39d5db62918480740d60abe4fabc93784987b582` | Sanitized incomplete 39-call ledger; 38 PASS, one FAIL, circuit open, evaluation and promotion ineligible. |
| `evidence/gpt-primary-terminal-outcome-v1.json` | `sha256:29f0a73fe758e4b564870922269bce24297795e8ba3040f346fca610ef1007b8` | Terminal decision, task `006` deterministic recheck, evaluator/replay rejection, privacy state, and unchanged activation state. |
| `evidence/gpt-authorization-closure-v1.json` | `sha256:bde1c49d7458f43a1cb9bfd478bd84991645b161d6b9f9e7794f042e2051bf42` | Resolved cumulative-authorization P1, consumed policy identity, zero-call reuse probes, atomic runtime claim, and final review PASS with no open P0/P1/P2 findings. |

## Task006 Diagnostic Attempt Terminal State

A separate diagnostic-only protocol was authorized to inspect task `ute-corpus-v1-006` without changing the primary canary decision. Its authorization identity is `sha256:920e6370cebb84739872233cd4a0eeb88295bf816b19b6d43cfac99591a1dc20`. The protocol froze both arm `A` and arm `B` at the full-five profile for 10 planned calls: eight `xhigh` calls and two `max` calls with a 228,000-raw-token cap, concurrency one, zero retries, and no promotion eligibility. Its policy hash is `sha256:4a4b84f7087a5bf40aa0f5c3c2e883e29d235e80bf91c84c1186ec758248b12f`.

The diagnostic parser extension accepts only bounded `finding_code` and `scope_hash` fields in diagnostic mode. The existing primary strict parser and its fail-closed semantics are unchanged. A valid bounded diagnostic finding could continue the diagnostic sequence, while transport, usage, identity, privacy, schema, or cap failure still terminates the diagnostic without retry.

The diagnostic attempt terminated at call 1, task `006`, arm `A`, full-profile reviewer 1 at `xhigh`, with `process_nonzero`. Attempted and observed calls equal one, but actual-usage calls, observed raw tokens, tool calls, retries, and diagnostic findings all equal zero. The deterministic patch and test recheck passed, but no diagnostic response was admitted, so no task006 diagnostic quality conclusion exists. The authorization is consumed and an actual same-authorization reuse probe was rejected with zero provider calls.

Post-terminal P1 review found that the nonzero-process path could materialize a diagnostic row before the process result was admitted. The resolved implementation now checks a nonzero provider process before telemetry lookup, receipt materialization, or `build_diag_row`, and persists only schedule-bound metadata plus a sanitized `process_nonzero` failure stub. The immutable historical partial ledger is not rewritten or retroactively validated; its nested provider-derived fields are not diagnostic claims.

The same-authorization reuse probe is independently captured in `gpt-diagnostic-authorization-reuse-v1.json`: exit code 1, sentinel executable invocations 0, provider calls 0, raw tokens 0, and runtime claim SHA-256 unchanged before and after the probe. This P2 receipt makes the no-reuse assertion operationally verifiable without changing the existing authorization, ledger, or terminal outcome.

This diagnostic outcome is `DIAGNOSTIC_TERMINAL_OPERATIONAL_FAILURE`. It does not modify the immutable primary terminal evidence or prior authorization closure, does not satisfy S24 or S27, does not authorize promotion, and does not change `implemented=false` or the active `full_ultra` profile. Another live diagnostic requires transport-failure diagnosis, a new frozen policy hash, and explicit authorization.

### Diagnostic evidence index

| Evidence | SHA-256 | Meaning |
|---|---|---|
| `evidence/gpt-diagnostic-cohort-v1.json` | `sha256:d060fe3ae06e5ee063c66a57dc7b6a96bd9c929361e81ca2e26d48d333db0d9f` | Task006 A/B full-five cohort and exact 10-call ordering. |
| `evidence/gpt-diagnostic-config-v1.json` | `sha256:bff64292fdf49343bde1a53d9d40ffbea0af5f338cd48385c2a2dc6d752e0565` | Diagnostic-only provider, model, execution, and retention configuration. |
| `evidence/gpt-diagnostic-policy-v1.json` | `sha256:4a4b84f7087a5bf40aa0f5c3c2e883e29d235e80bf91c84c1186ec758248b12f` | Single-use 10-call/228,000-token no-promotion policy. |
| `evidence/gpt-diagnostic-verdict-schema-v1.json` | `sha256:d81f0205cbc02ac7af0d0897078041f765873ba287b8dfd2c0dd3e66f35ca605` | Diagnostic-only bounded `finding_code` and `scope_hash` schema. |
| `evidence/gpt-diagnostic-preflight-v1.json` | `sha256:83d79efa72892605276746e114ce740113e97d5225dbf5b6ab4bd1519f4de552` | Provider-zero preflight and explicit diagnostic execution readiness. |
| `evidence/gpt-diagnostic-call-ledger-v1.partial-fail.json` | `sha256:d0d3881e6fdad03c3289761a535f3f34f5603001c0d96af9c66acea840ac6ee0` | Immutable historical process-failure ledger; nested provider-derived fields are not admitted as validated diagnostic claims. |
| `evidence/gpt-diagnostic-terminal-outcome-v1.json` | `sha256:9869bbceea0a5ba2db05b48980f4cda44ede8dceda1e2325e678826155a13892` | Operational terminal decision, deterministic recheck, consumed authorization, and unchanged rollout state. |
| `evidence/gpt-diagnostic-authorization-reuse-v1.json` | `sha256:340e59d8791403853d5d4281bb02b0cdb4fb2af5d1c01ecc5644f15719649ebd` | Exit/sentinel/provider-zero reuse rejection and unchanged runtime claim hash. |

## Task006 Transport-Smoke Terminal State

A separate single-use transport-only smoke was authorized under identity `sha256:7078b87735deb9026654c38ae04305ab8874099ad99c5b4ae37d9956e0232b27`. It approved exactly one task `ute-corpus-v1-006` reviewer call at `xhigh`, with a 22,000-raw-token cap, concurrency one, and zero retries. The frozen policy, config, and schema hashes are `sha256:38b6ae94b0edf4c9cf09a505a5ff1b4f8cec17a478306cde99b95d9aec2411a3`, `sha256:9dd237bf913b7ac30d4002e733b8299c007f7cb1646a0c1020a9dbbdc1bc2e34`, and `sha256:61006491dddaadb43822608d10af5c3baa2e166950973dec95655ffd28003ced`.

The immutable executable was version `0.50.68-ute-transport-smoke-v1` with SHA-256 `b90c7445ca8365ccf20ea044a4793f7f8bd16a4cd0f7385915b605e6493518d5`. The smoke attempted its single call and terminated with `process_nonzero`. Actual-usage calls equal zero and usage status is `unavailable`; observed raw-token total and tool-call count are null, not zero. No retry occurred.

The terminal outcome is `TRANSPORT_SMOKE_TERMINAL_OPERATIONAL_FAILURE`. A transport conclusion is unavailable, semantic evaluation was not performed, and promotion remains false. The authorization is consumed and cannot be resumed. This smoke does not change the primary or diagnostic terminal records, does not satisfy S24 or S27, and leaves `implemented=false` with `full_ultra` active. Another live attempt requires a new policy and explicit authorization.

### Transport-smoke evidence index

| Evidence | SHA-256 | Meaning |
|---|---|---|
| `evidence/gpt-transport-smoke-policy-v1.json` | `sha256:38b6ae94b0edf4c9cf09a505a5ff1b4f8cec17a478306cde99b95d9aec2411a3` | Single-call task006 transport-only policy and cap. |
| `evidence/gpt-transport-smoke-config-v1.json` | `sha256:9dd237bf913b7ac30d4002e733b8299c007f7cb1646a0c1020a9dbbdc1bc2e34` | Canonical read-only execution and retention configuration. |
| `evidence/gpt-transport-smoke-schema-v1.json` | `sha256:61006491dddaadb43822608d10af5c3baa2e166950973dec95655ffd28003ced` | Transport acknowledgement schema without semantic findings. |
| `evidence/gpt-transport-smoke-preflight-v1.json` | `sha256:d92a3cd44da58a7daff27a60245b6a4c6197c6b5d3cc6a1596384784237c90a4` | Provider-zero frozen-artifact and single-use admission preflight. |
| `evidence/gpt-transport-smoke-ledger-v1.partial-fail.json` | `sha256:eca927f2802b734336d5c34bd3833fb1a4385c93eff6f97a825bf153cbbe37c6` | Sanitized one-attempt operational failure with unavailable usage and null observed raw tokens. |
| `evidence/gpt-transport-smoke-terminal-outcome-v1.json` | `sha256:b92db43b32a9880b8e7e6987ba6398a50ccaef323f709beef698b07f6438f268` | Consumed authorization, runtime identity, null-safe execution fields, and unchanged rollout state. |

## Task006 Transport Diagnosis v2/v3 Terminal State

Two later GPT/Codex-only diagnosis runs each admitted exactly one `ute-corpus-v1-006` reviewer call at `xhigh`, with a 22,000-raw-token cap, concurrency one, and zero retries. Diagnosis v2 consumed authorization `sha256:345523b25569eee5f691d3960c2caca0709aebcd324c7db103cb3c2b0ecf013f`; diagnosis v3 consumed authorization `sha256:db4f738c9226c55e339d8e52e875ac7e7b3aefe4efdf0b0372066fcb325a1de9`. Each authorization was used once and is not resumable.

| Run | Frozen identities | Runtime identity |
|---|---|---|
| v2 | Policy `sha256:e60cc741ea5a2abd0be0ba7e25d5d6c639fe9e2b0cd52c10dcb14ac1565b6cdc`; config `sha256:bdec20a7c73616afdcd5c70fa069f88a392a8da928bb90c8f4e192af657f5c80`; schema `sha256:0f77892d0472dedf2e6cdee5a9064e7e0798ad29e43769c87a00aff26f159306`; preflight `sha256:4e26043b3cc0ac3df8ff74a78f0616a48dacb99bc7041419f572bbc400ac8c04`; reservation `sha256:f0b4b4d3b6b40a0048ce245905b6884ac736cbb5161140cdb254cd94a05a83a5` | `0.50.68-ute-transport-diagnosis-v2`; executable `sha256:fc5bf47bb3db020876605d7450661e732817790ab9190895bd6f78726096360a` |
| v3 | Policy `sha256:b5c627569f95733511d10dda2c67b034e25f645fd292fe7b90f7bd1913180d07`; config `sha256:f05ca25b74cdd748d97919c31357cc3676cf15c1f41b53f830232fe070d56eb6`; schema `sha256:ab787a1b9581b76160ffc48b1c22659865caa250a5f20de4249fb0ec6b81ec8d`; preflight `sha256:18792e4bdc12ca1007d6ef275b2d02032f08a1e5ba674c03d81d22598a56f4f4`; reservation `sha256:28dff2e5d0507620bdedf93fdb1c3e2baa7f175f9f056f6bca80fcb14fb76f79` | `0.50.68-ute-transport-diagnosis-v3`; executable `sha256:9e118039a9e1027087b578935c410c43187d1e9ec583eb4258d6c223c9b770a5` |

Both runs terminated `process_nonzero` after one attempt. Each records zero actual-usage calls, usage `unavailable`, observed raw tokens null, zero retries, operational-error class `unknown`, and fingerprint `sha256:e161c851c1a8c4fdea86c031ea524f1e0c7d39c7399eb950ea081fa8d90f0a42`. Diagnosis v3 included the provider-error detail capture surface, but the terminal decision records `provider_error_event_detail_classified=false` and `failure_source_metadata_available=false`. It therefore does not establish a transport root cause.

The current implementation has additional raw-free `operational_error_stage` and `operational_error_signals` hardening, and its canonicalization, allowlist, and invalid-value fail-closed behavior is hermetically verified. The immutable v3 ledger contains neither field, so that later hardening is implementation evidence only and cannot be retroactively claimed as v3 failure-source evidence.

The v2 and v3 outcomes remain `TRANSPORT_DIAGNOSIS_TERMINAL_OPERATIONAL_FAILURE`: transport conclusion unavailable, semantic evaluation false, and promotion false. SPEC status remains `approved`, while `implemented=false` and `full_ultra` remains the effective profile. S24, S27, CD-02, and CD-03 remain open. The full 58-call primary plus 5-call replay is prohibited and unapproved until a transport-only run records Transport PASS. Any further live attempt requires a newly frozen policy and explicit authorization. This terminal closure is limited to GPT through the Codex execution path and does not expand live scope to Claude or Gemini.

### Transport-diagnosis v2/v3 evidence index

| Evidence | SHA-256 | Meaning |
|---|---|---|
| `evidence/gpt-transport-smoke-ledger-v2.partial-fail.json` | `sha256:3f5e76621709e9a77e222a317d34ebad664bbc1735c1f1ae2cec1f9fb2a51c6f` | v2 one-attempt partial failure; no actual usage and no root-cause classification. |
| `evidence/gpt-transport-smoke-terminal-outcome-v2.json` | `sha256:1723f7906b5e544fd9c7c3b7418bf9fd3522254e4d434da6987a34776a9c7196` | v2 consumed authorization and unchanged fail-closed decision. |
| `evidence/gpt-transport-smoke-ledger-v3.partial-fail.json` | `sha256:32b0144aa61e34bad7b277390ac341e57f9cd0a2c1cefb9d240b45cd743735d8` | v3 one-attempt partial failure without failure-source stage or signals. |
| `evidence/gpt-transport-smoke-terminal-outcome-v3.json` | `sha256:ac0e928e43d1da5a4ed609e255ea2d67ef9361cbec27fa6fc42a5a96c518b23e` | v3 sidecar-verified terminal SHA; detail remained unclassified and rollout stayed unchanged. |

## Task006 Transport Diagnosis v4 Live Terminal State

The frozen v4 GPT/Codex transport-only envelope received explicit live authorization. Authorization `sha256:11bc59a14df0ce2c77e44e28bf644b1a8e07d6d63825189b81988b1da51a1a03` was used once, is consumed, and is not resumable.

| Field | V4 terminal state |
|---|---|
| Evidence chain | Policy `sha256:a562141171b03e3ffc61aee968dc09a1760b1216448469d8db95788ec33654bf`; config `sha256:1dce4c89d3e52231bb1cdeb54c96f2a6bbf82004b1ac14e3fcabd7d2f1821711`; schema `sha256:a90b361e401497102d90b563082f38ff0a3600376dffdef9b4ebfe106ccb1421`; preflight `sha256:da363839a19dc63805e1f5574759a3eb1a1d6fcc73ea340e5a665cc6a19e78cb`; ledger `sha256:7fdd5f75b610e71fceabbb910b477b2d00ec838f02b048b0d073b3bb16d04383`; reservation `sha256:078042f1035b9b29051dff784bc7c456b5c534a39550b2e155c7411461ffd6e6`; terminal `sha256:27a2cae7b3470ab219153738dbf75d9d35a24c342ed88aae00e2b7bad2edad18`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v4`, executable `sha256:f7d4c12f354e3f8b106e68ada07286e658f888b32b3467f8a9b6137debd89bd9`; harness `sha256:9078eee0b022d5da750762942585d5189245d9b12eddc0dafb6b1e8ccadb5c42`; Codex `0.144.1`. |
| Execution | One task006 reviewer call at `xhigh`; attempted 1; retries 0; `process_nonzero`; actual-usage calls 0; observed raw tokens null; usage unavailable. |
| Failure metadata | Class `unknown`; fingerprint `sha256:e161c851c1a8c4fdea86c031ea524f1e0c7d39c7399eb950ea081fa8d90f0a42`; stage `process_wait`; signals `[provider_failure_event]`; stderr observed false. |
| Privacy | Raw stderr, provider-error event, prompt, response, and absolute-path retention are all false. |

The provider-failure-event signal narrows the failure source, but no event detail or root cause was classified. The outcome remains `TRANSPORT_DIAGNOSIS_TERMINAL_OPERATIONAL_FAILURE` and provides no Transport PASS, semantic evaluation, or promotion evidence. SPEC status remains `approved`, `implemented=false`, `full_ultra` remains effective, and S24/S27 plus CD-02/CD-03 remain open. The 58-call primary plus 5-call replay stays prohibited. Any next live attempt requires a new frozen policy and new explicit authorization, remains GPT/Codex-only, and does not add Claude/Gemini live scope.

## Task006 Transport Diagnosis v5 Live Terminal State

The frozen v5 GPT/Codex transport-only envelope received explicit live authorization. Authorization `sha256:036f1f0534fddaf72897ea5062ebd5747c558a725a47fa0149fd8de033469a64` was consumed by its single attempt and is not resumable.

| Field | V5 terminal state |
|---|---|
| Evidence chain | Policy `sha256:49b57c44cfef9105dd93d92441b3c17c1d678cfb6b04527da2fec81c78388f19`; config `sha256:42b57ec9d7619263dd78cabff60570cdc7051203270625558b798a036428b074`; schema `sha256:2c6da9ba42f09487b4c7a8d1704e08133cbfe2b05ea7001101fea92a23994d6c`; authorized preflight `sha256:a3548711c44c3dc3b776cc38ef7460417795b9485dac9f70f2b5c37d998cea16`; ledger `sha256:df2f065ba9e0f5e9548e64e69eb92f36497f0a975da150abd28327ebaa30cc3a`; reservation `sha256:f4593e9e203e7b32ef9e8985fae944967827b64794c15b6c2e45be3f2c13294e`; terminal `sha256:40a7abef2e1d2d9da5477352b09277892b138b4ec17ceee68240189b2ec389a0`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v5`, executable `sha256:b4d67aac8b946f067c2df3c3656422647947a7ab28757d8bab0e671f9b620f37`; harness `sha256:82d0b30338fbc467d80b507411cf2bdd5e8cc7d3dd1f60de973bd4e2c4bb4e14`; Codex `0.144.1`. |
| Execution | One task006 reviewer call at `xhigh`; attempted 1; retries 0; `process_nonzero`; actual-usage calls 0; observed raw tokens null; usage unavailable. |
| Failure metadata | Class `unknown`; stage `process_wait`; signal `[provider_failure_event]`; event kind `error_and_turn_failed`; shape `[top_level_message, nested_error_object, nested_error_message]`. |
| Privacy | Provider-event values and all raw sources were not retained. |

V5 narrows the observed provider-failure event to its kind and field-presence shape, but classifies neither its values nor root cause. The transport conclusion remains unavailable, and semantic evaluation and promotion are false. SPEC remains `approved`, `implemented=false`, and `full_ultra`; S24/S27 and CD-02/CD-03 remain open, and the 58+5 sequence stays prohibited. Any next live attempt requires a new frozen GPT/Codex policy and new explicit authorization, with no Claude/Gemini live expansion.

## Task006 Transport Diagnosis v6 Terminal State

The user explicitly authorized identity `sha256:b956b634ab0664f276e9a6dfa09ce517b58b48055d0d7ed4df9136b3e69a6ea4`. The single-use GPT/Codex transport-only run reserved and attempted exactly one call with zero retries, then terminated with `process_nonzero`; the authorization is consumed and non-resumable.

| Field | V6 terminal state |
|---|---|
| Evidence chain | Policy `sha256:d2da17f57fa900281d350a186dd7ed0e2aa6dcda9957c03df00e200eeca33495`; config `sha256:d88c97544931b2ac9b409100656220bf20da75b134a7208f348fa3bf49345e78`; schema `sha256:4d7a5cb92e32f45897a3a21adea0c638dd75893b109b2cb20205ba307379b986`; authorized preflight `sha256:c0eb6253943680908eaa1a9027469cf5ba6d5a44b631bee380b35c8b92fbb1b2`; partial ledger `sha256:f583d987fa793b98c89553c33789f8e8c88b4429546a341c8d09022b671d709f`; terminal outcome `sha256:35f76e17743874c7878f26743b6eacc75e201dbe0fc8897f23ee7b1f2a1e4905`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v6`, executable `sha256:083a42a0e2016d1d46b435a511027684c9752aa0a37bca9e70a939bc80ee02f4`; harness `sha256:4550f954dd178f904c31747012daffdc164a9d701c12270b720ddafb72ae27ff`; Codex `0.144.1`; consumed reservation `sha256:d0a5acbe560f0a49e498ca46a452daf20701bb594c4e8cc78aa6192d742f5dfe`. |
| Execution | Planned 1, attempted 1, completed false, retries 0; actual-usage calls 0, observed raw tokens `null`, and usage status `unavailable`. |
| Receipt | Stage `process_wait`; signal `provider_failure_event`; class `schema_or_response`; canonical `error` then `turn_failed` events with shapes `top_level_message` and `nested_error_object`/`nested_error_message`; both events contain only trait `schema_or_response` and explicit empty status-family arrays. |
| Decision | `TRANSPORT_DIAGNOSIS_TERMINAL_OPERATIONAL_FAILURE`; transport conclusion unavailable, semantic evaluation false, promotion false, and no root cause classified. |

Official Codex documentation guarantees JSONL failure event kinds but does not define a stable failure-payload schema. The v6 `schema_or_response` trait is therefore a lexical coarse observation, not proof of a schema defect, response defect, or any other root cause. SPEC remains `approved`, `implemented=false`, and `full_ultra`; S24/S27 and CD-02/CD-03 remain open, the 58+5 sequence stays prohibited, and any further GPT/Codex live attempt requires a newly frozen policy and new explicit authorization.

## Task006 Transport Diagnosis v7 Provider-Zero Frozen State

This subsection is the freeze-time historical snapshot from before the later v7 authorization and terminal run recorded below.

Provider-zero reconstruction confirms that v6 copied `gpt-transport-smoke-schema-v6.json` to the runtime basename `gpt-diagnostic-verdict-schema-v1.json`. Both arrays in that actual runtime schema lacked `items`, which is inconsistent with the Structured Outputs contract. Because no raw provider message exists, this is `STRONG_HYPOTHESIS_NOT_CONFIRMED_ROOT_CAUSE`; the separate diagnostic schema v1 `allOf`/`if`/`then`/`else` content was not the v6 runtime schema and is not attributed as the v6 cause.

| Field | V7 frozen state |
|---|---|
| Evidence | Policy `sha256:e03ab4a20c4c6cea4a82364d9f86b782556c800dcd4a78d875d3673095384d66`; config `sha256:d3c1a5747c3dc1626fad4e4636086eab32f94b54fa9ce2f479ed388965ee299d`; schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`; preflight `sha256:309fd921b8bee649b002cef7286c4f1dc576bdd6b07d7e165bdcafad2cb958bc`; authorization identity `sha256:fa56811f74e755711208878b2eb7e41db6071bb83e2208f8fd95024cb6d8d72a`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v7`, executable `sha256:10099d5ceb8aad68920e1ac4fd1aa8204dd2ec9c29d23f7b09303bc5d6306d69`; harness `sha256:ba9330911d8f34d72e5e868c7baf2236ede22b9f1bd4c0a411bcaa2ce86a8f50`; Codex `0.144.1`. |
| Schema contract | Root object; all four fields `verdict`, `finding_count`, `finding_codes`, and `finding_scope_hashes` required; `additionalProperties:false`; verdict enum `PASS`; finding-count enum `0`; both arrays use string `items`; no `$schema`, `$id`, `const`, `maxItems`, `minItems`, `uniqueItems`, `pattern`, or composition. A local postvalidator requires PASS/count 0/empty arrays. |
| Provider-zero gates | The harness verifies the exact v7 schema and actual runtime-copy hash before claim creation while retaining v6 raw-free failure receipts. Missing `items`, `allOf`, missing-required, and `additionalProperties:true` fixtures are rejected with provider/Auto/claim counts `0/0/0`; approval-pending is also `0/0/0`; fake success/failure, v1-v6 regression, and frozen-evidence immutability pass. |
| Freeze-time decision | `AWAITING_EXPLICIT_LIVE_AUTHORIZATION`; provider receipt false; observed spend of 0 calls and 0 raw tokens was preflight-only; provider execution had not started and no claim existed. |

At freeze time, the next gate was separate explicit authorization for the exact v7 identity; the later exact `실행승인` satisfied only that live-admission gate. Scope remained GPT/Codex-only with no Claude/Gemini live expansion, and no completion gate changed.

## Task006 Transport Diagnosis v7 Terminal State

The user's exact `실행승인` was bound to authorization identity `sha256:fa56811f74e755711208878b2eb7e41db6071bb83e2208f8fd95024cb6d8d72a` at `2026-07-15T11:42:07+09:00`. The single-use authorization was consumed by one GPT/Codex transport-only attempt and is non-resumable.

| Field | V7 terminal state |
|---|---|
| Evidence chain | Policy `sha256:e03ab4a20c4c6cea4a82364d9f86b782556c800dcd4a78d875d3673095384d66`; config `sha256:d3c1a5747c3dc1626fad4e4636086eab32f94b54fa9ce2f479ed388965ee299d`; schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`; authorized preflight `sha256:33b1a1a266d722a32fd40abe93d9878c656e022f58a6b40e5726676cf651c4ab`; reservation `sha256:3a3939b2373d3335c1612e048a847db12bae3c5436430f487f715c1c0698f4cb`; partial ledger `sha256:a614cf254c02bbbe178f3b818cf50eae34b8191d3251db7834ae85aac185b73f`; terminal outcome `sha256:9e6db74077d64d72b515497a640b21891e7d65cd3780a505d2ca1800cb19f0f6`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v7`, executable `sha256:10099d5ceb8aad68920e1ac4fd1aa8204dd2ec9c29d23f7b09303bc5d6306d69`; harness `sha256:ba9330911d8f34d72e5e868c7baf2236ede22b9f1bd4c0a411bcaa2ce86a8f50`; Codex `0.144.1`. |
| Execution | Planned/attempted `1/1`, retries 0, exit 1, failure code `missing_or_invalid_result`, completed false. |
| Usage and operational receipt | Actual-usage calls 0; observed raw total tokens `null`; usage `unavailable`; operational class/fingerprint/stage `null`; serialized signals/events empty; event and stderr observation plus operational receipt classification unavailable. No canonical result or usage receipt exists. These fields establish neither zero usage/cost nor provider-event/stderr absence. |
| Decision | `TRANSPORT_DIAGNOSIS_TERMINAL_OPERATIONAL_FAILURE`; Transport conclusion unavailable; root cause unclassified; semantic evaluation false; promotion false. |

The terminal result proves neither success nor failure of the v7 schema fix. The v6 missing-`items` finding remains a strong, unconfirmed hypothesis. V7 must never be retried or reused; any later live attempt requires a new policy/config/schema identity and separate explicit authorization. SPEC remains `approved`, `implemented=false`, and `full_ultra`; S24/S27 and CD-02/CD-03 remain open, the 58+5 sequence stays prohibited, and no Claude/Gemini live scope is added.

## Task006 Transport Diagnosis v8 Provider-Zero Frozen State

This subsection is the freeze-time historical snapshot from before the later v8 authorization and terminal PASS recorded below.

Provider-zero caller/shared-contract analysis found that Auto's task result marks `finding_codes` and `finding_scope_hashes` with YAML `omitempty`, so empty PASS arrays can be omitted from `result.yaml`. The smoke consumer previously required explicit `[]`, and a hermetic RED reproduced the same `missing_or_invalid_result`. Because the historical v7 raw result was not retained, the evidence level is `LOCALLY_REPRODUCED_HISTORICAL_RESULT_NOT_RETAINED`, not a confirmed exact v7 cause.

| Field | V8 frozen state |
|---|---|
| Evidence | Policy `sha256:a8452ea635e09298b71044a00dddef02b13ac7073f828e4cf902add6d8a6b845`; config `sha256:2d714d493ca29d9b1bcea87dfd96e8cef0e1278edd5e34bf15053e9430fc3f39`; schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`; preflight `sha256:66c05018a7edfec0d773ada618566dffb3f71dcb66ed6a6c64a9c114f0f64c7a`; authorization identity `sha256:c524d53a9b12d84457ee2ad44c7149c31648c35227ef066f938467af8a74945a`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v8`, executable `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`; harness `sha256:3caf3539135f03c1b613b7bf0468674736520d8a841961822f5ae1131814c8c8`; Codex `0.144.1`. |
| Minimal consumer fix | `((has("finding_codes") | not) or .finding_codes == [])` and the equivalent scope-hash check normalize only missing keys to empty; explicit `null` and nonempty values remain `missing_or_invalid_result`. |
| Verification | RED→GREEN, including RED null accepted → GREEN null rejected; explicit-empty and omitted-empty successes PASS; null/nonempty findings reject; sanitized failure PASS; generic/v6/v7 regressions PASS; all 130 v1-v7 evidence files remain immutable. |
| Freeze-time decision | `AWAITING_EXPLICIT_LIVE_AUTHORIZATION`; provider receipt false; provider/Auto/claim counts were `0/0/0`; observed raw tokens 0 was preflight-only. |

At freeze time, consumed v7 authorization could not authorize v8 and exact v8 authorization was pending. The later exact authorization satisfied only v8 transport admission; its terminal result follows.

## Task006 Transport Diagnosis v8 Terminal State

Exact authorization identity `sha256:c524d53a9b12d84457ee2ad44c7149c31648c35227ef066f938467af8a74945a` was granted at `2026-07-15T12:48:10+09:00`, consumed by one GPT/Codex transport-only call, and is non-resumable.

| Field | V8 terminal state |
|---|---|
| Evidence chain | Policy `sha256:a8452ea635e09298b71044a00dddef02b13ac7073f828e4cf902add6d8a6b845`; config `sha256:2d714d493ca29d9b1bcea87dfd96e8cef0e1278edd5e34bf15053e9430fc3f39`; schema `sha256:ceedc01912682cbb2cf870a7e0cd00c7096f48449d9f8602e9e52d92449b94e4`; authorized preflight `sha256:76fa7e15ea7114428d74ef9520a7055778c361ea4f702e69c8cabd2e89a76353`; ledger `sha256:f81df2cc4b083d2649bcd948328003a434411a394aea98a82aa095ee86e6593d`; reservation `sha256:b300d143c097ed6bcc4f3af61304ad39e78e611a33db189cbd8cf66c77824142`; terminal `sha256:13f31960752167111bc202899721f67e05b3c3d6da5c71372f778bfe65776792`. |
| Runtime | Auto `0.50.68-ute-transport-diagnosis-v8`, executable `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`; harness `sha256:3caf3539135f03c1b613b7bf0468674736520d8a841961822f5ae1131814c8c8`; Codex `0.144.1`. |
| Execution | `TRANSPORT_DIAGNOSIS_TERMINAL_PASS`; completed true; planned/attempted `1/1`; actual-usage calls 1; observed raw tokens 16,094 within the 22,000 cap; unique model calls 1; tool calls 0; retries 0; schema conformance PASS. |
| Retention | Canonical ledger and terminal receipts exist; no partial ledger or raw runtime files remain. Raw prompt, response, provider stdout/stderr, provider-event values, and absolute paths are not retained. |
| Boundaries | Transport prerequisite for a future full evaluation is satisfied. Missing-key normalization was not retained or observed live, v7 exact historical root cause remains unclassified, semantic evaluation is false, promotion is false, and implementation remains false. |

V8 authorization must not be reused. The full 58+5 sequence is no longer blocked specifically on Transport PASS, but remains unapproved and non-executable until a separate full-evaluation admission and budget are frozen and explicitly authorized. SPEC remains `approved`, `implemented=false`, and `full_ultra`; S24/S27 and CD-02/CD-03 remain open. Scope stays GPT/Codex-only with no Claude/Gemini live expansion.

## GPT/Codex Full-Evaluation v2 Provider-Zero Frozen State

The separate full-evaluation admission required after v8 Transport PASS is now frozen in provider-zero state. It starts a new v2 generation at primary call 1; the historical v1 39-call partial ledger and consumed v8 transport authorization are immutable, non-resumable inputs and cannot authorize or seed this run.

| Field | Frozen v2 receipt |
|---|---|
| Exact identity | `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` |
| Static artifacts | Eight JSON artifacts plus eight named SHA-256 sidecars; policy `sha256:147b22163b267ddcd12af5ff27225399f92351db75512d12afbc73386ce7d791`, config `sha256:cbeec982ffb2e11eea1d65345c3190a03016d6d0c96c1e84b28c50d006d0a6de`, preflight `sha256:aa95f09214dce1b9968586bee649dc0ac394d328ab56ecdd1522aeb3d0e09d8c`, and 142 prior evidence artifacts bound without absolute paths. |
| Source/runtime binding | Full 17-member chain `sha256:9c449510ae7918f2d066119cfc2374739e4ae7e83100321645c0c79710afc626`; nine-member admission bundle `sha256:45711d43d420414be3ec462155542d9eeb0d4dd5d36daec9dee1ae801bd616ab`; Auto `0.50.68-ute-transport-diagnosis-v8` at `sha256:5744f2c4190a1f557320bd6f8034dd809e88bec626b237df1dd8d403fdc148f8`; Codex `0.144.1` at `sha256:134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477`. |
| Envelope | Primary 58 calls / 1,332,000 raw-token worst case; gated replay 5 calls / 114,000; planned total 63 / 1,446,000; authorization caps 64 / 1,500,000; concurrency 1; retries 0; no 64th scheduled call. |
| Deterministic and privacy preflight | 12 of 12 deterministic receipts PASS; provider calls, raw tokens, authorization receipts, reservations, v2 claims, and dynamic v2 artifacts all equal zero; no raw prompt/response or absolute path is retained. |
| Evaluation binding | The evaluator is provider-zero and requires exact 14 security/quality receipts. Task005 is the isolated 100% audit proof while primary sampling remains 20%; high task006 and critical task009 remain full-depth at audit rate 0 and are bound by raw experiment identity, result digest, aggregate hashes, and finalizer cross-checks. |
| Verification | v2 admission and v2 evaluator/finalizer hermetic suites PASS; v1 canary/evidence regressions PASS; relevant shell syntax/size gates and focused Go tests PASS; independent Guardian review reports P0/P1 `0/0` and closes all three prior P1 findings. |

At this freeze-time checkpoint, the decision was `AWAITING_EXPLICIT_FULL_EVALUATION_AUTHORIZATION`. No authorization receipt, reservation, runtime claim, primary ledger, evaluator artifact, replay ledger, terminal outcome, provider call, activation, promotion, or implementation claim existed for v2. The later exact-identity authorization admitted only the ordered sequence `primary 58 → provider-zero evaluator → gated replay 5 → provider-zero finalizer`; its completed terminal state follows.

## GPT/Codex Full-Evaluation v2 Terminal State

The user bound explicit single-use authorization to exact identity `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` at `2026-07-15T17:27:52+09:00`. The admitted sequence ran in the required order and completed without a retry, tool call, repository file modification, or circuit break.

| Field | Terminal result |
|---|---|
| Authorization and reservation | Authorization `sha256:875d17bf450b78fa656263f7f5676cd56b36d5ea33748e34d9f81efa861ac677`; primary reservation `sha256:bd733533ab255957154e2b31cd905c165c6f0c9719671ace7909116a620e648e`; single-use, concurrency 1, retries 0. |
| Primary | 58 of 58 PASS: baseline 35 and candidate 23; 44 `xhigh` and 14 `max`; 58 actual-usage receipts; 1,071,031 observed raw tokens; tools/files modified/retries `0/0/0`; circuit `CLOSED`. |
| Provider-zero evaluator | Provider calls 0; measurement and neutrality gates PASS; 14 paired trials, quality 7/7, security 14/14, task005 audit PASS, high006 and critical009 PASS, median paired raw-token reduction 59.918%, and high/critical regressions 0. Decision `ELIGIBLE_NEXT_CANARY`. |
| Applied rollback | Fault-injected decision `ROLLBACK`; atomic replace and fsync PASS; active profile and state readback both `full_ultra`. Receipt `sha256:a00bed018f5c5f4482fcec2dacdd5a07c82f43e5e936ecf27e7042fe9d482def`. |
| Replay | Nested reservation `sha256:032eec05a180733e14a0fccf0f375869b74c490010e799d5e5ddba2bbfec4edb`; 5 of 5 PASS, four `xhigh` plus one `max`, 90,735 raw tokens, tools/files modified/retries `0/0/0`. |
| Combined envelope | 63 provider calls and 1,161,766 observed raw tokens under caps of 64 and 1,500,000; 338,234 raw-token observed margin; no 64th call. |
| Terminal and closure | Terminal `sha256:982863631858aad2a8eafc8eec4aa23218cf66b83110aed340fceb590098f376`, success true, terminal state `ELIGIBLE_NEXT_CANARY`; closure `sha256:a2e7af9b2fee116318316864fcae4802da53ec2843f4b648ab15c8523d8ade7c`, consumed true and reusable false. Raw retained false; exact full-evaluation partial-ledger count 0. |
| Independent review | Guardian final reports P0/P1 `0/0`, S24/S26/S27 PASS, CD-02/CD-03 resolved, and blockers 0. |

`ELIGIBLE_NEXT_CANARY` means that the bounded evidence gate passed. It does not mean that the adaptive policy was promoted or activated. The terminal and closure artifacts intentionally retain `promotion_eligible=false`, `activation_eligible=false`, and `implemented=false`; no user configuration or repository policy was changed, and the effective profile remains `full_ultra`. After the terminal chain and independent Guardian review, the SPEC/document lifecycle is `completed` with lifecycle `implemented=true`. This lifecycle transition does not rewrite the experiment artifacts or enable the candidate policy.

### Full-evaluation v2 terminal evidence index

| Evidence | SHA-256 | Meaning |
|---|---|---|
| `evidence/gpt-full-evaluation-identity-v2.json` | `sha256:129521ff443c4ec01bc71cbb621c1dd3d515d5f460130a16732bf639b52e4978` | Frozen full-evaluation identity. |
| `evidence/gpt-full-evaluation-authorization-v2.json` | `sha256:875d17bf450b78fa656263f7f5676cd56b36d5ea33748e34d9f81efa861ac677` | Exact single-use user authorization and hard caps. |
| `evidence/gpt-full-evaluation-reservation-v2.json` | `sha256:bd733533ab255957154e2b31cd905c165c6f0c9719671ace7909116a620e648e` | Consumed primary plus replay envelope reservation. |
| `evidence/gpt-primary-call-ledger-v2.json` | `sha256:a8ceed2ebceae72bb120c84ecc6ace10e6ee0c2f4678c3eaa340ab86a3602aab` | Complete 58-call primary ledger. |
| `evidence/gpt-primary-evaluation-summary-v2.json` | `sha256:7ee81e2bf2f71ebc5d9a09471aa8b1f4b1e5513c0b69d60dfc161aff9a064f4e` | Provider-zero exact14, quality, security, efficiency, and replay eligibility summary. |
| `evidence/gpt-applied-rollback-v2.json` | `sha256:a00bed018f5c5f4482fcec2dacdd5a07c82f43e5e936ecf27e7042fe9d482def` | Applied atomic rollback and `full_ultra` readback. |
| `evidence/gpt-rollback-reservation-v2.json` | `sha256:032eec05a180733e14a0fccf0f375869b74c490010e799d5e5ddba2bbfec4edb` | Evaluator-gated nested replay reservation. |
| `evidence/gpt-rollback-call-ledger-v2.json` | `sha256:e3b3dd78cbffa28419b6f9eabd88a11799370959400083b5322e62979cfeeba5` | Complete five-call rollback replay ledger. |
| `evidence/gpt-full-evaluation-terminal-outcome-v2.json` | `sha256:982863631858aad2a8eafc8eec4aa23218cf66b83110aed340fceb590098f376` | Terminal success and non-activation boundary. |
| `evidence/gpt-full-evaluation-authorization-closure-v2.json` | `sha256:a2e7af9b2fee116318316864fcae4802da53ec2843f4b648ab15c8523d8ade7c` | Consumed, non-reusable authorization closure. |

## v0.2.1 GPT/Codex Context-Integrity Amendment

Revision 0.2.1 is an additive hardening amendment to the completed v0.2.0 lifecycle. It does not change the full-evaluation result, promote or activate the adaptive policy, mutate user or repository configuration, or replace the effective `full_ultra` profile.

The 800–2,000-token receipt limit applies only to handoff metadata and optional recall. Required GPT/Codex documents are a separate non-reducible frozen snapshot: `go` receives complete `spec.md`, `plan.md`, and `acceptance.md`, while complete four-document SPEC-review admission applies only when every selected provider is GPT/Codex and receives one coherent snapshot containing complete `spec.md`, `plan.md`, `research.md`, and `acceptance.md`. Mixed sets and Claude/Gemini-only review retain their legacy prompt behavior. Available architecture-profile documents and supervisor-declared task-specific references join each applicable snapshot. Required bodies are never trimmed, summarized, dropped, or charged to the receipt budget. Each required `spec.md` ID is bound to its containing SPEC directory. Raw secrets are redacted and prompt-injection directives are neutralized without discarding surrounding evidence; `source_hash` binds the original source bytes, `prompt_hash` binds the sanitized delivered bytes, and redaction metadata records the transformation. Serialized manifests carry no document body.

Snapshot construction occurs after final worktree assignment, and verification fails closed against the supervisor-held command, SPEC, conditional-profile, extra-reference set, and hashes. For an actual all-GPT `auto spec review`, every revision rebuilds and verifies context from the same supervisor-held `--required-document` and `--conditional-profile` declarations before constructing the provider prompt; the verified in-memory delivery contributes core, available architecture, selected conditional-profile, and extra-reference full bodies alongside the four SPEC documents exactly once rather than reloading or duplicating them. Missing, empty, unreadable, stale, directory-ID-mismatched, wrong-SPEC, incomplete, tampered, omitted-reference, replayed, wrong-set, reference-set-mismatched, or hash-mismatched context is rejected before compact selection or any provider call and retains `full_ultra`. A verified prompt above 128K is blocked or split rather than reduced. One frozen snapshot identity is reused for the admitted dispatch across retained direct Codex execution, every pipeline phase, and concurrent all-GPT/Codex review fan-out; the original task appears once in the first-phase input and is reattached once after later transitions. Native Codex delegation uses the current `spawn_agent(task_name, fork_turns="all", message)` schema and requests `context_ack` as diagnostic evidence, while supervisor-held reference/hash verification is the enforceable gate; legacy `agent_type` and `fork_context` fields do not satisfy this contract.

## Requirements

### Scope And Ownership

**REQ-UTE-SCOPE-01**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL implement canonical source changes inside `autopus-adk/`, SHALL regenerate installed platform surfaces through existing adapters or generators, and SHALL NOT directly edit meta-workspace generated, plugin-cache, or runtime artifacts.

**REQ-UTE-SCOPE-02**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL preserve Balanced behavior, user-owned model and provider configuration, hard workflow caps, tool-call budget semantics, mandatory deterministic gates, and non-Claude fallback behavior.

### Actual Usage And Accounting

**REQ-UTE-USAGE-01**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL define one versioned normalized UsageEnvelope for every eligible worker and orchestra provider invocation with stable run and call identity, task, attempt, provider, model, effort, phase, role, usage status, usage source, component inclusion semantics, actual cost, estimated cost, and sanitized source schema metadata.

**REQ-UTE-USAGE-02**  
Priority: Must  
Type: Event-driven

WHEN a provider exposes usage or cost data THEN THE SYSTEM SHALL normalize inclusive input, uncached input, cache read, cache creation, inclusive output, reasoning or thinking, tool tokens, actual cost, and source schema without storing raw prompt or response bodies in the usage receipt.

**REQ-UTE-USAGE-03**  
Priority: Must  
Type: Unwanted

IF a provider usage field is absent, inconsistent, or semantically ambiguous THEN THE SYSTEM SHALL serialize the actual field as null, SHALL record a machine-readable unavailable or conflict reason, and SHALL NOT infer zero or promote an estimate to actual.

**REQ-UTE-USAGE-04**  
Priority: Must  
Type: Event-driven

WHEN usage is aggregated across events, results, phases, retries, workers, and orchestra rounds THEN THE SYSTEM SHALL deduplicate identical run and call identities, SHALL count each retry with a distinct call identity, SHALL add reasoning, cache, or tool components only when provider metadata declares them separate from the inclusive totals, and SHALL persist the final normalized envelope exactly once through the explicit telemetry bridge owned by the supervisor or worker loop.

**REQ-UTE-USAGE-05**  
Priority: Must  
Type: Event-driven

WHEN telemetry summary, cost, comparison, or JSON output is rendered THEN THE SYSTEM SHALL expose actual coverage, raw totals, billable cost, estimates, model-call count, tool-call count, failed-task spend, accepted-task count, and raw tokens per accepted task as separate fields while remaining backward-compatible with legacy `estimated_tokens` events.

**REQ-UTE-USAGE-06**  
Priority: Must  
Type: Unwanted

IF an arm has zero accepted tasks or lacks actual-complete token semantics THEN THE SYSTEM SHALL return a null accepted-task efficiency value with a reason and SHALL NOT return zero or an efficiency claim.

### Thin Routing And Bounded Context

**REQ-UTE-ROUTER-01**  
Priority: Must  
Type: Event-driven

WHEN Claude or Gemini root auto routing surfaces are rendered THEN THE SYSTEM SHALL emit a route-only root no larger than 8,192 bytes, SHALL preserve every supported command route and mandatory policy, and SHALL resolve exactly one detailed command contract for the selected route.

**REQ-UTE-ROUTER-02**  
Priority: Must  
Type: Event-driven

WHEN an auto subcommand is selected THEN THE SYSTEM SHALL load the core workspace policy plus only the command context profile, SHALL use budgeted recall for relevant evidence, and SHALL NOT load scenarios, canary, signatures, learnings, or unrelated product documents outside their declared profiles.

**REQ-UTE-CONTEXT-01**  
Priority: Must  
Type: Event-driven

WHEN a supervisor delegates a task THEN THE SYSTEM SHALL select a context-receipt metadata-and-optional-recall budget between 800 and 2,000 estimated tokens, SHALL produce metadata and optional recall no larger than the selected budget containing the Outcome Lock, constraints, owned paths, forbidden paths, acceptance criteria, required references, current decision delta, snapshot hash, and prompt-manifest hash, and SHALL allocate only the remaining budget to optional recall while keeping complete required-document bodies outside that budget.

**REQ-UTE-CONTEXT-02**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL classify policy and command instructions as stable prompt layers, selected project and SPEC evidence as frozen snapshot layers, and user request, provider output, and retry state as ephemeral layers so dynamic changes do not invalidate unrelated stable hashes.

**REQ-UTE-CONTEXT-03**
Priority: Must
Type: Event-driven

WHEN required context is resolved for a GPT/Codex `worker` or `go` dispatch or for a SPEC-review provider set composed entirely of GPT/Codex identities THEN THE SYSTEM SHALL load full untrimmed, unsummarized, non-omittable required-document bodies outside the selected 800–2,000-token receipt metadata-and-optional-recall budget, SHALL require `spec.md`, `plan.md`, and `acceptance.md` for `go` plus one coherent `spec.md`, `plan.md`, `research.md`, and `acceptance.md` snapshot for the all-GPT/Codex review set while preserving legacy review behavior for mixed Codex-plus-Claude, Claude-only, and Gemini-only sets, SHALL include available architecture context for `go` and applicable review plus supervisor-declared task-specific references, SHALL rebuild and verify the actual `auto spec review` delivery on every revision from the supervisor-held `--required-document` and `--conditional-profile` sets, SHALL inject verified core, available architecture, conditional-profile, extra-reference, and four-SPEC-document full bodies exactly once into the provider prompt without reloading or duplicating the SPEC documents, SHALL bind every required `spec.md` ID to its containing SPEC directory, SHALL redact raw secrets and neutralize prompt-injection directives while preserving surrounding evidence, SHALL bind original bytes to `source_hash` and sanitized delivered bytes to `prompt_hash` with redaction metadata and `complete=true` in a body-free serialized manifest, SHALL build one execution snapshot after final worktree assignment and reuse that identity across retained direct Codex execution, every pipeline phase, and concurrent all-GPT/Codex review dispatch while carrying the original task exactly once without first-phase duplication, SHALL use the current native Codex `spawn_agent(task_name, fork_turns="all", message)` schema and request diagnostic `context_ack` while enforcing the supervisor-held command, SPEC, conditional-profile, extra-reference-set, and hash match, SHALL reject missing, empty, unreadable, stale, directory-ID-mismatched, wrong-SPEC, incomplete, tampered, omitted-reference, replayed, wrong-set, reference-set-mismatched, or hash-mismatched context before compact selection or any provider call while retaining `full_ultra`, and SHALL block or split a verified prompt above 128K instead of trimming, summarizing, or dropping required content.

### Typed Proactive Pruning

**REQ-UTE-PRUNE-01**  
Priority: Must  
Type: Event-driven

WHEN a phase payload has passed its direct consumer and contains more than two completed successful tool call/result pairs THEN THE SYSTEM SHALL keep the two most recent pairs and SHALL replace each older successful pair body with status, digest, artifact reference, source references, and a bounded evidence excerpt before the existing hard compaction threshold.

**REQ-UTE-PRUNE-02**  
Priority: Must  
Type: Unwanted

IF context contains a failure, unresolved finding, user correction, security or migration invariant, acceptance criterion, decision, file reference, incomplete tool pair, or provider-required reasoning signature THEN THE SYSTEM SHALL preserve the content or a lossless stable artifact reference and SHALL fail closed when pair or preservation integrity cannot be established.

**REQ-UTE-PRUNE-03**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL install the existing structured compressor in production worker phase transitions, SHALL retain hard compaction as an overflow fallback, and SHALL emit compaction events for both soft pruning and hard summarization without retaining raw secrets or privileged absolute paths.

### Evidence-Adaptive Ultra

**REQ-UTE-POLICY-01**  
Priority: Must  
Type: Unwanted

IF changed-path discovery, risk evidence, context integrity, or workflow binding is missing, malformed, ambiguous, high, critical, sensitive, or unknown THEN THE SYSTEM SHALL select the current full Ultra review profile before dispatch and SHALL record the exact fallback reason.

**REQ-UTE-POLICY-02**  
Priority: Must  
Type: State-driven

WHERE Ultra is active for a task deterministically classified as low or medium risk, non-sensitive, scoped, and not selected for full-depth audit THEN THE SYSTEM SHALL bind one reviewer vote, mandatory security, and no synthesis before dispatch while retaining the current implementation fan-out, retry, model, effort, build, test, acceptance, coverage, and release contracts.

**REQ-UTE-POLICY-03**  
Priority: Must  
Type: Event-driven

WHEN deterministic rollout sampling selects an eligible low or medium task for full-depth audit THEN THE SYSTEM SHALL bind the current full Ultra review profile before dispatch and SHALL record `audit_sample` as the selection reason.

**REQ-UTE-POLICY-04**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL preserve the existing implementation ownership grouping, hard fan-out cap of five, and scheduling semantics and SHALL NOT lower or adapt implementation fan-out in this SPEC.

**REQ-UTE-POLICY-05**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL retain the current canonical Ultra model and effort profile for every phase and role and SHALL preserve user-owned custom or pinned provider configuration.

### Evidence, Promotion, And Optional Provider Efficiency

**REQ-UTE-EVAL-01**  
Priority: Must  
Type: Unwanted

IF A/A actual-complete call coverage is below 95 percent or instrumentation changes objective output, call policy, or acceptance THEN THE SYSTEM SHALL block behavior-changing policy promotion and SHALL report the missing provider and execution-path coverage.

**REQ-UTE-EVAL-02**  
Priority: Must  
Type: Event-driven

WHEN baseline and candidate arms are compared THEN THE SYSTEM SHALL pair only common task identities with matching provider, model version, effort policy, risk policy, and cache stratum, SHALL include every eligible attempt in spend, and SHALL report unpaired or excluded tasks explicitly.

**REQ-UTE-EVAL-03**  
Priority: Must  
Type: Event-driven

WHEN an adaptive policy is considered for promotion THEN THE SYSTEM SHALL report the provisional 25 percent median raw-token target as an experiment result, SHALL require zero new high or critical objective and security regressions, and SHALL roll back on quality, measurement, policy-parity, context-integrity, or registered reliability failure regardless of token savings.

### GPT/Codex Operational Evidence

**REQ-UTE-CANARY-01**
Priority: Must
Type: Ubiquitous

THE SYSTEM SHALL collect new v0.2.0 live completion evidence only from `gpt-5.6-sol` through `auto agent run` and the worker Codex adapter while preserving all existing non-Codex hermetic compatibility and generated-parity tests.

**REQ-UTE-CANARY-02**
Priority: Must
Type: Event-driven

WHEN the frozen canary is admitted THEN THE SYSTEM SHALL require the exact 12-task corpus hash and 12-of-12 deterministic target preflight, SHALL execute the exact seven-task AB/BA cohort and full-or-compact tuples defined above, and SHALL record all excluded tasks and the authorization reason.

**REQ-UTE-CANARY-03**
Priority: Must
Type: Unwanted

IF a live call is not read-only and ephemeral, exposes a tool event, retains a raw provider body, lacks normalized actual usage, or has ambiguous provider, model, effort, config, identity, or schema evidence THEN THE SYSTEM SHALL open the circuit without retry and SHALL make the arm ineligible for promotion.

**REQ-UTE-CANARY-04**
Priority: Must
Type: Ubiquitous

THE SYSTEM SHALL pre-admit a 63-call and 1,446,000-raw-token hard envelope under the 64-call and 1,500,000-token authorization with concurrency one and zero retries, SHALL reserve each call against its 22,000 `xhigh` or 26,000 `max` rollout budget, and SHALL run the five-call applied rollback replay after every prior gate passes and the circuit breaker remains closed.

**REQ-UTE-CANARY-05**
Priority: Must
Type: Event-driven

WHEN v0.2.0 promotion is evaluated THEN THE SYSTEM SHALL require 14 paired trials, seven complete quality rows, authoritative deterministic patch/test/security receipts, mandatory security, exact audit linkage, full audit/high/critical profiles, zero regressions, and the provisional 25 percent median target, and SHALL prove isolated atomic rollback to `full_ultra` before any implementation claim.

## Compatibility And Contract Revisions

- `SPEC-COMPRESS-001` remains the authority for structured hard compaction. Its below-threshold full-pass contract is narrowed so completed stale tool pairs may be losslessly pruned before 50% while protected evidence remains intact.
- `SPEC-HARNESS-WORKFLOW-TEAM-001` remains the authority for the full Ultra depth tuple. Its three-vote plus synthesis shape becomes the immediate high/critical/sensitive/unknown, audit, and binding-failure profile, while eligible low/medium work may receive the existing one-vote/no-synthesis depth before dispatch.
- `SPEC-CODEXQUAL-001` remains the authority for canonical managed model and effort tuples. This SPEC does not lower model or effort for any Ultra phase or role.
- `SPEC-BUDGET-001` continues to count tool iterations. Token usage accounting is additive observability and does not reinterpret the tool-call budget as a token budget.

## Acceptance Criteria

- [x] S1–S6 prove normalized actual, null, deduplication, backward compatibility, accepted-task denominator, and cache/raw arithmetic.
- [x] S7–S10 prove thin routing, context-profile selection, bounded receipts, and stable/snapshot/ephemeral hash behavior.
- [x] S11–S13 prove proactive pruning, protected evidence, production compressor wiring, and event metadata.
- [x] S14–S18 prove conservative risk, compact pre-dispatch binding, audit/failure fallback, unchanged fan-out, and canonical model/effort profiles.
- [x] S19–S22 prove A/A gating, paired evidence, zero-regression rollback, generated parity, and Balanced compatibility.
- [x] S23–S27 prove the exact GPT/Codex scope, corpus/cohort, call arithmetic, evidence safety, budget admission, quality ledger, and applied rollback behavior.
- [x] S28 proves complete GPT/Codex required-document delivery, per-revision actual `auto spec review` build-and-verify from supervisor-held CLI sets, exactly-once core/architecture/conditional/extra/SPEC prompt composition, all-GPT/Codex-only four-document review admission with legacy mixed/Claude/Gemini behavior, SPEC-directory ID binding, evidence-preserving secret redaction and injection neutralization with raw-source versus sanitized-prompt hashes, one snapshot identity across phases/concurrent review, fail-closed zero-call admission, original-task continuity, current native spawn syntax, diagnostic `context_ack`, and the 128K block-or-split boundary.

The historical v1 attempt proves the S25 non-PASS circuit-breaker branch. The later v2 chain additionally completes all 58 primary calls, exact14 evaluation, the five-call replay, and the applied rollback/readback proof. Guardian independently accepted S24, S26, and S27.

The provider-zero freeze remains the historical admission checkpoint. The subsequent exact authorization, primary, evaluator, replay, terminal, and closure receipts close the aggregate S23–S27 live evidence item without activating or promoting the candidate policy.

Final implementation verification also closes S1–S22 and Edge Cases 1–3. Broad race/coverage, vet, build, full Go tests, frozen-Auto architecture enforcement, changed-Go formatting, strict SPEC validation, scratch template generation, compatibility, and hygiene gates all passed. Guardian approved explicit aggregate-coverage exceptions for `pkg/worker` at 78.7%, `pkg/worker/host` at 61.1%, and `pkg/adapter/gemini` at 84.7% because the changed paths have direct targeted tests and the broad race/full gates pass; the exceptions do not waive changed-path regression coverage. The final Codex matrix debt was test-only: the oracle now distinguishes root supervisor Sol/`ultra` from managed orchestra Sol/`max`, production code is unchanged, targeted RED→GREEN and full gates pass, and the matrix review reports P0/P1/P2 `0/0/0`.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|---|---|---|---|
| REQ-UTE-SCOPE-01, REQ-UTE-SCOPE-02 | T11, T12 | S22 | INV-COMPAT-01 |
| REQ-UTE-USAGE-01, REQ-UTE-USAGE-02 | T1, T2 | S1, S2, Edge Case 2 (AC-024) | INV-USAGE-01 |
| REQ-UTE-USAGE-03 | T1, T2 | S3 | INV-USAGE-02 |
| REQ-UTE-USAGE-04 | T3, T4 | S4, Edge Case 1 (AC-023), Edge Case 2 (AC-024) | INV-USAGE-03 |
| REQ-UTE-USAGE-05, REQ-UTE-USAGE-06 | T3 | S5, S6 | INV-EVAL-03, INV-CACHE-01 |
| REQ-UTE-ROUTER-01 | T5 | S7 | INV-ROUTER-01 |
| REQ-UTE-ROUTER-02, REQ-UTE-CONTEXT-01 | T6 | S8, S9 | INV-CONTEXT-01 |
| REQ-UTE-CONTEXT-02 | T6 | S10 | INV-CONTEXT-02 |
| REQ-UTE-CONTEXT-03 | T14 | S28 | INV-CONTEXT-03 |
| REQ-UTE-PRUNE-01 | T7 | S11 | INV-PRUNE-01 |
| REQ-UTE-PRUNE-02, REQ-UTE-PRUNE-03 | T7 | S12, S13 | INV-PRUNE-02 |
| REQ-UTE-POLICY-01 | T8, T9 | S14, Edge Case 3 (AC-025) | INV-POLICY-01 |
| REQ-UTE-POLICY-02 | T9 | S15 | INV-POLICY-02 |
| REQ-UTE-POLICY-03 | T8, T9 | S16 | INV-POLICY-03 |
| REQ-UTE-POLICY-04 | T8, T9 | S17 | INV-FANOUT-01 |
| REQ-UTE-POLICY-05 | T8, T9 | S18 | INV-COMPAT-01 |
| REQ-UTE-EVAL-01 | T10 | S19 | INV-EVAL-01 |
| REQ-UTE-EVAL-02 | T10 | S20 | INV-EVAL-02 |
| REQ-UTE-EVAL-03 | T10, T11 | S21 | INV-QUALITY-01, INV-ROLLBACK-01 |
| REQ-UTE-CANARY-01 | T13 | S23, S25 | INV-CANARY-01, INV-CANARY-03 |
| REQ-UTE-CANARY-02 | T13 | S23, S24 | INV-CANARY-01, INV-CANARY-02 |
| REQ-UTE-CANARY-03 | T13 | S25 | INV-CANARY-03 |
| REQ-UTE-CANARY-04 | T13 | S24, S26 | INV-CANARY-02 |
| REQ-UTE-CANARY-05 | T13 | S27 | INV-CANARY-04, INV-ROLLBACK-01 |

### Extended Oracle Crosswalk

| Review-normalized ID | Canonical acceptance heading | Requirement | Plan Task |
|---|---|---|---|
| AC-023 | Edge Case 1: provider usage after the final message | REQ-UTE-USAGE-04 | T2, T3 |
| AC-024 | Edge Case 2: separate reasoning-token semantics | REQ-UTE-USAGE-02, REQ-UTE-USAGE-04 | T1 |
| AC-025 | Edge Case 3: changed-file discovery failure | REQ-UTE-POLICY-01 | T8 |

## Out of Scope

- A user-facing `Ultra Lite`, `Adaptive Ultra`, or equivalent new mode.
- Balanced behavior or naming changes.
- Provider price-table and billing-service modernization.
- Fixed token caps that can truncate required reasoning, tool use, or final output.
- Security auditor, deterministic build/test/coverage, release hygiene, or data-safety gate removal.
- High/critical or sensitive domain downshift.
- Within-run dynamic reviewer expansion based on model-produced confidence, disagreement, or finding schemas.
- Adaptive implementation fan-out, batching semantics, or model/effort downshift.
- Public 25% savings claim without actual paired accepted-task evidence.
- Provider-native compaction, tool search, cache pre-warming, and cost dashboard work not required by the mandatory requirements.
- Direct edits to installed generated surfaces in the meta workspace.
- Claude/Gemini live calls, live Claude `route_team` proof, or multi-provider consensus as v0.2.0 completion evidence. Their hermetic compatibility coverage remains in scope.

## Revision History

| Version | Date | Status | Change |
|---|---|---|---|
| 0.1.0 | 2026-07-11 | approved | Initial cross-provider implementation and evaluation contract. |
| 0.2.0 | 2026-07-12 | approved | Narrows new live operational proof to the GPT/Codex canonical path; freezes the seven-task paired cohort, budgets, safety contract, quality gates, and rollback proof without weakening non-Codex hermetic compatibility. Records the call-39 fail-closed result, consumed authorization, resolved cumulative-authorization P1, and final review PASS while keeping live Completion Debt open. |
| 0.2.0 | 2026-07-13 | approved | Records the diagnostic-only task006 authorization and call-1 transport failure, hardens nonzero-process receipts, adds an operational no-reuse receipt, and records the separate one-call transport-smoke failure without changing primary evidence, promotion, Completion Debt, or `full_ultra`. |
| 0.2.0 | 2026-07-14 | approved | Closes documentation for GPT/Codex transport diagnosis v2/v3: both single-use runs ended `process_nonzero` without root-cause evidence, current raw-free stage/signal hardening is hermetic-only evidence, and S24/S27 plus CD-02/CD-03 remain open under `full_ultra`. |
| 0.2.0 | 2026-07-14 | approved | Records the frozen v4 GPT/Codex preflight as unapproved, unconsumed, and unexecuted; explicit user live authorization remains required before its single transport-only call. |
| 0.2.0 | 2026-07-14 | approved | Records the authorized v4 single-call terminal failure, consumed non-resumable authorization, provider-failure-event source signal without classified detail, and unchanged fail-closed rollout. |
| 0.2.0 | 2026-07-14 | approved | Records the frozen v5 GPT/Codex event-shape preflight as unapproved, unconsumed, and provider-zero; explicit user authorization remains required. |
| 0.2.0 | 2026-07-15 | approved | Records the authorized v5 single-call terminal failure, consumed non-resumable authorization, raw-free provider event kind/shape, and unchanged fail-closed rollout. |
| 0.2.0 | 2026-07-15 | approved | Freezes the v6 raw-free per-event trait diagnosis package in provider-zero, approval-pending state; no Transport PASS, completion, or live-scope expansion is claimed. |
| 0.2.0 | 2026-07-15 | approved | Records the authorized v6 single-call terminal failure, consumed non-resumable authorization, raw-free `schema_or_response` coarse traits, and unchanged fail-closed rollout without claiming a root cause. |
| 0.2.0 | 2026-07-15 | approved | Records v7's freeze-time provider-zero approval-pending state with a conservative Structured Outputs schema and pre-claim runtime-copy validation; the v6 schema mismatch remains a strong hypothesis, not a confirmed root cause. |
| 0.2.0 | 2026-07-15 | approved | Records the exact-authorized v7 one-call terminal `missing_or_invalid_result`, consumed non-resumable identity, unavailable usage, absent canonical receipts, and unchanged fail-closed rollout without confirming the schema hypothesis. |
| 0.2.0 | 2026-07-15 | approved | Freezes v8 provider-zero after hermetically reproducing the omitted-empty-array consumer mismatch and applying a missing-to-empty-only fix; historical v7 causality remains unconfirmed and exact v8 authorization is pending. |
| 0.2.0 | 2026-07-15 | approved | Records v8 terminal Transport PASS with one canonical actual-usage receipt and schema conformance while preserving semantic/promotion/implementation boundaries and requiring separate frozen admission, budget, and authorization for any full 58+5 evaluation. |
| 0.2.0 | 2026-07-15 | approved | Freezes the separate v2 full-evaluation identity, 63-call/1,446,000-token schedule, full17/admission9 source chain, exact evaluator/replay/finalizer bindings, provider-zero verification, and P0/P1 `0/0` review while leaving exact authorization and all live Completion Debt open. |
| 0.2.0 | 2026-07-15 | completed | Records the exact-authorized 58-call primary, provider-zero exact14 evaluator, applied rollback/readback, five-call replay, terminal and non-reusable closure. Guardian accepts S24/S26/S27 and resolves CD-02/CD-03 with no promotion, activation, user-config mutation, or repository-policy mutation; `full_ultra` remains effective. |
| 0.2.0 | 2026-07-15 | completed | Closes final implementation verification: T1–T13, S1–S27, and Edge Cases 1–3 pass; broad build/race/full/architecture/strict/generation/hygiene gates pass; three explicit aggregate-coverage exceptions are review-approved; and the Codex matrix test-only oracle debt closes at P0/P1/P2 `0/0/0` with production unchanged. |
| 0.2.1 | 2026-07-15 | completed | Adds GPT/Codex complete-context integrity as REQ-UTE-CONTEXT-03/T14/S28/INV-CONTEXT-03: actual all-GPT `auto spec review` rebuilds and verifies the supervisor-held required-document/conditional-profile set every revision and composes core, available architecture, extra, and SPEC bodies once; complete four-document admission remains all-GPT/Codex-only while mixed/Claude/Gemini stays legacy; SPEC-directory ID, sanitization hashes, snapshot continuity, and zero-call failure gates remain verified. Policy promotion, activation, release, and the effective `full_ultra` outcome remain unchanged. |
