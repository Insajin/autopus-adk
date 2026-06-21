# Route Team Workflow — Human Contract (Source of Truth)

This document and its sibling `route_team.schema.json` are the **machine- and
human-authoritative source of truth** for the harness `/auto go --team --workflow`
Route Team. The Workflow JS adapter
(`templates/claude/workflows/route_team.workflow.js.tmpl` and the installed
`.claude/workflows/route_team.workflow.js`) is a **generated surface** derived
from this manifest by `auto generate-templates`. Do not edit the JS by hand;
edit the manifest and regenerate.

This route is **claude-code scoped** (the `--team` opt-in path) and reuses the
shared deterministic workflow machinery already proven by Route A: the same
parity gate, the same `auto workflow gate` exit-code bridge, and the same
`auto check --hygiene` release-hygiene gate. Route Team only adds the additional
team phases (test scaffolding, annotation, testing, review) between planning and
release.

A parity gate compares the phase-id, retry, budget, result-type, and per-phase
model/effort/depth tokens across this markdown, `route_team.schema.json`, and the
derived JS. Any divergence fails generation closed and names the diverging
element.

## Phases (in execution order)

The deterministic team workflow runs exactly eight ordered phases. The phase-ids
below are authoritative and must match `route_team.schema.json` exactly.

### planning

The planning phase produces the implementation plan and task breakdown. It runs
the `planner` agent and does not mutate the repository working tree beyond plan
artifacts.

### test_scaffold

The `test_scaffold` phase produces failing test skeletons (the RED stage) ahead
of implementation. It runs the `test_scaffold` agent so the executor fan-out has
a concrete failing target to satisfy.

### implementation

The implementation phase performs the repository-mutating work through a
**bounded executor fan-out** (`fan_out_cap`). Worktree creation, branch naming,
the worktree slot cap, and worktree reclaim stay in the Go runtime
(`pkg/pipeline`); the workflow JS only owns sequencing and the bounded loop.

### gate_build_test

The `gate_build_test` phase is the **deterministic gate**. Its verdict derives
from build and test command **exit codes** (`verdict_source: exit_code`), not
from an LLM verdict. The workflow JS shells out to the `auto workflow gate` CLI
subcommand — the **JS-to-Go execution bridge** — which runs build and test
through an injectable `CommandRunner` seam and emits a structured
`{verdict, verdict_source, build_exit, test_exit}` JSON. A non-zero build or test
exit code yields `verdict: fail`.

### annotation

The annotation phase runs the `annotator` agent to apply `@AX` tags and other
structured annotations to the implemented changes.

### testing

The testing phase runs the `tester` agent to round out coverage beyond the
initial scaffold and confirm the suite is green.

### review

The review phase runs the `reviewer` agent (bounded by `verify_votes`) followed
by the `security_auditor` agent. An optional `synthesis` pass merges reviewer
findings when the schema enables it.

### release_hygiene

The `release_hygiene` terminal phase enforces release safety before sync:

- **Generated-surface drift gate**: blocks the run when generated surfaces are
  staged without a matching source-of-truth change, and always blocks runtime
  artifacts. The workflow JS shells out to `auto check --hygiene`.
- **300-line source limit**: enforces the staged source limit through the same
  workflow call: `auto check --hygiene --arch --quiet --staged`.

## JS-to-Go bridge

The `gate_build_test` and `release_hygiene` phases cross into Go execution for
deterministic verdicts. `gate_build_test` uses the `auto workflow gate` CLI
subcommand, while `release_hygiene` uses `auto check --hygiene --arch --quiet
--staged`. JS owns sequencing; Go owns policy and exit-code adjudication.
