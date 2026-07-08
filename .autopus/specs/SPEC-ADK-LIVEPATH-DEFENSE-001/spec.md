# SPEC-ADK-LIVEPATH-DEFENSE-001: Wire Advertised Defenses Onto The Live Execution Path

**Status**: completed
**Created**: 2026-07-08
**Domain**: ADK-LIVEPATH-DEFENSE
**Priority**: HIGH
**Source**: SPEC-writer request (design-audit remediation)
**Target module**: `autopus-adk/`
**Module ownership**: `autopus-adk/.autopus/specs/SPEC-ADK-LIVEPATH-DEFENSE-001/**`
**Related**: `SPEC-ADK-SAFE-RAILS-001`

## Purpose

A design audit found three independent cases where the harness already implements a
defense or safety mechanism, but that mechanism is not connected to the code path that
actually runs. Each gap is an "advertised-but-unwired defense": the protective code
exists and is tested in isolation, yet the live execution path bypasses it. This SPEC
closes all three as one cohesive change story — "live-path defense wiring" — so that the
defenses the codebase claims to provide are the defenses that actually execute.

The three gaps were verified against the current source (the codebase is the source of
truth; audit line references were treated as untrusted hints and re-checked with `rg`
and direct reads):

- **Gap 1 (prompt-injection fence):** the subprocess debate path fences untrusted
  participant output with an unforgeable sentinel and a SECURITY NOTE, but the default
  interactive debate path pastes participant output raw.
- **Gap 2 (worktree lock retry):** the documented shared-lock retry/backoff lives in a
  worktree manager that production never constructs, while the wired parallel worktree
  path has no retry at all.
- **Gap 3 (experiment hard stop):** the experiment circuit breaker and `MaxIterations`
  are defaulted but never read by any loop-control code, so nothing stops runaway
  iteration.

## Background

Verified evidence (existing references, confirmed in the current tree):

- Fenced subprocess path: `pkg/orchestra/pipeline.go` sets `pb.Sentinel =
  sentinelForPreviousResults(prevAnon)` (round 2) and `sentinelForJudgeResults(judgeAnon)`
  (judge); templates `templates/shared/orchestra-debater-r2.md.tmpl` and
  `templates/shared/orchestra-judge.md.tmpl` emit a `SECURITY NOTE` plus
  `{{.Sentinel}}-BEGIN` / `{{.Sentinel}}-END` fences. The sentinel generator lives in
  `pkg/orchestra/debate_sentinel.go` (`newDebateSentinel`, `sentinelCollides`).
- Unfenced interactive path: `pkg/orchestra/debate.go` `buildRebuttalPrompt` writes
  `## Participant %c:` with the raw output, and `buildJudgmentPrompt` writes
  `### Participant %c:` with the raw output. No fence, no SECURITY NOTE. These two
  builders are the single assembly source reached by `runDebate` (`debate.go`),
  `executeRound` (`pkg/orchestra/interactive_debate_round.go`), and `runJudgeRound`
  (`pkg/orchestra/interactive_debate_helpers.go`). Path selection is
  `pkg/orchestra/pane_capable.go` `paneCapable`.
- Dead worktree retry: `pkg/pipeline/worktree.go` `WorktreeManager.addWorktreeWithRetry`
  implements base-3s factor-2 backoff on `isLockError`, but `pipeline.NewWorktreeManager`
  is constructed only by tests (`pkg/pipeline/worktree_test.go`,
  `pkg/pipeline/worktree_internal_test.go`, `pkg/workflow/boundary_test.go`). The
  `pkg/pipeline` package is imported by CLI code for other symbols, but its
  `WorktreeManager` type is unreachable in production.
- Wired worktree path (no retry): `pkg/worker/parallel/worktree.go` `WorktreeManager.Create`
  runs a single `git worktree add`; it is constructed at `pkg/worker/loop_runtime.go`
  and called from `pkg/worker/worktree_safety.go` (`assignTaskWorktree`,
  `assignPipelineWorktree`).
