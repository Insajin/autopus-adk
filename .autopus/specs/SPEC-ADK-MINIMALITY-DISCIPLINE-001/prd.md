# PRD Minimal: Default Minimality Discipline For Autopus Workflows

> Lightweight PRD for introducing Autopus's default "only implement what is needed" work discipline across existing plan/go/fix/review flows.

---

## Quick Discovery Check

- **Problem**: Autopus can over-plan, add abstractions, or accept new dependencies without consistently proving that existing code, native platform features, or current dependencies were checked first.
- **Who**: Autopus users who already run `@auto plan`, `@auto go`, `@auto fix`, and `@auto review` without wanting another mode toggle to manage.
- **Success metric**: 100% of affected workflow guidance emits or preserves a minimality decision receipt and requires justification for new dependencies or abstractions.
- **Not doing**: Do not expose a user-managed lean/Ponytail mode and do not vendor Ponytail upstream content.

---

## 1. Problem

Autopus already has strong gates for SPEC quality, TDD, security, accessibility, QAMESH evidence, and generated-surface hygiene. It does not yet have a single default discipline that makes every normal workflow ask whether the proposed work is necessary, whether existing code or native features can solve it, and whether the verification plan is sufficient without expanding scope.

The desired behavior is not "write fewer lines." It is "check the existing path first, implement only what closes the outcome, and explain the important choices." Users should keep using the same commands and see a short decision receipt, not manage a separate mode state.

## 2. Requirements

- WHEN `@auto plan` creates a SPEC, THE SYSTEM SHALL include a Minimality Decision Matrix in `research.md` covering need, existing code, native/stdlib, existing dependencies, new dependency or abstraction justification, and minimum sufficient verification.
- WHEN `@auto go` or the agent pipeline prepares implementation work, THE SYSTEM SHALL inject the minimality ladder as stable instruction before executor/planner task assignment.
- WHEN `@auto fix` scopes a bug fix, THE SYSTEM SHALL inspect caller and shared root-cause paths before accepting a symptom-only patch plan.
- WHEN `@auto review` reports findings, THE SYSTEM SHALL separate correctness/security findings from complexity findings.
- THE SYSTEM SHALL preserve existing validation, security, accessibility, data-loss, and generated-surface gates as non-reducible safety requirements.
- THE SYSTEM SHALL show a short final decision receipt using user-facing phrasing such as "implemented only what was needed" or "important decisions," not a mode name.

## 3. Technical Notes

**Constraints:**
- Source of truth is `autopus-adk/`.
- Root generated surfaces under `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, plugin cache, and runtime artifacts are not edited directly.
- Ponytail upstream `DietrichGebert/ponytail` at user-provided main HEAD `c4d1925ae9b76a1b641877328209ad25cfeb5ef2` is provenance only. MIT license notice is required only if text or code is copied or vendored, which this PRD does not require.
- No new external dependency is expected for this work.

**Dependencies:**
- Existing ADK template and content parity infrastructure.
- Existing `qualityloop` improvement candidate lifecycle.
- Existing `skillevolve` quarantine, replay, approval, and promotion flow.

**Impact on existing code:**
- Updates source-owned skills, prompt templates, agent guidance, review instructions, and focused tests in `autopus-adk/`.
- Adds qualityloop/skillevolve classification and routing only for repeated unnecessary dependency, duplicate helper, and single-implementation abstraction signals.

## 4. Out of Scope

- No user-facing `lean`, `minimal`, or `Ponytail` mode toggle.
- No vendoring or installing Ponytail hooks.
- No replacement of correctness, security, accessibility, or data-safety review.
- No line-count target as a success metric.
- No direct edits to root generated surfaces.

## 5. Pre-mortem (Quick)

| Risk | Mitigation |
|------|------------|
| The discipline becomes vague "keep it short" advice. | Specify the concrete ladder and acceptance oracles for existing path, stdlib/native, existing dependency, justification, and verification. |
| Complexity review blocks valid safety work. | Mark security, validation, accessibility, and data-loss handling as non-reducible. |
| Cross-platform templates drift. | Add parity tests across source content and Codex/Gemini/Claude routed surfaces. |
| Qualityloop starts auto-applying simplification patches. | Keep candidates inactive and require existing quarantine/replay/approval gates. |

## 6. Key Q&A

**Q1:** Should users enable a mode?
**A1:** No. This is the default Autopus work discipline inside existing workflows.

**Q2:** Can a user still request a new dependency or abstraction?
**A2:** Yes. Explicit user intent is respected, but the SPEC records justification and verification evidence.

**Q3:** Is "less code" the metric?
**A3:** No. The metric is justified scope and sufficient verification, not line count.

**Q4:** Does this weaken security or accessibility gates?
**A4:** No. Those gates are explicitly non-reducible.
