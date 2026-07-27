# SPEC-CONTEXT-ENGINEERING-001: 네 플랫폼 generated context contract 정렬

---
id: SPEC-CONTEXT-ENGINEERING-001
title: 네 플랫폼 generated context contract 정렬
version: 0.2.0
status: completed
priority: HIGH
---

## Purpose

Claude Code, Codex, OpenCode, Gemini/Antigravity adapter가 동일한 command-selected document matrix, safe-reference JIT guidance, condensed worker return 계약을 생성하도록 정렬한다.

## Background

`pkg/promptlayer`는 verified supervisor delivery를, `pkg/memindex`는 bounded optional recall receipt를, `pkg/worker/compress`는 structured compaction을 이미 제공한다. Claude의 installed selector도 이미 `templates/claude/commands/auto-router.md.tmpl`에서 생성된 `.claude/skills/auto/SKILL.md`이며, `pkg/adapter/claude/claude_workflow_skills.go`가 route별 detail을 생성한다. `templates/claude/commands/auto-workflows.md.tmpl` 상단은 section extraction을 위한 generation-only source이고 installed selection을 소유하지 않는다.

## Outcome Boundary

- Outcome Lock: 네 adapter의 scratch generated effective paths가 acceptance의 exact command/document matrix와 safe JIT/condensed-return contract를 제공하고 기존 delivery·doctor·scenario 호환성을 보존한다.
- Mandatory requirements: Claude one-detail proof, four-adapter matrix oracle, safe project-relative JIT guidance, canonical five-field owner parity, supervisor/worker layer 구분, doctor symbol/ID stability, scenario executable-wire preservation.
- Explicit non-goals: runtime ContextPlan/JIT enforcement, provider-native history/tool control, default skill pruning, full required-body delivery 축소, generation-only `auto-workflows` preamble의 runtime selector 승격.
- Completion evidence: S1-S8와 Edge Case 1-3, two planned tests, focused adapter/promptlayer/doctor tests, strict SPEC validation, diff hygiene.

## Requirements

### Ubiquitous

REQ-SELECT-001: THE SYSTEM SHALL generate the exact required, delegated-worker optional, and default-excluded document sets defined by the canonical command matrix for Claude, Codex, OpenCode, and Gemini/Antigravity effective paths.

REQ-CLAUDE-001: THE SYSTEM SHALL preserve the generated Claude thin router that references each route detail exactly once, generate route details through `claude_workflow_skills.go`, and SHALL NOT install the generation-only `auto-workflows.md` source.

REQ-DELIVERY-001: THE SYSTEM SHALL distinguish supervisor verified delivery from delegated-worker optional recall: `go` SHALL include every available architecture document and resolved core/SPEC documents as complete prompt bodies outside the receipt, while worker recall SHALL be limited to signature, learning, and task-declared extra refs without duplicating required bodies.

REQ-RETURN-001: THE SYSTEM SHALL preserve exactly `owned_paths`, `changed_files`, `verification`, `blockers`, and `next_required_step` as worker receipt fields in the existing canonical owners and SHALL add condensed/JIT evidence only as guidance, not as a replacement field schema.

REQ-SCENARIO-001: THE SYSTEM SHALL preserve top-level `Build` and runnable scenario `Command`, `Verify`, and `Status` in the active executable wire and SHALL forbid index-only replacement.

REQ-DOCTOR-001: THE SYSTEM SHALL keep exported `ContextLoadSet`, `doctor.context_weight.total`, and `doctor.context_weight.doc.<name>` unchanged while context-weight text remains advisory.

REQ-HYGIENE-001: THE SYSTEM SHALL keep scratch generation artifacts temporary and SHALL NOT modify or stage generated/runtime artifacts or unrelated work-in-progress files.

### Event-Driven

REQ-JIT-001: WHEN generated guidance tells a worker to retrieve optional detail THEN THE SYSTEM SHALL require a clean project-relative reference, reject absolute paths, `..` traversal, symlinks, and non-regular files, sanitize/redact content while preserving injection evidence, and record selected refs/hashes plus omitted count.

REQ-PARITY-001: WHEN full-mode scratch roots are generated THEN THE SYSTEM SHALL expose the same normalized matrix and additive JIT/condensed semantics at every effective path, including the Antigravity plugin mirror.

### Unwanted

REQ-RAW-001: IF a raw tool result, provider payload, required document body, or repeated artifact body has a stable artifact reference THEN THE SYSTEM SHALL NOT replay it in delegated worker input or return.

REQ-COMPAT-001: IF existing full delivery or doctor consumers run THEN THE SYSTEM SHALL preserve `autopus.context_delivery.v1`, complete body/hash verification, available architecture auto-inclusion, `ContextLoadSet`, doctor JSON IDs, and overall advisory health behavior.

## Acceptance Criteria

The authoritative Must set is exactly S1-S8 and Edge Case 1-3 in `acceptance.md`; every item is required by the Exit Criteria.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|---|---|---|---|
| REQ-SELECT-001 | T1, T2 | S2, Edge Case 1 | INV-001 |
| REQ-CLAUDE-001 | T1, T2 | S1 | INV-002 |
| REQ-DELIVERY-001 | T1, T2, T5 | S2, S7, Edge Case 1, Edge Case 2 | INV-003, INV-007 |
| REQ-RETURN-001 | T1, T3 | S3, Edge Case 3 | INV-005 |
| REQ-SCENARIO-001 | T4 | S5 | INV-004 |
| REQ-DOCTOR-001 | T1, T4 | S6 | INV-009 |
| REQ-HYGIENE-001 | T1, T5 | S8 | INV-010 |
| REQ-JIT-001 | T1, T3 | S4, Edge Case 2 | INV-006 |
| REQ-PARITY-001 | T1, T2, T3 | S2, S4 | INV-001 |
| REQ-RAW-001 | T3 | S3, S4, Edge Case 3 | INV-008 |
| REQ-COMPAT-001 | T4, T5 | S6, S7 | INV-003, INV-009 |

## Related SPECs

None. The Primary SPEC closes one generated-surface contract and does not create a sibling SPEC.

## Completion

- Status: completed
- Completion Debt: none
- Evidence: S1-S8와 Edge Case 1-3, four-adapter/Antigravity scratch generation, focused and race tests, full adapter/template/doctor regressions, vet, build, strict SPEC, architecture, diff hygiene, reviewer verification, security verification PASS