- Unused experiment hard stop: `pkg/experiment/circuit.go` `CircuitBreaker` and
  `pkg/experiment/types.go` `Config.MaxIterations` / `Config.CircuitBreakerN` are used
  only by tests and defaults. `internal/cli/experiment_helpers.go` writes
  `cfg.MaxIterations` from a flag but no loop reads it. `internal/cli/experiment.go`
  exposes stateless per-iteration subcommands (`init`, `metric`, `record`, `commit`,
  `reset`, `summary`, `status`); iteration is driven externally.

## Outcome Boundary

- **Outcome Lock:** every advertised defense in the three audited cases runs on the path
  that actually executes — the interactive debate path fences untrusted participant
  output identically to the subprocess path; the wired worktree-create path performs the
  documented shared-lock retry/backoff with exactly one retry implementation remaining;
  and an in-process experiment loop runner enforces `MaxIterations` plus circuit-breaker
  no-progress as a hard stop, reachable from a live entrypoint.
- **Mandatory requirements:** REQ-LPD-01 through REQ-LPD-12 below.
- **Explicit non-goals:** non-atomic checkpoint writes in
  `pkg/pipeline/checkpoint.go` / `runner.go`; single-source-of-truth and
  policy-duplication drift including the stale `ARCHITECTURE.md` generator. These are
  separate future candidates and are recorded, without SPEC/task/acceptance IDs, in
  `research.md` `## Evolution Ideas`.
- **Completion evidence:** focused Go unit tests with deterministic oracles for each gap
  (injection ordering, retry attempt count plus retry-exhaustion delay sequence,
  iteration/stop-reason counts including timeout), a live CLI test that observes
  `stop_reason` and `total_iterations`, regression tests proving the subprocess fence and
  existing worktree/experiment public APIs are unchanged, and a passing
  `auto spec validate .../SPEC-ADK-LIVEPATH-DEFENSE-001 --strict`.

## Requirements

### Gap 1 — Interactive debate prompt-injection fence

**REQ-LPD-01**
Priority: Must
Type: Event-driven
WHEN the interactive or process debate path assembles a rebuttal prompt from other participants' outputs, THE SYSTEM SHALL fence each participant output between a per-round unforgeable sentinel `-BEGIN` and `-END` boundary and SHALL precede the fenced region with the untrusted-data SECURITY NOTE.

**REQ-LPD-02**
Priority: Must
Type: Event-driven
WHEN the interactive or process debate path assembles a judgment prompt, THE SYSTEM SHALL apply the same sentinel fence and SECURITY NOTE contract that the subprocess pipeline already applies to untrusted participant output.

**REQ-LPD-03**
Priority: Must
Type: Unwanted
IF a participant output contains a forged fence marker, a forged section header, or an "ignore previous instructions" directive, THEN THE SYSTEM SHALL retain that text inside the fenced data region so that it is not presentable to the next round or the judge as a top-level harness instruction.

**REQ-LPD-04**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL derive each debate fence sentinel so that the sentinel does not occur as a substring within any fenced participant output.

### Gap 2 — Wired worktree shared-lock retry

**REQ-LPD-05**
Priority: Must
Type: Event-driven
WHEN the wired parallel worktree-create path receives a git shared-lock error such as `refs.lock` or `packed-refs.lock`, THE SYSTEM SHALL retry the worktree add with exponential backoff of base three seconds and factor two for up to three retries before returning failure.

**REQ-LPD-06**
Priority: Must
Type: Unwanted
IF a worktree-create attempt fails with a non-lock error, THEN THE SYSTEM SHALL stop retrying and return that error without further attempts.

**REQ-LPD-07**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL retain exactly one shared-lock retry implementation and one lock-error classifier so that no unreachable duplicate worktree-retry path remains in the codebase.

### Gap 3 — Experiment loop hard stop

**REQ-LPD-08**
Priority: Must
Type: State-driven
WHILE the experiment loop runner is iterating, THE SYSTEM SHALL stop before beginning any iteration whose index would exceed the configured `MaxIterations` value and SHALL report a max-iterations stop reason.

**REQ-LPD-09**
Priority: Must
Type: State-driven
WHILE consecutive no-improvement iterations reach the configured `CircuitBreakerN` threshold, THE SYSTEM SHALL hard-stop the experiment loop and SHALL report a circuit-breaker stop reason.

