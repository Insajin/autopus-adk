# SPEC-ADK-IDEA-CLARIFY-001 Plan: Deep Interview Clarification Gate For Auto Idea

**Status**: completed
**Created**: 2026-05-14
**Synced**: 2026-05-15
**Target module**: `autopus-adk/`

## Implementation Strategy

Implement the feature as a source-owned workflow contract plus focused parser/prompt support. The first slice updates canonical skill/router/template sources, makes `auto orchestra brainstorm` honor an upstream ledger, and teaches planning/spec-writer guidance to consume the ledger. Generated root surfaces are refreshed only through ADK generation/update flows, not hand-edited.

## Feature Completion Scope

This single SPEC closes the requested outcome:

- pre-orchestra clarification behavior for `auto idea`
- BS `Clarification Ledger` schema
- `--auto` assumption behavior
- orchestra brainstorm handoff
- `auto plan --from-idea` consumption
- cross-platform source guidance parity
- tests that prevent the contract from becoming only prose

No sibling SPEC is needed unless implementation discovers that planner consumption requires a larger refactor of SPEC writer internals.

## File Impact Analysis

| Path | Action | Purpose |
| --- | --- | --- |
| `content/skills/idea.md` | Modify | Canonical `auto idea` workflow contract and BS schema. |
| `templates/codex/skills/auto-idea.md.tmpl` | Modify | Codex long-form `@auto-idea` skill parity. |
| `templates/codex/prompts/auto-idea.md.tmpl` | Modify | Codex prompt entry contract. |
| `templates/codex/skills/idea.md.tmpl` | Modify | Codex generated idea skill parity. |
| `templates/gemini/skills/idea/SKILL.md.tmpl` | Modify | Gemini generated idea skill parity. |
| `templates/claude/commands/auto-router.md.tmpl` | Modify | Claude `/auto idea` router section and `/auto plan --from-idea` handoff. |
| `templates/gemini/commands/auto-router.md.tmpl` | Modify | Gemini `/auto idea` router section and `/auto plan --from-idea` handoff. |
| `templates/codex/skills/auto-plan.md.tmpl` | Modify | Plan workflow instructions for `## Clarification Ledger` consumption. |
| `templates/codex/prompts/auto-plan.md.tmpl` | Modify | Codex plan prompt handoff behavior. |
| `content/agents/spec-writer.md` | Modify | Canonical spec-writer handling for ledger rows. |
| `templates/codex/agents/spec-writer.toml.tmpl` | Modify | Codex spec-writer generated guidance parity. |
| `templates/gemini/agents/spec-writer.md.tmpl` | Modify | Gemini spec-writer generated guidance parity. |
| `internal/cli/orchestra_brainstorm.go` | Modify | Brainstorm prompt honors supplied ledger and debate focus. |
| `[NEW] internal/cli/orchestra_brainstorm_clarify_test.go` | Add | Prompt contract tests for ledger, assumptions, and provider instructions. |
| `templates/*` or `pkg/adapter/*` tests | Modify/Add | Rendered-surface parity checks for ledger fields and plan handoff. |

## Tasks

- [x] T1: Update `auto idea` source guidance to define the Deep Interview-inspired gate, ledger schema, question budget, stop rule, and external provenance boundary.
- [x] T2: Add a reusable contract section or helper guidance for expected-gain ranking: `impact_weight * (1 - confidence/10)`, one-question output format, evidence-first filling, and `--auto` assumption confidence caps.
- [x] T3: Update BS output templates/instructions so saved BS files include `## Clarification Ledger` with the exact required columns and row statuses.
- [x] T4: Update `buildBrainstormPrompt` and related prompt tests so orchestra providers receive the ledger, skip re-asking `answered` rows, and use `assumed`/`deferred` rows as debate focus.
- [x] T5: Update `auto plan --from-idea` and spec-writer source guidance so ledger rows map into requirements, non-goals, risks, acceptance seeds, research/open questions, and reviewer focus while preserving legacy BS behavior.
- [x] T6: Add focused tests for the interactive question selection contract, `--auto` assumption behavior, BS ledger table shape, brainstorm prompt handoff, planner mapping, legacy no-ledger fallback, and external provenance boundary.
- [x] T7: Add cross-platform source/template parity tests so Claude, Codex, Gemini, and OpenCode-compatible surfaces contain the same clarification contract and generated surface edits remain source-owned.
- [x] T8: Run focused verification: `go test ./internal/cli -run 'Brainstorm|Clarif'`, template/content tests covering `auto-idea` and `auto-plan`, and a source scan for `Clarification Ledger`, `--deep-clarify`, `Current understanding`, `Plan Handoff`, and external provenance language.

## Architecture Considerations

- Source of truth is `autopus-adk/`; module-specific SPEC storage is `autopus-adk/.autopus/specs/`.
- The gate must not bypass existing 2-round debate enforcement, ICE scoring, or BS ID uniqueness rules.
- The planner handoff must treat BS content as untrusted prompt evidence, consistent with `SPEC-ACCGATE-002`.
- Existing `auto plan --from-idea` behavior remains valid for historical BS files without a ledger.

## Risks And Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Template drift across platforms | High | Add source/template parity tests and update canonical content first. |
| `--auto` assumptions become false certainty | High | Cap inferred confidence and require `If Wrong` consequences. |
| Planner handoff becomes vague prose | High | Require concrete mapping oracles in acceptance and spec-writer guidance. |
| Tests overfit exact Korean/English phrasing | Medium | Assert stable contract tokens, fields, and mapping behavior rather than every sentence. |
| External skill reference becomes supply-chain input | Medium | Record repository, commit, path, license, and hash as provenance only. |

## Dependencies

- `BS-051` for idea context and external provenance.
- Existing ADK source template generation and adapter parity test infrastructure.
- Existing `auto idea` and `auto plan --from-idea` source guidance.

## Exit Criteria

- [x] BS files produced by the updated flow include the required `Clarification Ledger`.
- [x] `--auto` mode never waits for clarification input and records assumptions/deferred questions.
- [x] Orchestra brainstorm prompts consume the ledger and preserve debate focus.
- [x] `auto plan --from-idea` guidance consumes ledger rows and remains backward-compatible.
- [x] Focused tests and scans pass without editing generated root surfaces directly.
