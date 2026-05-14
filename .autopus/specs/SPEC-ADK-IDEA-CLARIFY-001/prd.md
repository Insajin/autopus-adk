# SPEC-ADK-IDEA-CLARIFY-001 PRD: Deep Interview Clarification Gate For Auto Idea

## Problem

`auto idea` already structures rough prompts into What/Why/Who/When, assumptions, orchestra debate, and ICE scoring. The weak point is the pre-orchestra clarification step: it can ask broad questions, infer too much, or let critical uncertainty disappear before the BS file reaches `auto plan --from-idea`.

## Goal

Add a Deep Interview-inspired clarification gate that asks the single highest-impact question in interactive mode, records conservative assumptions in `--auto` mode, writes a structured `Clarification Ledger` into BS files, and makes `auto plan --from-idea` consume that ledger as requirements, risks, non-goals, acceptance seeds, and reviewer focus.

## Users

- ADK users running `auto idea` with rough feature prompts.
- Maintainers evolving Autopus workflow prompts and generated surfaces.
- SPEC writers and planners consuming BS files through `auto plan --from-idea`.

## Requirements Summary

| ID | Requirement |
| --- | --- |
| `R1` | The gate reads project docs and relevant code before asking the user. |
| `R2` | The gate maintains ledger fields for `goal`, `scope_boundary`, `constraints`, `done_evidence`, and `brownfield_impact`. |
| `R3` | Interactive mode asks one highest-impact question at a time using a fixed four-line format. |
| `R4` | `--auto` never blocks; it records assumptions and deferred questions with bounded confidence. |
| `R5` | BS output includes the ledger and plan handoff notes. |
| `R6` | `auto plan --from-idea` maps ledger rows into SPEC requirements, risks, acceptance seeds, open questions, non-goals, and reviewer focus. |
| `R7` | External `deep-interview` skill content is treated as pinned reference evidence, not executable imported instructions. |

## Success Metrics

- New BS files contain a `## Clarification Ledger` table with the five required fields and row status values `answered`, `assumed`, or `deferred`.
- In interactive fixtures, the first question targets the field with the highest expected gain rather than asking a generic multi-question checklist.
- In `--auto` fixtures, no user prompt is required and inferred values have confidence `6` or lower unless backed by user text or project docs.
- `auto plan --from-idea` fixtures produce non-goals from `scope_boundary`, risks/open questions from `assumed` and `deferred` rows, and acceptance seeds from `done_evidence`.

## Non-Goals

- Do not replace the orchestra debate or ICE scoring pipeline.
- Do not require installing or executing `https://github.com/devbrother2024/skills`.
- Do not vendor upstream `deep-interview` skill text into runtime prompts.
- Do not edit generated root `.codex/**`, `.gemini/**`, `.opencode/**`, `.claude/**`, or plugin cache files directly.
- Do not add an unlimited interview flow.

## Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| The interview slows down idea capture. | Medium | Default to one question, allow one critical follow-up, and keep `--auto` non-blocking. |
| Assumptions narrow divergent brainstorming too much. | High | Mark inferred rows as assumptions, cap confidence, and pass open questions as debate focus rather than hard facts. |
| Planner ignores the ledger. | High | Include planner handoff in this SPEC rather than leaving it as vague follow-up. |
| External skill provenance expands the trust boundary. | Medium | Store provenance as data only; do not execute or copy upstream instructions into trusted prompt layers. |

## Practitioner Q&A

| Question | Answer |
| --- | --- |
| Should this be a separate command? | No. It belongs inside the existing `auto idea` flow before orchestra fan-out. |
| Should `--deep-clarify` be required? | No. It is an optional deeper budget, with default behavior still one question. |
| Should the first slice include planner consumption? | Yes. Without planner consumption, the ledger improves BS readability but not SPEC quality. |
