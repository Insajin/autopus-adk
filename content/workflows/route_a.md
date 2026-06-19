# Route A Workflow — Human Contract (Source of Truth)

This document and its sibling `route_a.schema.json` are the **machine- and
human-authoritative source of truth** for the harness `/auto go --workflow`
Route A. The Workflow JS adapter
(`templates/claude/workflows/route_a.workflow.js.tmpl` and the installed
`.claude/workflows/route_a.workflow.js`) is a **generated surface** derived from
this manifest by `auto generate-templates`. Do not edit the JS by hand; edit the
manifest and regenerate.

A parity gate compares the phase-id, retry, budget, and result-type sets across
this markdown, `route_a.schema.json`, and the derived JS. Any divergence fails
generation closed and names the diverging element.

## Phases (in execution order)

The deterministic workflow runs exactly four ordered phases. The phase-ids below
are authoritative and must match `route_a.schema.json` exactly.

### planning

The planning phase produces the implementation plan and task breakdown. It does
not mutate the repository working tree beyond plan artifacts.

### implementation

The implementation phase performs the repository-mutating work. Worktree
creation, branch naming, the worktree slot cap, and worktree reclaim stay in the
Go runtime (`pkg/pipeline`); the workflow JS only owns sequencing.

### gate_build_test

The `gate_build_test` phase is the **deterministic gate**. Its verdict derives
from build and test command **exit codes** (`verdict_source: exit_code`), not
from an LLM verdict. The workflow JS shells out to the `auto workflow gate` CLI
subcommand — this is the **JS-to-Go execution bridge** — which runs build and
test through an injectable `CommandRunner` seam and emits a structured
`{verdict, verdict_source, build_exit, test_exit}` JSON. The JS parses that JSON
to branch. A non-zero build or test exit code yields `verdict: fail`.

### release_hygiene

The `release_hygiene` terminal phase enforces release safety before sync:

- **Generated-surface drift gate**: blocks the run when generated surfaces
  (`.claude` / `.codex` / `.gemini` / `.opencode` / `.autopus/orchestra`) are
  staged without a corresponding source-of-truth change.
- **Lore commit format**: enforces the pending commit message via
  `auto check --lore --message <msgfile>`.
- **300-line source limit**: enforces the limit via `auto check --arch --staged`.

## JS-to-Go bridge

The `gate_build_test` phase is the only phase that crosses into Go execution for
its verdict. The bridge is the `auto workflow gate` CLI subcommand, keeping
exit-code adjudication in the Go runtime and out of the JS sequencing layer.
