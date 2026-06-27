# SPEC-ADK-MINIMALITY-DISCIPLINE-001: Default Minimality Discipline For Autopus Workflows

**Status**: completed
**Created**: 2026-06-27
**Domain**: ADK-MINIMALITY-DISCIPLINE
**Priority**: HIGH
**Source**: direct `@auto plan` request
**Depends On**: `SPEC-ADK-IDEA-CLARIFY-001`, `SPEC-SPECWR-002`, `SPEC-ACCGATE-002`
**Related**: `SPEC-SPECWR-001`, `SPEC-ACCGATE-001`
**Target module**: `autopus-adk/`
**Module ownership**: `autopus-adk/.autopus/specs/SPEC-ADK-MINIMALITY-DISCIPLINE-001/**`

## Purpose

Make "implement only what is needed" the default Autopus work discipline across existing `@auto plan`, `@auto go`, `@auto fix`, and `@auto review` flows. Users continue using the same commands. They do not manage a lean/Ponytail mode. Autopus instead records the important implementation choices as a concise decision receipt.

This discipline is inspired by Ponytail's minimality posture, but it is reinterpreted as ADK-owned workflow guidance. Ponytail code, hooks, prompts, or package contents are not vendored or installed.

## Background

Current ADK surfaces already enforce strong gates:

- `templates/codex/skills/auto-plan.md.tmpl` requires `Plan Intent Ledger`, `Semantic Invariant Inventory`, `Reference Discipline`, and reviewer brief sections.
- `templates/codex/skills/auto-go.md.tmpl`, `templates/codex/prompts/auto-go.md.tmpl`, and `content/skills/agent-pipeline.md` control implementation routing, workflow authenticity, review loops, and sync readiness.
- `templates/codex/skills/auto-fix.md.tmpl`, `templates/codex/prompts/auto-fix.md.tmpl`, `content/agents/debugger.md`, and `content/skills/debugging.md` require reproduction-first bug fixing but do not yet require caller/shared root-cause inspection.
- `templates/codex/skills/auto-review.md.tmpl`, `templates/codex/prompts/auto-review.md.tmpl`, `content/agents/reviewer.md`, and `content/skills/review.md` use TRUST 5 but do not clearly separate correctness/security findings from complexity-only findings.
- `templates/gemini/skills/auto-plan/SKILL.md.tmpl`, `templates/gemini/skills/auto-go/SKILL.md.tmpl`, `templates/gemini/skills/auto-fix/SKILL.md.tmpl`, `templates/gemini/skills/auto-review/SKILL.md.tmpl`, `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl`, `templates/claude/commands/auto-router.md.tmpl`, and `templates/gemini/commands/auto-router.md.tmpl` are routed/generated surfaces that must carry equivalent contracts.
- `pkg/qualityloop` and `pkg/skillevolve` already have a safe candidate lifecycle with generated-surface blocking, metadata-only retention, replay, approval, and promotion gates.

The gap is a stable default decision ladder that prevents unnecessary new code, dependencies, and abstractions while preserving evidence, security, accessibility, and data-safety gates.

## Outcome Boundary

- Outcome Lock: existing `@auto plan`, `@auto go`, `@auto fix`, and `@auto review` workflows apply the minimality ladder by default and expose only concise decision receipts to users.
- Mandatory requirements: plan matrix, go/pipeline ladder injection, fix caller/shared root-cause check, review finding separation, non-reducible safety gates, source-surface parity, and inactive qualityloop/skillevolve candidates.
- Explicit non-goals: user-managed lean/Ponytail mode, Ponytail vendoring, line-count metric, correctness/security/accessibility replacement, generated root surface edits, and automatic simplification apply.
- Completion evidence: focused template/content/qualityloop/skillevolve tests, Codex/OpenCode/Gemini adapter-rendered surface tests, and `auto spec validate` for this SPEC package.

## Definitions

- **Minimality discipline**: internal ADK name for the default work discipline that checks necessity and reuse before adding new code, dependency, or abstraction.
- **User-facing phrasing**: "필요한 만큼만 구현" or "important decisions"; final responses do not expose a mode name.
- **Minimality ladder**: the ordered decision check: actual need, existing code/helper/pattern, stdlib/native feature, existing dependency, justified new dependency or abstraction, and minimum sufficient verification.
- **Minimality Decision Matrix**: `research.md` table that records each ladder step, evidence, decision, and receipt item.
- **Minimality receipt**: short final-response bullets explaining important choices, for example reused primitive, skipped dependency, added focused regression test.
- **Complexity finding**: advisory or blocking review issue about avoidable code, dependencies, helpers, abstractions, or scope expansion.
- **Correctness/security finding**: review issue about behavior, data safety, security, validation, accessibility, build, test, or contract correctness.
- **Non-reducible gate**: security, validation, accessibility, data-loss handling, generated-surface hygiene, and deterministic verification requirements that cannot be weakened in the name of minimality.
- **Repeated minimality failure**: at least three qualityloop failure inputs whose `FailureFingerprint` is the same stable key, formatted as `minimality.<reason_code>.<stable_target_or_fingerprint>`.

