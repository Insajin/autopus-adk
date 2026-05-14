# SPEC-ADK-IDEA-CLARIFY-001: Deep Interview Clarification Gate For Auto Idea

**Status**: completed
**Created**: 2026-05-14
**Synced**: 2026-05-15
**Domain**: ADK-IDEA-CLARIFY
**Priority**: HIGH
**Source**: `BS-051`
**Depends On**: `SPEC-ACCGATE-002`, `SPEC-AGENT-PROMPT-001`
**Related**: `SPEC-SKILLSURFACE-001`, `SPEC-PARITY-001`, `SPEC-SPECWR-002`
**Target module**: `autopus-adk/`
**Module ownership**: `autopus-adk/.autopus/specs/SPEC-ADK-IDEA-CLARIFY-001/**`

## Purpose

Make `auto idea` produce more actionable BS files by adding a bounded Deep Interview-inspired clarification gate before orchestra fan-out. The gate preserves momentum: interactive mode asks the highest-impact uncertainty first, while `--auto` records assumptions and deferred questions without blocking.

The feature is complete only when `auto plan --from-idea` consumes the new `Clarification Ledger` so clarification decisions affect requirements, non-goals, risks, acceptance seeds, open questions, and reviewer focus.

## Background

`BS-051` identifies that `auto idea` already has structure and debate, but it lacks a sharp one-question clarification contract. Current source surfaces are uneven:

- `content/skills/idea.md` and `templates/*/skills/idea*` describe What/Why/Who/When, assumptions, 2-round debate, and BS storage.
- `templates/codex/skills/auto-idea.md.tmpl` already has a partial clarification gate, but it does not define the ledger schema, risk-ranked question selection, or planner handoff contract.
- `templates/claude/commands/auto-router.md.tmpl` and `templates/gemini/commands/auto-router.md.tmpl` still describe simpler Step 2 behavior for `/auto idea`.
- `internal/cli/orchestra_brainstorm.go::buildBrainstormPrompt` asks providers to reconstruct intent, but it does not tell them how to honor an upstream ledger.

The external `deep-interview` skill is useful as a pattern reference only. Its pinned provenance from `BS-051` must remain untrusted evidence unless a separate promotion/vendoring gate is added.

## Definitions

- **Clarification Gate**: the pre-orchestra `auto idea` step that inspects existing evidence, updates the ledger, and optionally asks one user question.
- **Clarification Ledger**: a structured table with fields `goal`, `scope_boundary`, `constraints`, `done_evidence`, and `brownfield_impact`.
- **Ledger row status**: one of `answered`, `assumed`, or `deferred`.
- **Confidence**: an integer score from `1` to `10` where `10` means the field is explicitly answered by the user or strong project evidence.
- **Confidence norm**: `confidence / 10`, used only inside the expected-gain formula.
- **Impact weight**: an integer score from `1` to `10` expressing how much a wrong or missing field can damage the idea-to-plan handoff.
- **Expected gain**: the selection score `impact_weight * (1 - confidence_norm)`.
- **Auto assumption mode**: `--auto` behavior that records inferred values and unasked questions without stopping for user input.
- **Plan handoff**: the mapping from ledger rows into SPEC requirements, non-goals, risks, acceptance seeds, research/open questions, and reviewer focus.

## Requirements

## Clarification Ranking Contract

The default impact weights for required ledger fields are:

| Field | Default impact weight | Reason |
| --- | ---: | --- |
| `goal` | 8 | A wrong goal can steer the whole brainstorm. |
| `scope_boundary` | 8 | Missing non-goals create scope creep and invalid plan handoff. |
| `constraints` | 5 | Constraints matter, but some can be found during planning. |
| `done_evidence` | 9 | Weak done evidence directly weakens acceptance criteria. |
| `brownfield_impact` | 6 | Wrong module impact creates implementation and reviewer churn. |

Implementations MAY override an impact weight for a specific idea only when the source evidence explains the override. Confidence values in ledger rows and tests remain integers from `1` to `10`; only the formula uses `confidence_norm = confidence / 10`.

Example: if `done_evidence` has confidence `2` and impact weight `9`, expected gain is `9 * (1 - 2/10) = 7.20`.

### Scope And Source Ownership

**REQ-ADKIDEA-SCOPE-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL implement source-of-truth changes inside `autopus-adk/` and SHALL NOT directly edit generated root `.codex/**`, `.gemini/**`, `.opencode/**`, `.claude/**`, `.agents/plugins/marketplace.json`, or plugin cache files.

