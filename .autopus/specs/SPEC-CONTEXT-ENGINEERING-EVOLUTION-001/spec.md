# SPEC-CONTEXT-ENGINEERING-EVOLUTION-001: Context engineering evolution

---
id: SPEC-CONTEXT-ENGINEERING-EVOLUTION-001
title: Context engineering evolution
version: 0.1.0
status: completed
priority: HIGH
---

## Purpose

Add measurable shadow selection, a consumed worker receipt parser, and
Claude/Gemini compiler/tool parity while preserving full delivery and defaults.

## Outcome Boundary

- Outcome Lock: observe a safe JIT candidate, consume explicit worker receipts,
  and generate opt-in compiler/tool-restricted Claude/Gemini surfaces without
  reducing active context or default skill availability.
- Mandatory: separate v2 sidecar, v1 invariance, exact five-field parser,
  optional marker rollout, compiler parity, honest tool projection, compact
  examples.
- Non-goals: provider history mutation, default reduction, active JIT,
  required-body shrink, scenario parser migration, orchestra inference.
- Completion evidence: S1-S8 and Edge Case 1-4 plus regression/review gates.

## Requirements

### Ubiquitous

REQ-PLAN-001: THE SYSTEM SHALL keep `active_mode=full` and emit JIT selection
only as a separate body-free `autopus.context_plan.v2` shadow sidecar.

REQ-COMPAT-001: THE SYSTEM SHALL preserve `autopus.context_delivery.v1`,
complete bodies, hashes, verification, architecture inclusion, worker delivery,
and binding behavior.

REQ-RECEIPT-001: THE SYSTEM SHALL define the canonical worker receipt body as
exactly `owned_paths`, `changed_files`, `verification`, `blockers`, and
`next_required_step`, with optional evidence outside that body in the versioned
envelope.

REQ-COMPILER-001: THE SYSTEM SHALL apply the existing catalog/compiler to
Claude and Gemini while preserving workflow/core routes and full defaults.

REQ-TOOLS-001: THE SYSTEM SHALL project supported source tools into Gemini
native `tools` frontmatter and omit unsupported capabilities rather than claim
enforcement, without changing Gemini policy, sandbox, approval, or settings.

REQ-EXAMPLE-001: THE SYSTEM SHALL provide no more than three compact examples
covering raw-body replay, malformed receipt evidence, and unsupported tools.

### Event-Driven

REQ-METRIC-001: WHEN a fresh memindex search succeeds THEN THE SYSTEM SHALL
compute pinned/selected refs, omitted count, full/JIT token estimates, delta,
reduction percentage, and optional labeled hit metrics from verified v1
metadata.

REQ-METRIC-UNAVAILABLE-001: WHEN the index is missing, stale, or corrupt THEN
THE SYSTEM SHALL return `status=unavailable`, null candidate metrics, and a
reason without blocking full delivery.

REQ-RECEIPT-PARSE-001: WHEN exactly one worker receipt marker is present THEN
THE SYSTEM SHALL strictly decode bounded versioned JSON and append its canonical
receipt to the pipeline run receipt, using the exact delimiters
`<!-- AUTOPUS_WORKER_RECEIPT_BEGIN -->` and
`<!-- AUTOPUS_WORKER_RECEIPT_END -->`.

### Unwanted

REQ-PRIVACY-001: IF a plan or receipt is serialized THEN THE SYSTEM SHALL NOT
include raw queries, document bodies, prompts, provider payloads, secrets, or
absolute local paths.

REQ-LEGACY-001: IF output has no receipt marker THEN THE SYSTEM SHALL preserve
legacy phase behavior without inventing a receipt.

REQ-MALFORMED-001: IF a marker is malformed, duplicated, oversized, contains
unknown/trailing data, or unsafe evidence refs THEN THE SYSTEM SHALL fail the
phase evidence gate, rejecting absolute, traversal, NUL-bearing, or non-clean
`owned_paths`, `changed_files`, and evidence refs. Parsed paths are metadata
only and do not authorize filesystem access.

REQ-PRUNE-001: IF opt-in compilation removes generated skills THEN THE SYSTEM SHALL
keep cleanup inside platform-owned skill roots and preserve unrelated files.

## Acceptance Criteria

The authoritative Must set is exactly S1-S8 and Edge Case 1-4 in
`acceptance.md`.

## Traceability Matrix

| Requirement | Tasks | Acceptance | Invariant |
|---|---|---|---|
| REQ-PLAN-001, REQ-COMPAT-001 | T1, T2 | S1-S3, Edge 1-2 | INV-001, INV-002 |
| REQ-METRIC-001, REQ-METRIC-UNAVAILABLE-001 | T1, T2 | S2-S3 | INV-002 |
| REQ-RECEIPT-001, REQ-RECEIPT-PARSE-001 | T3, T4 | S4-S5, Edge 3 | INV-003, INV-004 |
| REQ-LEGACY-001, REQ-MALFORMED-001 | T3, T4 | S5, Edge 3 | INV-004, INV-007 |
| REQ-COMPILER-001, REQ-PRUNE-001 | T5, T6 | S6, Edge 4 | INV-005 |
| REQ-TOOLS-001 | T5, T6 | S7, Edge 4 | INV-006 |
| REQ-EXAMPLE-001, REQ-PRIVACY-001 | T2, T4, T6 | S3, S8 | INV-007 |

## Related SPECs

- `SPEC-CONTEXT-ENGINEERING-001`
- `SPEC-SCENARIO-PARSER-MIGRATION-001`

## Completion

- Status: completed
- Completed: 2026-07-27
- Acceptance: S1-S8 and Edge Case 1-4 PASS
- Guardians: reviewer APPROVE; security SEC-001 through SEC-003 RESOLVED;
  AX scan completed with no behavior-changing annotation
- Verification: focused/full affected packages, race, vet, build, strict SPEC,
  architecture, generator determinism, and diff gates PASS
- Completion Debt: none. Active delivery and default compiler surface remain
  full by design; JIT/default reduction requires a separate promotion decision.