## Requirements

### Scope And Source Ownership

**REQ-MINDISC-SCOPE-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL implement source-of-truth changes inside `autopus-adk/` and SHALL NOT directly edit root generated surfaces, plugin cache files, or runtime artifacts.

**REQ-MINDISC-SCOPE-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL express this capability as Autopus's default work discipline and SHALL NOT require users to enable, disable, or remember a lean/Ponytail mode.

**REQ-MINDISC-SCOPE-03**
Priority: Must
Type: Unwanted
IF Ponytail upstream material is referenced, THEN THE SYSTEM SHALL treat it as user-provided provenance evidence only and SHALL NOT vendor, install, execute, or copy upstream text without preserving required MIT license notices.

### Minimality Ladder

**REQ-MINDISC-LADDER-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL define a stable minimality ladder with this order: actual need, existing code/helper/pattern, stdlib or native platform feature, existing dependency, justified new dependency or abstraction, and minimum sufficient verification.

**REQ-MINDISC-LADDER-02**
Priority: Must
Type: Event-driven
WHEN a workflow considers a new dependency or new abstraction, THEN THE SYSTEM SHALL require evidence that existing code, stdlib/native features, and existing dependencies were checked first.

**REQ-MINDISC-LADDER-03**
Priority: Must
Type: Unwanted
IF a new dependency or new abstraction lacks justification, THEN THE SYSTEM SHALL mark the plan as revise-target or risk instead of treating the expansion as accepted scope.

**REQ-MINDISC-LADDER-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL NOT treat code line count reduction as the goal and SHALL NOT remove or weaken security, validation, accessibility, data-loss handling, deterministic oracle, or generated-surface hygiene gates.

### Auto Plan

**REQ-MINDISC-PLAN-01**
Priority: Must
Type: Event-driven
WHEN `@auto plan` creates or revises a SPEC, THEN THE SYSTEM SHALL write `## Minimality Decision Matrix` in `research.md`.

**REQ-MINDISC-PLAN-02**
Priority: Must
Type: Event-driven
WHEN `@auto plan` records user-requested extension design, a new abstraction, or a new dependency, THEN THE SYSTEM SHALL preserve the user intent and record the justification, considered alternatives, and verification obligation.

**REQ-MINDISC-PLAN-03**
Priority: Must
Type: Event-driven
WHEN `@auto plan` produces acceptance criteria, THEN THE SYSTEM SHALL identify the minimum sufficient verification set that closes the Outcome Lock without making security, accessibility, validation, or data-loss checks optional.

### Auto Go And Agent Pipeline

**REQ-MINDISC-GO-01**
Priority: Must
Type: Event-driven
WHEN `@auto go` or the agent pipeline prepares planner, executor, tester, reviewer, or fixer prompts, THEN THE SYSTEM SHALL include the minimality ladder as stable instruction.

**REQ-MINDISC-GO-02**
Priority: Must
Type: Event-driven
WHEN executor tasks are assigned, THEN THE SYSTEM SHALL ask workers to inspect existing code paths, helpers, and patterns before adding new helpers or abstractions.

**REQ-MINDISC-GO-03**
Priority: Must
Type: Event-driven
WHEN implementation completes, THEN THE SYSTEM SHALL include a concise minimality receipt in the terminal handoff or final response using user-facing decision phrasing rather than a mode name.

### Decision Receipts

**REQ-MINDISC-RECEIPT-01**
Priority: Must
Type: Event-driven
WHEN `@auto plan`, `@auto go`, `@auto fix`, or `@auto review` renders a final response or terminal handoff, THEN THE SYSTEM SHALL show a concise decision receipt for important minimality choices and SHALL NOT expose lean/Ponytail mode state.

### Auto Fix

**REQ-MINDISC-FIX-01**
Priority: Must
Type: Event-driven
WHEN `@auto fix` scopes a bug, THEN THE SYSTEM SHALL identify the failing symptom, the owning function or path, its callers, and any shared root-cause path before accepting the patch plan.

**REQ-MINDISC-FIX-02**
Priority: Must
Type: Unwanted
IF a proposed fix only patches the symptom location without checking caller/shared root-cause paths, THEN THE SYSTEM SHALL mark the plan as revise-target unless evidence shows the symptom location is the root cause.

### Auto Review

**REQ-MINDISC-REVIEW-01**
Priority: Must
Type: Event-driven
WHEN `@auto review` reports findings, THEN THE SYSTEM SHALL separate correctness/security findings from complexity findings.