**REQ-LPD-10**
Priority: Must
Type: Event-driven
WHEN an experiment loop is started from its live CLI entrypoint, THE SYSTEM SHALL drive every iteration through the loop runner so that the `MaxIterations` and circuit-breaker limits are enforced in process rather than by an external caller.

Observable output contract: the live CLI entrypoint exposes the loop stop reason and total recorded iteration count so automated tests can verify the enforced stop.

**REQ-LPD-11**
Priority: Should
Type: Unwanted
IF the experiment loop context is cancelled or the configured `ExperimentTimeout` elapses, THEN THE SYSTEM SHALL stop the loop and SHALL report the corresponding stop reason.

### Cross-cutting — brownfield preservation

**REQ-LPD-12**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL preserve the existing subprocess-path fencing behavior and the existing worktree and experiment public APIs that current callers rely on.

Preserved worktree public API includes `pkg/pipeline.NewWorktreeManager`, `Create`, `Remove`, and `ActiveCount`; the dead retry duplicate is removed without deleting or moving those public symbols.

## Affected Files

Existing (verified) files to change:

- `pkg/orchestra/debate.go` — fence `buildRebuttalPrompt` and `buildJudgmentPrompt`.
- `pkg/orchestra/debate_sentinel.go` — reuse `newDebateSentinel`; add a thin
  `[]ProviderResponse` sentinel wrapper if needed.
- `pkg/worker/parallel/worktree.go` — add shared-lock retry/backoff to `Create` behind an
  injectable command runner seam and validate task IDs before branch/path construction.
- `pkg/experiment/cmdvalidate.go` — reject background shell operators in metric commands
  so the new live experiment loop cannot detach runaway metric work.
- `pkg/pipeline/worktree.go` — remove the dead `WorktreeManager` retry duplicate while
  preserving `pkg/pipeline.NewWorktreeManager`, `Create`, `Remove`, and `ActiveCount`.
  Update existing `pkg/pipeline/worktree_test.go`, `pkg/pipeline/worktree_internal_test.go`,
  and `pkg/workflow/boundary_test.go` references only as needed to remove dependency on the
  deleted retry helper; do not delete the public API tests.
- `internal/cli/experiment.go` — add the `run` subcommand that drives the loop runner.

Planned additions:

- `[NEW] pkg/orchestra/debate_fence_test.go`
- `[NEW] pkg/worker/parallel/worktree_retry.go`
- `[NEW] pkg/worker/parallel/worktree_retry_test.go`
- `[NEW] pkg/worker/taskid/taskid.go`
- `[NEW] pkg/worker/taskid/taskid_test.go`
- `[NEW] pkg/experiment/loop.go`
- `[NEW] pkg/experiment/loop_test.go`
- `[NEW] internal/cli/experiment_run_test.go`

## Related SPECs

None required. This is a standalone audit-remediation Primary SPEC. It is thematically
adjacent to `SPEC-ADK-SAFE-RAILS-001` (worktree safety rails) but does not depend on it.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-LPD-01 | T1, T2 | AC-LPD-001, AC-LPD-002 | INV-001, INV-002 |
| REQ-LPD-02 | T1, T2 | AC-LPD-001, AC-LPD-002 | INV-001, INV-002 |
| REQ-LPD-03 | T1, T2 | AC-LPD-001 | INV-001 |
| REQ-LPD-04 | T1, T2 | AC-LPD-001 | INV-001 |
| REQ-LPD-05 | T3, T5 | AC-LPD-003 | INV-003 |
| REQ-LPD-06 | T3, T5 | AC-LPD-004 | INV-004 |
| REQ-LPD-07 | T4, T5 | AC-LPD-004 | INV-004 |
| REQ-LPD-08 | T6, T8 | AC-LPD-005 | INV-005 |
| REQ-LPD-09 | T6, T8 | AC-LPD-006 | INV-006 |
| REQ-LPD-10 | T7, T8 | AC-LPD-008 | INV-005 |
| REQ-LPD-11 | T6, T8 | AC-LPD-007 | INV-005 |
| REQ-LPD-12 | T9 | AC-LPD-008 | INV-002 |