**REQ-ADKIDEA-SCOPE-02**
Priority: Must
Type: Unwanted
IF external `deep-interview` skill material is referenced, THEN THE SYSTEM SHALL treat it as pinned provenance evidence and SHALL NOT execute, vendor, or present upstream text as trusted instructions.

### Clarification Gate

**REQ-ADKIDEA-GATE-01**
Priority: Must
Type: Event-driven
WHEN `auto idea` parses the idea description, THEN THE SYSTEM SHALL build a `Clarification Ledger` before `auto orchestra brainstorm` runs.

**REQ-ADKIDEA-GATE-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL maintain ledger rows for exactly these fields in this order: `goal`, `scope_boundary`, `constraints`, `done_evidence`, and `brownfield_impact`.

**REQ-ADKIDEA-GATE-03**
Priority: Must
Type: Event-driven
WHEN project docs or relevant code answer a ledger field with high confidence, THEN THE SYSTEM SHALL fill that row from evidence instead of asking the user.

**REQ-ADKIDEA-GATE-04**
Priority: Must
Type: Event-driven
WHEN interactive mode still has unresolved high-impact uncertainty, THEN THE SYSTEM SHALL ask only the ledger row with the highest expected gain using the Clarification Ranking Contract and SHALL format the question as `Current understanding`, `Blocked decision`, `Recommended answer`, and `Question`.

**REQ-ADKIDEA-GATE-05**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL limit default interactive clarification to one question, SHALL allow at most one extra critical-ambiguity question, and SHALL use optional `--deep-clarify` to permit at most three questions.

**REQ-ADKIDEA-GATE-06**
Priority: Must
Type: Unwanted
IF `--auto` is set, THEN THE SYSTEM SHALL ask zero questions, SHALL continue to orchestra, and SHALL record inferred or missing ledger rows as `assumed` or `deferred`.

**REQ-ADKIDEA-GATE-07**
Priority: Must
Type: Unwanted
IF a ledger row is inferred rather than stated by the user or supported by project evidence, THEN THE SYSTEM SHALL set confidence to `6` or lower and SHALL record an `If Wrong` consequence.

### BS Output And Orchestra Handoff

**REQ-ADKIDEA-BS-01**
Priority: Must
Type: Event-driven
WHEN the BS file is saved, THEN THE SYSTEM SHALL include a `## Clarification Ledger` table with columns `Field`, `Status`, `Source`, `Confidence`, `Decision / Assumption`, `If Wrong`, and `Plan Handoff`.

**REQ-ADKIDEA-BS-02**
Priority: Must
Type: Event-driven
WHEN `auto orchestra brainstorm` receives structured idea context, THEN THE SYSTEM SHALL include the ledger and SHALL direct providers not to re-ask `answered` fields.

**REQ-ADKIDEA-BS-03**
Priority: Must
Type: Event-driven
WHEN providers receive the brainstorm prompt, THEN THE SYSTEM SHALL make `assumed` and `deferred` ledger rows part of debate focus and assumption risk analysis.

### Plan Handoff

**REQ-ADKIDEA-PLAN-01**
Priority: Must
Type: Event-driven
WHEN `auto plan --from-idea` loads a BS file containing `## Clarification Ledger`, THEN THE SYSTEM SHALL map `answered` rows into requirement and explicit scope seeds.

**REQ-ADKIDEA-PLAN-02**
Priority: Must
Type: Event-driven
WHEN `auto plan --from-idea` loads `assumed` rows, THEN THE SYSTEM SHALL map them into risks, acceptance assumptions, validation experiments, or reviewer focus.

**REQ-ADKIDEA-PLAN-03**
Priority: Must
Type: Event-driven
WHEN `auto plan --from-idea` loads `deferred` rows, THEN THE SYSTEM SHALL map them into research/open questions and SHALL NOT silently promote them into requirements.

**REQ-ADKIDEA-PLAN-04**
Priority: Must
Type: Event-driven
WHEN a `scope_boundary` row exists, THEN THE SYSTEM SHALL map it into explicit SPEC non-goals.

**REQ-ADKIDEA-PLAN-05**
Priority: Must
Type: Unwanted
IF a BS file does not contain `## Clarification Ledger`, THEN THE SYSTEM SHALL preserve existing `auto plan --from-idea` behavior and SHALL NOT fail solely because the ledger is absent.

### Surface Parity And Verification

