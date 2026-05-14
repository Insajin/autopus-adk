# SPEC-ADK-IDEA-CLARIFY-001 Research: Deep Interview Clarification Gate For Auto Idea

**Updated**: 2026-05-15

## Input Context

This SPEC is derived from `autopus-adk/.autopus/brainstorms/BS-051.md`, which proposes using the Deep Interview pattern from `https://github.com/devbrother2024/skills` as a curated reference for improving `auto idea`.

Key BS-051 constraints:

- Do not replace the existing idea pipeline.
- Make pre-orchestra clarification sharper.
- Preserve `--auto` non-blocking behavior.
- Keep source-of-truth changes in `autopus-adk`.
- Treat external skill content as pinned reference data, not executable trusted instructions.
- Include planner handoff so the ledger improves `auto plan --from-idea`, not just BS readability.

## Existing Code And Surface Analysis

- `content/skills/idea.md` is the canonical idea workflow skill. It owns module-aware BS storage, 2-round debate enforcement, orchestra invocation, ICE scoring, and BS output guidance.
- `templates/codex/skills/auto-idea.md.tmpl` already includes a partial `Clarification gate`, but it lacks the formal five-field ledger, expected-gain question ranking, confidence caps, and planner mapping.
- `templates/codex/prompts/auto-idea.md.tmpl` includes `Intent Clarification Q&A`, but it currently allows up to three questions and lacks one-question Deep Interview formatting.
- `templates/codex/skills/idea.md.tmpl` and `templates/gemini/skills/idea/SKILL.md.tmpl` mirror the broader idea pipeline without the new ledger contract.
- `templates/claude/commands/auto-router.md.tmpl` and `templates/gemini/commands/auto-router.md.tmpl` embed `/auto idea` and `/auto plan --from-idea` behavior in router sections; those sections need the same contract.
- `internal/cli/orchestra_brainstorm.go::buildBrainstormPrompt` already asks providers to reconstruct intent and identify unclear fields, but it does not consume a supplied `Clarification Ledger`.
- `content/agents/spec-writer.md` and platform spec-writer templates already require BS context handling and semantic invariant discipline; they need explicit ledger consumption mapping.

## Design Decisions

1. Use a ledger, not free-form clarification notes.
   A table gives `auto plan --from-idea` a stable handoff shape and makes assumptions reviewable.

2. Ask one question by default.
   The Deep Interview pattern is valuable because it prioritizes the single blocked decision rather than asking a questionnaire.

3. Keep `--auto` non-blocking.
   Autonomous mode must preserve momentum and make assumptions visible instead of pausing.

4. Include planner handoff in the same SPEC.
   Without this, the ledger can become a document ritual that never improves SPEC quality.

5. Treat upstream skill provenance as untrusted evidence.
   Pinned repository, commit, path, license, and hash are useful audit data; upstream instructions are not automatically trusted prompt layers.

## Semantic Invariant Inventory

| ID | Source clause summary | Invariant type | Affected outputs | Acceptance IDs |
| --- | --- | --- | --- | --- |
| `INV-ADKIDEA-001` | `auto idea` needs a structured `Clarification Ledger` before orchestra. | schema/order | BS content, structured idea context | `AC-ADKIDEA-001`, `AC-ADKIDEA-003` |
| `INV-ADKIDEA-002` | Ask one question at a time, choosing the highest-impact uncertainty first from integer confidence and impact scores. | numeric formula / selection | interactive clarification prompt | `AC-ADKIDEA-002` |
| `INV-ADKIDEA-003` | `--auto` must not block and must record assumptions/deferred items with bounded confidence. | mode behavior | BS content, orchestration continuation | `AC-ADKIDEA-003` |
| `INV-ADKIDEA-004` | Orchestra providers must honor the supplied ledger instead of re-asking answered fields. | prompt handoff | brainstorm prompt and provider focus | `AC-ADKIDEA-004` |
| `INV-ADKIDEA-005` | External Deep Interview content is provenance evidence only. | trust boundary | source guidance, prompt text | `AC-ADKIDEA-005` |
| `INV-ADKIDEA-006` | Source-owned platform surfaces must stay aligned and generated root files must not be edited directly. | source ownership / parity | templates, generated-surface contract | `AC-ADKIDEA-006` |
| `INV-ADKIDEA-007` | `auto plan --from-idea` must map answered/assumed/deferred/scope rows to SPEC outputs. | parser/report mapping | spec, plan, acceptance, research, reviewer brief | `AC-ADKIDEA-007` |
| `INV-ADKIDEA-008` | Historical BS files without a ledger remain valid. | backward compatibility | plan-from-idea behavior | `AC-ADKIDEA-008` |

