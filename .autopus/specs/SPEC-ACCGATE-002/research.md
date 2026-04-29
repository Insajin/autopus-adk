# Research — SPEC-ACCGATE-002

## Existing Code Analysis

- `content/rules/spec-quality.md` already defines correctness, completeness, feasibility, style, and security checks. It catches traceability loss in general but does not specifically require semantic invariant extraction from the original task.
- `content/agents/spec-writer.md` already requires `Feature Coverage Map` and `Self-Verify Summary`. It does not require a separate inventory for matching, grouping, numeric formulas, or other domain invariants.
- `content/agents/tester.md` already bans tests that only assert `NoError` or `NotNil`. It does not explicitly reject heading-only or file-existence-only tests for oracle acceptance criteria.
- `content/agents/validator.md` already has Acceptance Coverage Verification. It checks whether scenarios are addressed, but it needs a stronger semantic-output assertion check for Must oracle criteria.
- `content/skills/agent-pipeline.md` defines the default `/auto go` subagent pipeline. It does not require completion evidence such as `subagent_dispatch_count`.
- `templates/claude/commands/auto-router.md.tmpl` owns the most complete Claude `/auto plan` and `/auto go` behavior. It must mirror any source-of-truth workflow language that affects generated Claude surfaces.
- `templates/codex/skills/agent-pipeline.md.tmpl` and `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl` own generated non-Claude pipeline guidance.

## Benchmark Evidence

The workflow-fair harness comparison reported:

- `autopus-fix` and `superpowers`: 7/7 verifier pass.
- `autopus-go`: 4/6 verifier pass, with the same cross-harness pairing defect as vanilla.
- `autopus-go` generated `SPEC-HARN-BENCH-009` and a Lore commit, but its acceptance AC-11 used one harness label for both baseline and other rows.
- The external verifier fixture paired `wf-fixture-base` rows with `harness="claude"` against `wf-fixture-other` rows with `harness="autopus"` by common `task_id`.
- `autopus-go` made zero subagent dispatch calls in the `claude --print` benchmark path despite presenting a full pipeline workflow.

## Root Cause

The failure chain is:

1. The original task specified paired McNemar and Cohen's d by common `task_id`.
2. The generated SPEC preserved the formulas but did not preserve the cross-harness pairing invariant in acceptance.
3. Tests derived from that acceptance used same-harness fixture rows.
4. Implementation grouped baseline and other rows by matching harness labels before pairing, so cross-harness comparisons produced no paired semantic result.
5. Validator saw the generated tests pass because the missing invariant was absent from acceptance.

The workflow authenticity issue is separate but visible in the same benchmark: a pipeline can be described in the transcript without observable subagent work. That weakens benchmark interpretation and hides degraded execution.

## Design Decision

Add a generic semantic invariant gate instead of a benchmark-specific rule.

The new gate focuses on classes of task semantics that often disappear during SPEC conversion:

| Invariant class | Example |
|-----------------|---------|
| Paired matching | common `task_id` across baseline and other runs |
| Cross-entity comparison | compare `harness="claude"` with `harness="autopus"` |
| Deduplication | last row wins for repeated IDs |
| Ordering | deterministic sorted rows or stable output order |
| Numeric oracle | exact formula result or tolerance |
| Parser/report contract | expected row, field, or section content |

This design works for benchmark tasks without tying ADK to benchmark internals.

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Extract semantic invariants from original request | SPEC-ACCGATE-002 REQ-001, T1, T2 | covered |
| Map invariants to requirements, plan, and acceptance | SPEC-ACCGATE-002 REQ-002..004, T1, T2 | covered |
| Force behavior tests for oracle criteria | SPEC-ACCGATE-002 REQ-005, T3 | covered |
| Fail validation on structural-only coverage | SPEC-ACCGATE-002 REQ-006, T4 | covered |
| Label or block degraded subagent pipeline execution | SPEC-ACCGATE-002 REQ-007..008, T5 | covered |
| Keep platform surface parity | SPEC-ACCGATE-002 REQ-009..010, T6, T7 | covered |
| Re-run full benchmark matrix repeatedly | Out of scope, `autopus-harness-bench` follow-up | deferred |

## Self-Verify Summary

| Q | status | attempt | files | reason |
|---|--------|---------|-------|--------|
| Q-CORR-01 | PASS | 1 | research.md, plan.md | Existing referenced files and benchmark paths were inspected before writing. |
| Q-CORR-02 | PASS | 1 | plan.md | Planned edits are described as future implementation tasks, not existing completed changes. |
| Q-CORR-03 | PASS | 1 | acceptance.md | Acceptance scenarios use parseable Given/When/Then steps with Priority metadata. |
| Q-COMP-01 | PASS | 1 | all | PRD, spec, plan, acceptance, and research serve distinct roles. |
| Q-COMP-02 | PASS | 1 | spec.md, plan.md, acceptance.md | Each requirement maps to implementation tasks and acceptance scenarios. |
| Q-COMP-03 | PASS | 1 | spec.md | EARS type, trigger, expected behavior, and observable evidence are explicit. |
| Q-COMP-04 | PASS | 1 | spec.md, research.md | The requested Autopus improvement is cohesive and covered by this SPEC. |
| Q-FEAS-01 | PASS | 1 | plan.md | Scope is prompt/rule/template hardening, matching the observed failure. |
| Q-FEAS-02 | PASS | 1 | plan.md | Owning repo is `autopus-adk`; generated surfaces are excluded from direct edits. |
| Q-FEAS-03 | PASS | 1 | plan.md, acceptance.md | Verification commands are runnable in `autopus-adk`. |
| Q-STYLE-01 | PASS | 1 | spec.md | Requirement text avoids ambiguous wording and uses decisive SHALL language. |
| Q-STYLE-02 | PASS | 1 | spec.md, acceptance.md | Priority and EARS type are separate axes. |
| Q-STYLE-03 | PASS | 1 | acceptance.md | Gherkin steps are readable and parseable. |
| Q-SEC-01 | PASS | 1 | spec.md, research.md | Original user requests and generated docs are treated as prompt inputs whose semantics must be traced, not trusted blindly. |
| Q-SEC-02 | PASS | 1 | plan.md | No secrets are required; absolute benchmark paths are evidence only. |
| Q-SEC-03 | PASS | 1 | spec.md, plan.md | New retained evidence is limited to workflow completion metadata and template tests. |
| Q-COH-01 | PASS | 1 | all | The SPEC focuses on preventing ceremony without semantic correctness or execution evidence. |
| Q-COH-02 | PASS | 1 | research.md | Repeated benchmark claims remain explicit out-of-scope follow-up. |
| Q-COH-03 | N/A | 1 | research.md | No sibling SPEC set is required for this cohesive change. |

## Open Questions

- The exact platform-specific signal for subagent dispatch may differ between Claude, Codex, Gemini, and OpenCode. Implementation should use surface-native evidence while preserving the same completion fields.
- OpenCode template ownership should be confirmed during implementation because part of the OpenCode surface may be adapter-generated rather than a single markdown template.