**REQ-ADKIDEA-SURF-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL update ADK source guidance and templates for Claude, Codex, Gemini, and OpenCode-compatible generated surfaces so the clarification contract does not drift by platform.

**REQ-ADKIDEA-TEST-01**
Priority: Must
Type: Event-driven
WHEN source/template tests run, THEN THE SYSTEM SHALL prove the rendered `auto idea` surfaces include the ledger fields, one-question budget, `--auto` assumption behavior, external provenance boundary, and plan handoff mapping.

## Related SPECs

- `SPEC-ACCGATE-002`: semantic invariant and oracle acceptance discipline that this SPEC follows.
- `SPEC-AGENT-PROMPT-001`: prompt layer discipline and generated-surface source ownership.
- `SPEC-SKILLSURFACE-001` and `SPEC-PARITY-001`: source-to-generated skill surface parity.
- `SPEC-SPECWR-002`: SPEC writer quality expectations for consuming brainstorm context.

No sibling SPEC is required because the complete outcome is cohesive inside `autopus-adk/`: clarification contract, BS schema, orchestra prompt handoff, and planner consumption.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
| --- | --- | --- | --- |
| `REQ-ADKIDEA-SCOPE-01` | `T1`, `T7` | `AC-ADKIDEA-006` | `INV-ADKIDEA-006` |
| `REQ-ADKIDEA-SCOPE-02` | `T1`, `T6` | `AC-ADKIDEA-005` | `INV-ADKIDEA-005` |
| `REQ-ADKIDEA-GATE-01` | `T1`, `T2` | `AC-ADKIDEA-001` | `INV-ADKIDEA-001` |
| `REQ-ADKIDEA-GATE-02` | `T1`, `T2` | `AC-ADKIDEA-001`, `AC-ADKIDEA-002` | `INV-ADKIDEA-001` |
| `REQ-ADKIDEA-GATE-03` | `T2`, `T6` | `AC-ADKIDEA-001` | `INV-ADKIDEA-002` |
| `REQ-ADKIDEA-GATE-04` | `T2`, `T6` | `AC-ADKIDEA-002` | `INV-ADKIDEA-002` |
| `REQ-ADKIDEA-GATE-05` | `T1`, `T2`, `T6` | `AC-ADKIDEA-002` | `INV-ADKIDEA-002` |
| `REQ-ADKIDEA-GATE-06` | `T1`, `T2`, `T6` | `AC-ADKIDEA-003` | `INV-ADKIDEA-003` |
| `REQ-ADKIDEA-GATE-07` | `T2`, `T6` | `AC-ADKIDEA-003` | `INV-ADKIDEA-003` |
| `REQ-ADKIDEA-BS-01` | `T3`, `T6` | `AC-ADKIDEA-001`, `AC-ADKIDEA-003` | `INV-ADKIDEA-001`, `INV-ADKIDEA-003` |
| `REQ-ADKIDEA-BS-02` | `T4`, `T6` | `AC-ADKIDEA-004` | `INV-ADKIDEA-004` |
| `REQ-ADKIDEA-BS-03` | `T4`, `T6` | `AC-ADKIDEA-004` | `INV-ADKIDEA-004` |
| `REQ-ADKIDEA-PLAN-01` | `T5`, `T6` | `AC-ADKIDEA-007` | `INV-ADKIDEA-007` |
| `REQ-ADKIDEA-PLAN-02` | `T5`, `T6` | `AC-ADKIDEA-007` | `INV-ADKIDEA-007` |
| `REQ-ADKIDEA-PLAN-03` | `T5`, `T6` | `AC-ADKIDEA-007` | `INV-ADKIDEA-007` |
| `REQ-ADKIDEA-PLAN-04` | `T5`, `T6` | `AC-ADKIDEA-007` | `INV-ADKIDEA-007` |
| `REQ-ADKIDEA-PLAN-05` | `T5`, `T6` | `AC-ADKIDEA-008` | `INV-ADKIDEA-008` |
| `REQ-ADKIDEA-SURF-01` | `T1`, `T7` | `AC-ADKIDEA-006` | `INV-ADKIDEA-006` |
| `REQ-ADKIDEA-TEST-01` | `T6`, `T7` | `AC-ADKIDEA-006` | `INV-ADKIDEA-006` |

## Out of Scope

- Installing, vendoring, or executing `devbrother2024/skills`.
- Changing provider configuration, debate strategy, or orchestra transport semantics.
- Directly editing generated workspace surfaces outside `autopus-adk/`.
- Adding UI flows outside the existing CLI/agent workflow surfaces.