**REQ-MINDISC-REVIEW-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL use complexity tags from this candidate set when applicable: `delete`, `stdlib`, `native`, `yagni`, `shrink`, `existing-helper`, and `existing-dependency`.

**REQ-MINDISC-REVIEW-03**
Priority: Must
Type: Unwanted
IF a complexity finding conflicts with a correctness, security, accessibility, validation, or data-safety requirement, THEN THE SYSTEM SHALL keep the safety requirement authoritative and downgrade or reject the complexity finding.

### Qualityloop And Skillevolve

**REQ-MINDISC-QLOOP-01**
Priority: Must
Type: Event-driven
WHEN qualityloop observes at least three unnecessary dependency, duplicate helper, single-implementation abstraction, native/stdlib-available, YAGNI, existing-helper, existing-dependency, or shrink-scope findings with the same `FailureFingerprint`, THEN THE SYSTEM SHALL normalize them into improvement candidates with metadata-only evidence.

**REQ-MINDISC-QLOOP-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep minimality improvement candidates inactive until existing quarantine, deterministic replay, human approval, and promotion gates allow application.

**REQ-MINDISC-QLOOP-03**
Priority: Must
Type: Unwanted
IF a minimality improvement candidate targets generated surfaces, root runtime artifacts, or plugin cache paths, THEN THE SYSTEM SHALL reject or quarantine it using existing safety policy.

**REQ-MINDISC-QLOOP-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL preserve the distinction between qualityloop repeated-failure aggregation requiring at least three ADK-owned inputs with the same `FailureFingerprint` and skillevolve quality-index candidate generation using its existing default `MinCount = 2` quarantine behavior.

### Surface Parity And Verification

**REQ-MINDISC-SURF-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL keep ADK source guidance and generated-source templates aligned for Codex skill/prompt surfaces, Gemini skill surfaces, Claude routed workflow surfaces, and shared agent-pipeline/reviewer surfaces.

**REQ-MINDISC-SURF-02**
Priority: Must
Type: Event-driven
WHEN the Codex adapter renders workflow skills, workflow prompts, and extended skills, THEN THE SYSTEM SHALL prove each rendered `.codex/**` and `.agents/**` output contains the source-owned subset of the relevant minimality ladder, decision matrix, receipt, root-cause, and review-section contracts defined by its source template or hardcoded rewrite body.

**REQ-MINDISC-SURF-03**
Priority: Must
Type: Event-driven
WHEN stable prompt guidance changes for this discipline, THEN THE SYSTEM SHALL classify the change across stable, snapshot, and ephemeral prompt layers and SHALL record cache invalidation observation points for source templates, adapter-rendered outputs, and generated workspace surfaces.

**REQ-MINDISC-SURF-04**
Priority: Must
Type: Event-driven
WHEN the OpenCode adapter renders workflow commands or shared workflow skills, THEN THE SYSTEM SHALL prove thin `.opencode/commands/auto-*.md` aliases route to the detailed shared workflow skills without duplicating contract text and SHALL prove `.agents/skills/auto-plan/SKILL.md`, `.agents/skills/auto-go/SKILL.md`, `.agents/skills/auto-fix/SKILL.md`, and `.agents/skills/auto-review/SKILL.md` carry their source-owned subset of the relevant minimality contracts: plan matrix, go ladder and receipt, fix caller/shared root-cause rule, and review section split.

**REQ-MINDISC-TEST-01**
Priority: Must
Type: Event-driven
WHEN source and template tests run, THEN THE SYSTEM SHALL prove the minimality ladder, decision matrix, root-cause caller check, separated review sections, receipt language, and candidate safety rules are present in source-owned surfaces.

## Related SPECs

- `SPEC-ADK-IDEA-CLARIFY-001`: direct intent ledger and plan handoff patterns reused by this SPEC.
- `SPEC-SPECWR-002`: SPEC writer quality expectations.
- `SPEC-ACCGATE-002`: semantic invariant and oracle acceptance discipline.
- `SPEC-SPECWR-001` and `SPEC-ACCGATE-001`: earlier SPEC quality and acceptance-gate foundations.

Prompt-layer and skill-evolution source anchors exist in ADK code and comments, but no current `.autopus/specs/` package named `SPEC-AGENT-PROMPT-001` or `SPEC-SKILL-EVOLVE-001` exists in this repository. They are therefore treated as code-level references in `research.md`, not SPEC dependencies.

