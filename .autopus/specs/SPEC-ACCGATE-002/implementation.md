# Implementation — SPEC-ACCGATE-002

## Summary

Completed on 2026-04-29.

This implementation hardens the ADK prompt/rule/template source of truth so SPEC generation preserves semantic invariants through research, requirements, plan tasks, acceptance scenarios, tester guidance, and validator coverage checks. It also adds workflow authenticity evidence requirements for default subagent pipeline execution.

## Completed Changes

- Added `Q-COMP-05` to `content/rules/spec-quality.md` for semantic invariant traceability to requirements, plan tasks, and Must oracle acceptance criteria.
- Updated `content/agents/spec-writer.md` to require `## Semantic Invariant Inventory`, source-clause handling as untrusted evidence, and oracle acceptance mapping for paired/comparative/grouping/ordering/deduplication/parser/report/numeric semantics.
- Updated `content/agents/tester.md` and `content/agents/validator.md` so Must oracle acceptance criteria require concrete behavioral assertions and structural-only tests do not satisfy coverage.
- Updated `content/skills/agent-pipeline.md` and Claude/Codex/Gemini route templates to require subagent surface preflight, `subagent_dispatch_count`, `subagent_roles_dispatched`, and explicit `degraded-mode` reporting.
- Mirrored semantic-invariant and workflow-authenticity language across Codex, Gemini, Claude, and OpenCode surfaces through source templates and adapter regression tests.
- Added regression coverage in `templates/template_test.go` and `pkg/adapter/opencode/opencode_test.go`.

## Verification

Run from `autopus-adk`:

```bash
go test ./templates ./pkg/content ./pkg/spec
go test ./internal/cli -run 'Test.*Template|Test.*Preview|Test.*Verify' -count=1
go test ./... -run 'TestTemplate|TestSkill|TestAgent|TestSpec|TestReview|TestValidator' -count=1
rg -n "Semantic Invariant Inventory|oracle acceptance|subagent_dispatch_count|structural-only" content templates pkg/adapter/opencode
```

Observed result:

- `go test ./templates ./pkg/content ./pkg/spec`: PASS
- `go test ./internal/cli -run 'Test.*Template|Test.*Preview|Test.*Verify' -count=1`: PASS
- `go test ./... -run 'TestTemplate|TestSkill|TestAgent|TestSpec|TestReview|TestValidator' -count=1`: PASS
- `rg` check: PASS, expected phrases found in source-of-truth content/templates and OpenCode adapter regression coverage

## @AX Lifecycle

`@AX: no-op`

Changed files were scanned for `@AX` lifecycle targets. The only matches in the changed paths are workflow/template instructions about @AX handling itself; no actionable `@AX:TODO`, changed source `@AX:ANCHOR`, or orphaned changed-path `@AX:NOTE` required modification.

## Notes

- Generated workspace outputs under `.claude`, `.codex`, `.gemini`, `.opencode`, `.agents`, or `.autopus/plugins` were not edited directly.
- Full harness benchmark re-runs remain outside this SPEC and belong to the benchmark repo follow-up workflow.
