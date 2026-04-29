# SPEC-ACCGATE-002: Semantic invariant acceptance gate

**Status**: completed
**Created**: 2026-04-29
**Completed**: 2026-04-29
**Domain**: ACCGATE
**Target module**: autopus-adk

## Purpose

This SPEC hardens Autopus planning and implementation workflows after the workflow-fair harness comparison. The direct failure was not lack of SPEC files. The failure was that the generated SPEC did not preserve the original task's semantic invariant in acceptance criteria and tests. The benchmark also exposed that `/auto go` can appear to run a full pipeline while no subagent dispatch actually happens in a non-interactive Claude Code setting.

The fix is to add a semantic invariant layer to SPEC generation, testing, and validation, and to require observable workflow authenticity evidence from `/auto go`.

## Definitions

- **Semantic invariant**: a domain rule from the original task that must remain true for the feature to be correct, such as pairing by common IDs, preserving ordering, deduplicating with last-row wins, grouping by one key while comparing across another key, or matching exact numeric formulas.
- **Oracle acceptance**: an acceptance scenario that states concrete input and expected observable output values, not only file existence, section headings, or successful exit.
- **Structural-only test**: a test that verifies headings, non-empty output, `NoError`, `NotNil`, or file creation without validating the semantic content required by a Must acceptance criterion.
- **Workflow authenticity evidence**: recorded or reported proof that the requested execution mode actually ran, such as subagent dispatch count, agent names, phase-to-agent mapping, or an explicit degraded-mode label.

## Requirements

- **REQ-001** — EARS type: Event-driven / Priority: Must
  WHEN `spec-writer` converts a user request into SPEC files, THEN THE SYSTEM SHALL extract a `Semantic Invariant Inventory` from the original request and write it to `research.md` with traceable source clauses.

- **REQ-002** — EARS type: Event-driven / Priority: Must
  WHEN the inventory contains paired, comparative, grouping, ordering, deduplication, or cross-entity semantics, THEN THE SYSTEM SHALL create at least one Must acceptance scenario that uses heterogeneous entities and concrete expected outputs.

- **REQ-003** — EARS type: Event-driven / Priority: Must
  WHEN a Must requirement defines a numeric formula, statistical method, parser rule, matching rule, or report row contract, THEN THE SYSTEM SHALL include oracle acceptance assertions for exact values or explicit tolerances.

- **REQ-004** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL extend `content/rules/spec-quality.md` with a completeness check that fails when semantic invariants from `research.md` do not map to requirements, plan tasks, and acceptance criteria.

- **REQ-005** — EARS type: Event-driven / Priority: Must
  WHEN Phase 1.5 or Phase 3 tester work derives tests from acceptance criteria, THEN THE SYSTEM SHALL instruct the tester to create behavior assertions for every Must oracle acceptance criterion and reject structural-only tests for those criteria.

- **REQ-006** — EARS type: Event-driven / Priority: Must
  WHEN Gate 2 validator performs acceptance coverage verification, THEN THE SYSTEM SHALL fail validation if a Must oracle acceptance criterion lacks a corresponding test that asserts the required semantic output.

- **REQ-007** — EARS type: Event-driven / Priority: Must
  WHEN `/auto go` selects the default subagent pipeline route, THEN THE SYSTEM SHALL preflight that the required subagent surface is available before Phase 1 and record subagent dispatch evidence before completion.

- **REQ-008** — EARS type: Unwanted / Priority: Must
  IF the default subagent pipeline route cannot create or observe subagent dispatch, THEN THE SYSTEM SHALL stop with a workflow authenticity blocker unless the user explicitly selected `--solo`.

- **REQ-009** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL keep source-of-truth changes in `content/` and `templates/`, and SHALL NOT modify generated `.claude`, `.codex`, `.gemini`, `.opencode`, `.agents`, or `.autopus/plugins` output directly.

- **REQ-010** — EARS type: Event-driven / Priority: Should
  WHEN generated platform surfaces are tested, THEN THE SYSTEM SHALL include template regression checks that verify semantic-invariant and workflow-authenticity instructions appear in Claude, Codex, Gemini, and OpenCode surfaces.

## In Scope

- Prompt and rule updates in `autopus-adk/content/**`.
- Platform template updates in `autopus-adk/templates/**`.
- Template or content tests that verify generated instructions are present.
- No direct changes to `autopus-harness-bench`.

## Out of Scope

- Re-running the full 5-harness benchmark matrix.
- Adding a domain-specific statistics oracle engine to ADK.
- Changing provider API credentials, Claude Code installation, or benchmark shell wrappers.

## Related SPECs

- SPEC-ACCGATE-001 — existing acceptance lifecycle, parser, and prompt injection foundation.
- SPEC-SPECWR-001 — spec-writer self-checklist prompt-layer foundation.
- SPEC-SPECWR-002 — runtime injection of the SPEC quality checklist into `auto spec review`.
- SPEC-HARN-BENCH-008 — claim-scale benchmark guardrails that surfaced the need for objective verifier behavior.

## Open Issues

None.
