# PRD — SPEC-ACCGATE-002

**Title**: Semantic invariant acceptance gate for Autopus workflows
**Status**: completed
**Created**: 2026-04-29
**Completed**: 2026-04-29
**Target module**: autopus-adk

## Problem

The harness benchmark showed that `autopus-go` produced SPEC ceremony and a Lore commit, but still shipped the same cross-harness pairing bug as the vanilla run. The generated SPEC preserved formulas and file constraints, yet the acceptance scenarios did not encode the key semantic invariant: paired statistics compare baseline and other runs by common `task_id` even when harness labels differ.

The same benchmark also showed weak workflow authenticity evidence. `autopus-go` claimed a full pipeline path but made zero subagent dispatch calls in the `claude --print` setting. This makes quality comparisons noisy because a pipeline can be represented in prose without actually exercising the pipeline.

## Goals

- Make spec generation capture semantic invariants from the original user request before acceptance criteria are written.
- Force algorithmic and comparative tasks to include oracle-style acceptance scenarios that assert behavior, not only document shape.
- Make tester and validator prompts reject structural-only tests when a Must acceptance criterion requires semantic output.
- Add explicit workflow authenticity evidence for `/auto go` so degraded single-agent execution is labeled or stopped instead of reported as a full pipeline.

## Non-Goals

- Do not change the benchmark runner in `autopus-harness-bench`.
- Do not require external statistics libraries or domain-specific oracle code in ADK.
- Do not redesign `auto spec review` verdict merging.
- Do not hotfix generated `.claude`, `.codex`, `.gemini`, or `.opencode` surfaces directly.

## Users

- Autopus operators comparing workflow quality.
- Spec writers and reviewers using `/auto plan` and `/auto go`.
- Implementers relying on acceptance criteria as executable behavioral guidance.

## Completion Outcome

When an Autopus workflow receives a task with algorithmic, paired, comparative, grouping, ordering, or cross-entity semantics, the generated SPEC and downstream tests preserve those invariants as traceable Must acceptance scenarios, and `/auto go` reports whether the intended subagent pipeline actually ran.

## Discovery Notes

- Benchmark task source: `<workspace>/autopus-harness-bench/bench/sandbox/wf-runs/TASK.md`.
- Benchmark matrix: `<workspace>/autopus-harness-bench/bench/sandbox/wf-runs/MATRIX.md`.
- Failed generated SPEC: `<workspace>/autopus-harness-bench/bench/sandbox/wf-runs/autopus-go/.autopus/specs/SPEC-HARN-BENCH-009/`.
- Direct failure pattern: `acceptance.md` AC-11 used overlapping `task_id` rows under the same harness label; the verifier used `claude` baseline rows versus `autopus` other rows.

## Success Metrics

- A generated acceptance scenario for the benchmark-like task includes heterogeneous harness labels and common `task_id` pairing.
- Tester guidance requires numeric or row-level assertions for paired statistics.
- Validator guidance fails tests that only check section headings when semantic output values are required.
- `/auto go` completion includes explicit subagent dispatch evidence or a degraded-mode blocker.

## Risks

- Overfitting to the `bench summary` task would add noise. Mitigation: describe semantic invariant classes generically and use the benchmark only as a regression example.
- More acceptance scenarios can slow small tasks. Mitigation: trigger the oracle requirement only for Must requirements with algorithmic, comparative, grouping, paired, ordering, or cross-entity semantics.
- Hard-stopping degraded pipeline execution can surprise users in `--print` mode. Mitigation: provide `--solo` as the explicit opt-in path for single-session work.
