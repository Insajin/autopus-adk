# SPEC-ADK-ULTRA-EFFICIENCY-001: Token-Efficient Ultra Quality Allocation

---
id: SPEC-ADK-ULTRA-EFFICIENCY-001
title: Token-Efficient Ultra Quality Allocation
version: 0.1.0
status: approved
priority: HIGH
---

**Created**: 2026-07-11  
**Source**: `BS-052`  
**Target module**: `autopus-adk/`  
**Depends On**: `SPEC-BUDGET-001`, `SPEC-COMPRESS-001`, `SPEC-CODEXQUAL-001`, `SPEC-HARNESS-WORKFLOW-TEAM-001`, `SPEC-ACCGATE-002`  
**Narrowly revises**: `SPEC-COMPRESS-001` below-threshold pass-through, `SPEC-HARNESS-WORKFLOW-TEAM-001` Ultra review depth  

## Purpose

Make Ultra the highest-confidence Autopus mode without treating every task as if it requires the maximum prompt, context, reviewer count, and synthesis pass. The system first measures provider usage, removes repeated fixed context, and then uses conservative pre-dispatch risk evidence to choose either a compact premium review lane or the current full Ultra review profile. Model, effort, implementation fan-out, security, and deterministic quality gates remain unchanged.

## Background

The current implementation contains the right foundations but they are not connected into a token-efficiency contract:

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
- **Completion evidence**: strict SPEC validation, focused Go tests with 85%+ coverage in changed packages, rendered-surface size/parity oracles, A/A measurement evidence, paired accepted-task report, high/critical zero-regression report, full-depth audit sample, and rollback receipt.

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
- **Context receipt**: bounded task-specific snapshot containing outcome, constraints, ownership, acceptance, references, decision delta, and manifest hashes while keeping original artifacts available by stable reference.
- **Policy promotion**: movement from shadow to a larger canary stage after deterministic usage and quality gates pass.

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

WHEN a supervisor delegates a task THEN THE SYSTEM SHALL select a total context-receipt budget between 800 and 2,000 estimated tokens, SHALL produce a receipt no larger than the selected budget containing the Outcome Lock, constraints, owned paths, forbidden paths, acceptance criteria, required references, current decision delta, snapshot hash, and prompt-manifest hash, and SHALL allocate only the remaining budget to optional recall.

**REQ-UTE-CONTEXT-02**  
Priority: Must  
Type: Ubiquitous

THE SYSTEM SHALL classify policy and command instructions as stable prompt layers, selected project and SPEC evidence as frozen snapshot layers, and user request, provider output, and retry state as ephemeral layers so dynamic changes do not invalidate unrelated stable hashes.

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

## Compatibility And Contract Revisions

- `SPEC-COMPRESS-001` remains the authority for structured hard compaction. Its below-threshold full-pass contract is narrowed so completed stale tool pairs may be losslessly pruned before 50% while protected evidence remains intact.
- `SPEC-HARNESS-WORKFLOW-TEAM-001` remains the authority for the full Ultra depth tuple. Its three-vote plus synthesis shape becomes the immediate high/critical/sensitive/unknown, audit, and binding-failure profile, while eligible low/medium work may receive the existing one-vote/no-synthesis depth before dispatch.
- `SPEC-CODEXQUAL-001` remains the authority for canonical managed model and effort tuples. This SPEC does not lower model or effort for any Ultra phase or role.
- `SPEC-BUDGET-001` continues to count tool iterations. Token usage accounting is additive observability and does not reinterpret the tool-call budget as a token budget.

## Acceptance Criteria

- [ ] S1–S6 prove normalized actual, null, deduplication, backward compatibility, accepted-task denominator, and cache/raw arithmetic.
- [ ] S7–S10 prove thin routing, context-profile selection, bounded receipts, and stable/snapshot/ephemeral hash behavior.
- [ ] S11–S13 prove proactive pruning, protected evidence, production compressor wiring, and event metadata.
- [ ] S14–S18 prove conservative risk, compact pre-dispatch binding, audit/failure fallback, unchanged fan-out, and canonical model/effort profiles.
- [ ] S19–S22 prove A/A gating, paired evidence, zero-regression rollback, generated parity, and Balanced compatibility.

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