No sibling SPEC is required. The requested outcome is one cohesive ADK prompt/workflow discipline that spans plan/go/fix/review and routes repeated signals through existing quality improvement machinery.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
| --- | --- | --- | --- |
| `REQ-MINDISC-SCOPE-01` | `T1`, `T8` | `AC-MINDISC-007` | `INV-MINDISC-007` |
| `REQ-MINDISC-SCOPE-02` | `T1`, `T7` | `AC-MINDISC-001`, `AC-MINDISC-006` | `INV-MINDISC-001`, `INV-MINDISC-006` |
| `REQ-MINDISC-SCOPE-03` | `T1`, `T8` | `AC-MINDISC-008` | `INV-MINDISC-008` |
| `REQ-MINDISC-LADDER-01` | `T1`, `T2`, `T3` | `AC-MINDISC-001`, `AC-MINDISC-003` | `INV-MINDISC-001` |
| `REQ-MINDISC-LADDER-02` | `T2`, `T3`, `T8` | `AC-MINDISC-002`, `AC-MINDISC-003` | `INV-MINDISC-002` |
| `REQ-MINDISC-LADDER-03` | `T2`, `T8` | `AC-MINDISC-002` | `INV-MINDISC-002` |
| `REQ-MINDISC-LADDER-04` | `T1`, `T3`, `T5`, `T8` | `AC-MINDISC-005` | `INV-MINDISC-005` |
| `REQ-MINDISC-PLAN-01` | `T2`, `T8` | `AC-MINDISC-001` | `INV-MINDISC-001` |
| `REQ-MINDISC-PLAN-02` | `T2`, `T8` | `AC-MINDISC-002` | `INV-MINDISC-002` |
| `REQ-MINDISC-PLAN-03` | `T2`, `T8` | `AC-MINDISC-010` | `INV-MINDISC-010` |
| `REQ-MINDISC-GO-01` | `T3`, `T8` | `AC-MINDISC-003` | `INV-MINDISC-003` |
| `REQ-MINDISC-GO-02` | `T3`, `T8` | `AC-MINDISC-003` | `INV-MINDISC-003` |
| `REQ-MINDISC-GO-03` | `T3`, `T7`, `T8` | `AC-MINDISC-006` | `INV-MINDISC-006` |
| `REQ-MINDISC-RECEIPT-01` | `T2`, `T3`, `T4`, `T5`, `T7`, `T8` | `AC-MINDISC-006`, `AC-MINDISC-014` | `INV-MINDISC-006` |
| `REQ-MINDISC-FIX-01` | `T4`, `T8` | `AC-MINDISC-004` | `INV-MINDISC-004` |
| `REQ-MINDISC-FIX-02` | `T4`, `T8` | `AC-MINDISC-004` | `INV-MINDISC-004` |
| `REQ-MINDISC-REVIEW-01` | `T5`, `T8` | `AC-MINDISC-005`, `AC-MINDISC-011` | `INV-MINDISC-005` |
| `REQ-MINDISC-REVIEW-02` | `T5`, `T8` | `AC-MINDISC-005`, `AC-MINDISC-011` | `INV-MINDISC-005` |
| `REQ-MINDISC-REVIEW-03` | `T5`, `T8` | `AC-MINDISC-005`, `AC-MINDISC-011` | `INV-MINDISC-005` |
| `REQ-MINDISC-QLOOP-01` | `T6`, `T8` | `AC-MINDISC-009` | `INV-MINDISC-009` |
| `REQ-MINDISC-QLOOP-02` | `T6`, `T8` | `AC-MINDISC-009`, `AC-MINDISC-012` | `INV-MINDISC-009` |
| `REQ-MINDISC-QLOOP-03` | `T6`, `T8` | `AC-MINDISC-009`, `AC-MINDISC-012` | `INV-MINDISC-009` |
| `REQ-MINDISC-QLOOP-04` | `T6`, `T8` | `AC-MINDISC-009`, `AC-MINDISC-012` | `INV-MINDISC-009` |
| `REQ-MINDISC-SURF-01` | `T1`, `T7`, `T8` | `AC-MINDISC-007`, `AC-MINDISC-011` | `INV-MINDISC-007` |
| `REQ-MINDISC-SURF-02` | `T3`, `T8` | `AC-MINDISC-007` | `INV-MINDISC-007` |
| `REQ-MINDISC-SURF-03` | `T1`, `T8` | `AC-MINDISC-013` | `INV-MINDISC-011` |
| `REQ-MINDISC-SURF-04` | `T7`, `T8` | `AC-MINDISC-007` | `INV-MINDISC-007` |
| `REQ-MINDISC-TEST-01` | `T8` | `AC-MINDISC-001` through `AC-MINDISC-014` | `INV-MINDISC-001` through `INV-MINDISC-011` |

## Out of Scope

- User-managed lean/Ponytail mode state.
- Ponytail hook vendoring, package installation, or copied upstream prompt text.
- Replacing correctness/security/accessibility review with complexity review.
- Measuring success by source line count.
- Direct root generated-surface edits.
- Automatic application of qualityloop or skillevolve simplification candidates without replay and approval.