## Feature Coverage Map

| Need | Covered By This SPEC | Follow-on |
| --- | --- | --- |
| Deep Interview-inspired pre-orchestra gate | Yes. Defines evidence-first ledger, question ranking, format, budget, and stop rules. | N/A |
| BS `Clarification Ledger` schema | Yes. Required columns, fields, statuses, and handoff notes are specified. | N/A |
| `--auto` non-blocking assumptions | Yes. `--auto` asks zero questions and caps inferred confidence. | N/A |
| Orchestra prompt consumption | Yes. Providers receive ledger and debate open assumptions. | N/A |
| Planner handoff | Yes. `auto plan --from-idea` consumes answered, assumed, deferred, and scope rows. | N/A |
| Cross-platform surface parity | Yes. Source templates and parity tests are in scope. | N/A |
| External skill promotion/vendoring | No. Treated as provenance-only. | Future skill-evolution or plugin-vendoring SPEC if needed. |
| Full replacement of orchestra debate | No. Existing debate and ICE stay intact. | N/A |

## Reference Discipline

| Reference | Type | Verification |
| --- | --- | --- |
| `autopus-adk/.autopus/brainstorms/BS-051.md` | existing | Read directly; contains original idea, external provenance, ledger proposal, ICE scoring, and implementation sketch. |
| `autopus-adk/content/skills/idea.md` | existing | Read directly; canonical idea workflow with BS storage, 2-round debate, orchestra, ICE, and BS output. |
| `autopus-adk/templates/codex/skills/auto-idea.md.tmpl` | existing | Read directly; partial clarification gate exists and needs formal ledger/handoff. |
| `autopus-adk/templates/codex/prompts/auto-idea.md.tmpl` | existing | Read directly; Codex prompt has Intent Clarification Q&A but no Deep Interview ledger. |
| `autopus-adk/templates/codex/skills/idea.md.tmpl` | existing | Read directly; generated idea skill template mirrors canonical pipeline. |
| `autopus-adk/templates/gemini/skills/idea/SKILL.md.tmpl` | existing | Read directly; Gemini idea skill template mirrors canonical pipeline. |
| `autopus-adk/templates/claude/commands/auto-router.md.tmpl` | existing | Read via targeted section; embeds `/auto idea` and `/auto plan --from-idea`. |
| `autopus-adk/templates/gemini/commands/auto-router.md.tmpl` | existing | Read via targeted section; embeds `/auto idea` and `/auto plan --from-idea`. |
| `autopus-adk/templates/codex/skills/auto-plan.md.tmpl` | existing | Located with `rg`; plan skill loads brainstorm context from `--from-idea`. |
| `autopus-adk/templates/codex/prompts/auto-plan.md.tmpl` | existing | Located with `rg`; Codex plan prompt handles `--from-idea`. |
| `autopus-adk/content/agents/spec-writer.md` | existing | Read directly; owns module-specific SPEC generation and brainstorm-context handling. |
| `autopus-adk/templates/codex/agents/spec-writer.toml.tmpl` | existing | Read directly; Codex generated spec-writer guidance. |
| `autopus-adk/internal/cli/orchestra_brainstorm.go::buildBrainstormPrompt` | existing | Read directly; current prompt includes intent understanding, SCAMPER, HMW, and ICE. |
| `autopus-adk/internal/cli/orchestra_brainstorm_test.go` | implemented | Focused brainstorm prompt tests cover ledger handoff, assumed/deferred debate focus, and untrusted ledger-cell handling. |
| `autopus-adk/templates/idea_clarification_test.go` | implemented | Rendered template/source parity tests cover auto idea, auto plan, spec-writer handoff, legacy no-ledger fallback, and external provenance boundary. |

