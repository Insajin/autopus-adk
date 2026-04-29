# Acceptance — SPEC-ACCGATE-002

## Scenarios

### S1: Semantic invariant inventory is created

Priority: Must

Given a user task states that paired statistics use common `task_id` values across baseline and other runs.
When `spec-writer` creates the SPEC document set.
Then `research.md` contains `## Semantic Invariant Inventory`.
And the inventory includes a row for common-`task_id` pairing with the source clause and at least one acceptance ID.

### S2: Cross-entity pairing becomes oracle acceptance

Priority: Must

Given a generated SPEC covers a summary report comparing baseline rows with `harness="claude"` against other rows with `harness="autopus"`.
When `acceptance.md` is inspected.
Then at least one Must scenario uses overlapping `task_id` values across different harness labels.
And the scenario asserts expected Cohen's d or McNemar values with an explicit tolerance.

### S3: Structural-only markdown checks are insufficient

Priority: Must

Given a Must oracle acceptance criterion requires a McNemar p-value for paired rows.
When the tester creates a test that only checks Markdown section headings.
Then validator acceptance coverage reports that the criterion is not addressed.
And Gate 2 verdict is FAIL with Recommended Agent set to tester or executor according to whether the missing piece is test coverage or implementation.

### S4: Tester creates behavior assertions for oracle criteria

Priority: Must

Given acceptance criteria include exact output rows, numeric tolerances, or matching rules.
When Phase 1.5 or Phase 3 tester guidance is followed.
Then the generated tests assert concrete output values, row contents, JSON fields, stdout substrings tied to values, or file content.
And tests that only assert successful execution are marked invalid for those criteria.

### S5: Spec quality checklist detects invariant loss

Priority: Must

Given `research.md` lists a semantic invariant and `acceptance.md` omits it.
When `content/rules/spec-quality.md` is applied during self-verification or review.
Then the new completeness check returns FAIL.
And the fix guidance points to `spec.md`, `plan.md`, and `acceptance.md` as potentially affected files.

### S6: Subagent pipeline mode records dispatch evidence

Priority: Must

Given `/auto go SPEC-ID --auto` selects default subagent pipeline mode.
When the workflow reaches completion guidance.
Then the response includes `subagent_dispatch_count`.
And the response lists the role names or phase names dispatched through the subagent surface.

### S7: Degraded pipeline without dispatch stops unless solo is explicit

Priority: Must

Given `/auto go SPEC-ID --auto` selects default subagent pipeline mode.
And the runtime cannot create or observe any subagent dispatch.
When the workflow attempts to enter Phase 1.
Then the workflow stops with a workflow authenticity blocker.
And the blocker tells the user to rerun with a working subagent surface or choose `--solo`.

### S8: Solo mode is labeled as solo

Priority: Must

Given `/auto go SPEC-ID --solo --auto` is used.
When the workflow completes.
Then the response reports `subagent_dispatch_count: 0`.
And the degraded-mode state is not presented as a full subagent pipeline.

### S9: Source-of-truth template parity is covered

Priority: Should

Given the implementation modifies source-of-truth prompt or rule text.
When template regression tests run.
Then Claude, Codex, Gemini, and OpenCode source templates include the semantic invariant and workflow authenticity language where those surfaces expose `/auto plan` or `/auto go`.

## Non-Functional Acceptance

- Source-of-truth edits stay in `autopus-adk/content/**`, `autopus-adk/templates/**`, and tests.
- Generated workspace copies are not edited directly.
- Existing SPEC review checklist behavior remains compatible with SPEC-SPECWR-001 and SPEC-SPECWR-002.
- File size limits for source files remain respected.

## Verification Commands

```bash
cd autopus-adk
go test ./templates ./pkg/content ./pkg/spec
go test ./internal/cli -run 'Test.*Template|Test.*Preview|Test.*Verify' -count=1
rg -n "Semantic Invariant Inventory|oracle acceptance|subagent_dispatch_count|structural-only" content templates
```
