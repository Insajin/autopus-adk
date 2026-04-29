# Implementation Plan — SPEC-ACCGATE-002

## Scope

This is a prompt, rule, and template hardening change in `autopus-adk`. It does not alter benchmark runner code or generated workspace output directly.

## File Impact

| Path | Action | Purpose |
|------|--------|---------|
| `content/rules/spec-quality.md` | edit | Add semantic invariant traceability check. |
| `content/agents/spec-writer.md` | edit | Require `Semantic Invariant Inventory` and oracle acceptance mapping. |
| `content/agents/tester.md` | edit | Require behavior assertions for oracle acceptance criteria. |
| `content/agents/validator.md` | edit | Fail structural-only tests that miss Must oracle criteria. |
| `content/skills/agent-pipeline.md` | edit | Add subagent route preflight and dispatch evidence reporting. |
| `templates/claude/commands/auto-router.md.tmpl` | edit | Mirror plan/go instructions for Claude surface. |
| `templates/codex/skills/agent-pipeline.md.tmpl` | edit | Mirror subagent route preflight and evidence reporting. |
| `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl` | edit | Mirror pipeline authenticity language. |
| `templates/opencode/**` or adapter-owned OpenCode workflow template | edit if present | Keep OpenCode route parity for core workflow text. |
| `templates/template_test.go` or focused template tests | edit/add | Assert required phrases appear in generated surfaces. |

Generated paths such as `.claude/**`, `.codex/**`, `.gemini/**`, `.opencode/**`, `.agents/**`, and `.autopus/plugins/**` are excluded from direct edits.

## Tasks

### T1 — Add semantic invariant quality rule

- Extend `content/rules/spec-quality.md` under completeness with a new check, for example `Q-COMP-05 — Semantic invariants are mapped to oracle acceptance`.
- PASS when each invariant in `research.md` maps to at least one requirement, plan task, and acceptance scenario.
- FAIL when an invariant is only described in prose or disappears before acceptance.
- Include the benchmark pairing case as an example, not as a hard-coded domain rule.

### T2 — Harden spec-writer instructions

- Update `content/agents/spec-writer.md` to require `research.md` section `## Semantic Invariant Inventory`.
- Require each inventory row to include source clause, invariant type, affected outputs, and acceptance IDs.
- Require oracle acceptance for algorithmic, paired, grouping, ordering, deduplication, report, parser, and numeric formula work.
- Update the self-verification loop to apply the new `Q-COMP-05` item.

### T3 — Harden tester instructions

- Update `content/agents/tester.md` Phase 1.5 and Phase 3 guidance.
- Add a rule that Must oracle criteria require tests with concrete return, file, stdout, row, JSON, or numeric assertions.
- Mark tests that only assert section headings, file existence, exit code, `NoError`, or non-empty output as invalid for oracle criteria.

### T4 — Harden validator instructions

- Update `content/agents/validator.md` acceptance coverage verification.
- Add a semantic-output mapping check after acceptance parsing.
- Fail Gate 2 if a Must oracle acceptance criterion has no test asserting the semantic output.
- Include a fix hint that points to tester for missing tests and executor for implementation mismatch.

### T5 — Add workflow authenticity preflight and evidence

- Update `content/skills/agent-pipeline.md` Route A.
- Require a subagent surface preflight before Phase 1 when mode is subagent pipeline.
- Require completion output to include `subagent_dispatch_count`, roles dispatched, and degraded-mode state.
- If no dispatch happens in subagent mode, fail the route with a blocker unless `--solo` was selected.

### T6 — Mirror platform templates

- Apply the same source-of-truth language to Claude, Codex, Gemini, and OpenCode templates.
- For Claude, `templates/claude/commands/auto-router.md.tmpl` owns much of `/auto plan` and `/auto go` behavior.
- For Codex and Gemini, use their `agent-pipeline` templates.
- For OpenCode, update the workflow template or adapter output source that owns `/auto go` parity.

### T7 — Add regression tests

- Add or update template tests to search generated template text for:
  - `Semantic Invariant Inventory`
  - `oracle acceptance`
  - `structural-only`
  - `subagent_dispatch_count`
  - degraded-mode blocker text.
- Keep tests focused on source-of-truth templates, not generated workspace copies.

## Feature Completion Scope

This SPEC closes the core Autopus improvement from the benchmark: SPEC ceremony must preserve semantic correctness, and pipeline claims must carry observable execution evidence. Full statistical confidence for harness ranking remains outside this SPEC and belongs to repeated benchmark runs in `autopus-harness-bench`.

## Verification

Run from `autopus-adk`:

```bash
go test ./templates ./pkg/content ./pkg/spec
go test ./internal/cli -run 'Test.*Template|Test.*Preview|Test.*Verify' -count=1
go test ./... -run 'TestTemplate|TestSkill|TestAgent|TestSpec|TestReview|TestValidator' -count=1
```

Manual verification:

```bash
rg -n "Semantic Invariant Inventory|oracle acceptance|subagent_dispatch_count|structural-only" content templates
```

Benchmark regression target for a later run:

```bash
cd <workspace>/autopus-harness-bench
python3 bench/sandbox/wf-runs/verify.py bench/sandbox/wf-runs/autopus-go
```

## Risks

- Template parity can drift across Claude, Codex, Gemini, and OpenCode. Mitigate with template tests.
- Overly broad invariant extraction can increase acceptance noise. Mitigate by binding oracle requirements to Must criteria with algorithmic or cross-entity semantics.
- Subagent preflight wording must match actual platform capabilities. Mitigate by making the blocker evidence-based rather than naming one exact tool API across all platforms.