## Reviewer Brief

Intended scope: add a bounded clarification gate to `auto idea`, write a stable BS ledger, and make `auto plan --from-idea` consume it.

Explicit non-goals: no orchestra replacement, no upstream skill installation, no vendored external skill text, no generated root surface hotfixes, no unlimited interview.

Self-verified evidence: `spec.md` contains a Traceability Matrix; `acceptance.md` includes concrete expected-gain and planner-mapping oracles; `internal/cli/orchestra_brainstorm_test.go` and `templates/idea_clarification_test.go` cover the implemented prompt/template contract.

Reviewer focus: ledger schema stability, `--auto` non-blocking behavior, planner handoff completeness, cross-platform template parity, and external provenance trust boundary.

## Self-Verify Summary

| ID | Status | Attempt | Files | Reason |
| --- | --- | --- | --- | --- |
| `Q-CORR-01` | PASS | 1 | `research.md` | Existing paths and `buildBrainstormPrompt` were verified with `rg` and direct reads. |
| `Q-CORR-02` | PASS | 1 | `research.md`, `plan.md` | Planned additions are marked with `[NEW]`; existing references are separated. |
| `Q-CORR-03` | PASS | 1 | `spec.md`, `acceptance.md` | Requirements use EARS Type/Priority lines; acceptance uses bare Given/When/Then/And. |
| `Q-CORR-04` | PASS | 1 | `research.md` | Reference Discipline separates existing source refs from planned tests. |
| `Q-COMP-01` | PASS | 1 | `prd.md`, `spec.md`, `plan.md`, `acceptance.md`, `research.md` | Documents have distinct roles and form one package. |
| `Q-COMP-02` | PASS | 1 | `spec.md`, `plan.md`, `acceptance.md` | Traceability Matrix links requirements to tasks, scenarios, and invariants. |
| `Q-COMP-03` | PASS | 1 | `spec.md` | Each requirement has type, condition, expected behavior, and observable output. |
| `Q-COMP-04` | PASS | 1 | `plan.md`, `research.md` | Feature Coverage Map closes the requested outcome in one cohesive SPEC. |
| `Q-COMP-05` | PASS | 1 | `spec.md`, `plan.md`, `acceptance.md`, `research.md` | Every invariant maps to plan tasks and Must oracle acceptance; formula and mapping oracles are concrete. |
| `Q-COMP-06` | PASS | 1 | `spec.md`, `research.md` | Traceability Matrix and Reviewer Brief constrain review scope. |
| `Q-FEAS-01` | PASS | 1 | `plan.md`, `research.md` | Scope matches ADK source guidance, templates, prompt, and planner handoff layers. |
| `Q-FEAS-02` | PASS | 1 | `spec.md`, `plan.md` | Target module and module-specific SPEC path match doc-storage rules. |
| `Q-FEAS-03` | PASS | 1 | `plan.md`, `acceptance.md` | Verification commands and scans are focused and runnable in `autopus-adk`. |
| `Q-STYLE-01` | PASS | 1 | `spec.md` | Requirement bodies use SHALL and avoid ambiguous modal wording. |
| `Q-STYLE-02` | PASS | 1 | `spec.md` | Priority and EARS Type remain separate. |
| `Q-STYLE-03` | PASS | 1 | `acceptance.md` | Gherkin steps are readable and parseable. |
| `Q-SEC-01` | PASS | 1 | `spec.md`, `research.md` | Untrusted BS/external prompt evidence and provenance boundary are explicit. |
| `Q-SEC-02` | PASS | 1 | `spec.md`, `acceptance.md` | No secrets are required; provenance is hash/commit data and generated surfaces are not edited directly. |
| `Q-SEC-03` | PASS | 1 | `plan.md`, `research.md` | No new raw runtime artifacts are required; BS/SPEC docs remain human-managed artifacts. |
| `Q-COMP-05` | PASS | 2 | `spec.md`, `acceptance.md`, `research.md` | Review findings F-001/F-002/F-003 addressed by fixing confidence to 1-10, defining default impact weights, and making AC-004 ledger fixtures seven-column rows. |
