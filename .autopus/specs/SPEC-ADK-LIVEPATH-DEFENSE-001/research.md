# SPEC-ADK-LIVEPATH-DEFENSE-001 Research

## Existing Code Analysis

All references below were verified with `rg` and direct file reads against the current
tree; audit line hints were treated as untrusted and re-confirmed.

### Gap 1 — debate prompt-injection fence

- Fenced (good) subprocess path:
  - `pkg/orchestra/pipeline.go` sets `pb.Sentinel = sentinelForPreviousResults(prevAnon)`
    for round 2 and `pb.Sentinel = sentinelForJudgeResults(judgeAnon)` for the judge.
  - `pkg/orchestra/debate_sentinel.go` provides `newDebateSentinel(outputs ...string)`,
    `sentinelCollides`, `sentinelForPreviousResults`, `sentinelForJudgeResults`. The
    sentinel base is `AUTOPUS_PART_` plus `randomHex()` (defined in
    `pkg/orchestra/pane_runner.go`), re-extended until absent from all outputs.
  - `templates/shared/orchestra-debater-r2.md.tmpl` and
    `templates/shared/orchestra-judge.md.tmpl` render a SECURITY NOTE followed by
    `{{.Sentinel}}-BEGIN` / `{{.Sentinel}}-END` around each `{{.Output}}`.
- Unfenced (bad) interactive/process path:
  - `pkg/orchestra/debate.go` `buildRebuttalPrompt` (writes `## Participant %c:` then the
    raw capped output) and `buildJudgmentPrompt` (writes `### Participant %c:` then the raw
    capped output). Neither emits a sentinel or SECURITY NOTE.
  - Callers reached only through these two builders: `runDebate` (`debate.go`, judgment and
    rebuttal), `executeRound` (`pkg/orchestra/interactive_debate_round.go`, rebuttal),
    `runJudgeRound` (`pkg/orchestra/interactive_debate_helpers.go`, judgment). Fencing the
    two builders closes all four sites at once.
  - Path selection is `pkg/orchestra/pane_capable.go` `paneCapable(term, subprocessMode)`;
    the interactive/pane path is the default when a real terminal is attached.
- Out of injection scope: `buildDebateMerged` / `FormatDebate` format the final result for
  the user, not a prompt fed back to a provider, so they are not an instruction surface.

### Gap 2 — worktree shared-lock retry

- Dead retry duplicate: `pkg/pipeline/worktree.go` `WorktreeManager.Create` calls
  `addWorktreeWithRetry`, which loops `lockRetryAttempts` (3) times with `lockRetryBase`
  (3s) and `lockRetryFactor` (2), retrying only when `isLockError(output)` matches
  `refs.lock`, `packed-refs.lock`, `shallow.lock`, etc. `pipeline.NewWorktreeManager` is
  constructed only in tests (`pkg/pipeline/worktree_test.go`,
  `pkg/pipeline/worktree_internal_test.go`, `pkg/workflow/boundary_test.go`). `pkg/pipeline`
  is imported by CLI (`internal/cli/pipeline*.go`) for other symbols, but its
  `WorktreeManager` type has no production constructor, so the retry never runs live.
- Wired path (no retry): `pkg/worker/parallel/worktree.go` `WorktreeManager.Create` runs a
  single `git -c gc.auto=0 worktree add <path> -b <branch>` and returns the error verbatim
  on failure. Constructed at `pkg/worker/loop_runtime.go` (`parallel.NewWorktreeManager(
  wl.config.WorkDir)`) and invoked from `pkg/worker/worktree_safety.go`
  (`assignTaskWorktree`, `assignPipelineWorktree`). On lock contention it fails immediately
  and routes to `handleWorktreeCreateFailure`, degrading isolation.
- Branch-name validation `ValidateBranchName` lives in `pkg/pipeline/branchvalidate.go`; a
  relocated shared helper must keep equivalent validation for the wired path. Implementation
  added a small `[NEW] pkg/worker/taskid` validator so the wired task ID cannot become an
  unsafe branch/path fragment before retry classification.

### Gap 3 — experiment loop hard stop

- `pkg/experiment/circuit.go` `CircuitBreaker` (`Record`, `IsTripped`,
  `ConsecutiveNoProgress`, `Reset`, default threshold 10) is exercised only by
  `pkg/experiment/circuit_test.go` and `coverage_extra_test.go`.
- `pkg/experiment/types.go` `Config.MaxIterations` (default 50) and `Config.CircuitBreakerN`
  (default 10). `internal/cli/experiment_helpers.go` writes `cfg.MaxIterations` from a flag;
  no code reads it to stop a loop. `CircuitBreakerN` is only a default.
- No loop runner exists. `internal/cli/experiment.go` exposes stateless per-iteration
  subcommands (`init`, `metric`, `record`, `commit`, `reset`, `summary`, `status`);
  iteration index is a `--iteration` flag, so the loop is driven externally with no
  in-process hard stop. Reusable primitives for a runner already exist: `Recorder`
  (`recorder.go`), `RunMetricWithTimeout` / `ExtractMetric` (`metric.go`), and `Git`
  (`git.go`, methods `CommitExperiment`, `ResetToCommit`, `CheckCleanWorktree`, ...).
  The live `run` entrypoint keeps using the existing metric command validator; sync-time
  implementation tightened it to reject the `&` background operator because a detached
  metric process would undercut the hard-stop guarantee.

## Outcome Lock

- **User-visible outcome:** the harness actually applies the defenses it advertises on the
  paths that run — injected participant instructions cannot escape the debate fence on the
  default interactive path; parallel worktree creation survives transient git lock
  contention; and an experiment loop cannot run away past its configured limits.
- **Mandatory requirements:** REQ-LPD-01 .. REQ-LPD-12.
- **Explicit non-goals:** checkpoint write atomicity
  (`pkg/pipeline/checkpoint.go`/`runner.go`); single-source-of-truth / policy-duplication
  drift and the stale `ARCHITECTURE.md` generator. Recorded as Evolution Ideas only.
- **Completion evidence:** deterministic unit-test oracles per gap (injection ordering,
  retry attempt count plus retry-exhaustion delay sequence, iteration/stop-reason counts
  including timeout), live CLI output proof for `stop_reason` and `total_iterations`,
  regression proof that the subprocess fence and existing worktree/experiment public APIs
  are unchanged, and a passing `auto spec validate --strict` for this SPEC package.

## Reviewer Brief

- **Intended scope:** wire three already-implemented defenses onto their live execution
  paths and remove one dead retry duplicate. Nothing more.
- **Explicit non-goals:** checkpoint atomicity and SSoT/generator drift are Evolution Ideas,
  not review targets. Do not expand scope into them or into broader orchestra/worker/
  experiment redesign.
- **Self-verified:** Traceability Matrix (spec.md), Semantic Invariant Inventory with oracle
  acceptance mapping, existing vs `[NEW]` Reference Discipline, and the two
  `auto spec validate --strict` parser traps (bullet-requirement skip; acceptance
  structural-only token check).
- **Reviewer should focus on:** correctness of the fence parity and injection oracle,
  retry attempt-count/exhaustion/delay and dedup correctness, loop hard-stop
  counter/reset/cancel/timeout correctness, live CLI output observability, convergence
  safety, and regression risk to the already-fenced subprocess path and existing
  worktree/experiment APIs including `pkg/pipeline.WorktreeManager`. Completion Debt is
  none.

## Visual Planning Brief

See `plan.md` `## Visual Planning Brief` for the BEFORE/AFTER command-flow diagram of where
each defense attaches to the live path. Summary: Gap 1 attaches the fence inside the two
`debate.go` builders (subprocess path already fenced); Gap 2 attaches retry to the wired
`parallel.WorktreeManager.Create` and collapses two retry copies into one; Gap 3 adds a
`Loop` runner plus an `auto experiment run` entrypoint so the hard stop runs in process.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go module `github.com/insajin/autopus-adk` | Go 1.26 (toolchain go1.26.4), cobra + testify already vendored | `autopus-adk/go.mod` | 2026-07-08 | No new runtime/deps; adding a dependency would violate the Minimality ladder |

This is brownfield: existing manifest versions are compatibility constraints, not freshness
evidence. No migration is in scope and no new dependency is introduced, so no external
version research is required beyond confirming the existing manifest.

## Design Decisions

- **Fence at the builder, not the caller (Gap 1).** The two `debate.go` builders are the
  only assembly points that format participant output for a downstream provider, so fencing
  them is the minimal change that covers every interactive/process caller. The sentinel is
  derived from the exact capped participant strings placed inside the fence, not from an
  earlier pre-cap intermediate, so REQ-LPD-04 is tied to the bytes actually emitted.
  Reusing `newDebateSentinel` and the existing SECURITY NOTE wording keeps the interactive
  contract byte-for-byte aligned with the subprocess templates (parity), which is exactly
  what INV-002 asserts.
- **Injectable command-runner seam (Gap 2).** Real lock contention is nondeterministic. A
  function-field seam over `git worktree add` lets tests simulate "lock on attempt 1, ok on
  attempt 2" and assert an exact attempt count without spawning concurrent git processes.
  An injectable backoff base/clock keeps tests fast and pins the persistent-lock boundary
  to 4 attempts with delays `base`, `base*2`, `base*4`. One shared `retryOnLock` +
  `isLockError` removes the duplicate implementation (INV-004, REQ-LPD-07). The older
  `pkg/pipeline.WorktreeManager` public API remains source-compatible; only its private
  retry/backoff helper is removed so `pkg/pipeline` does not import `pkg/worker/parallel`.
- **Loop runner owns iteration; CLI makes it live (Gap 3).** A `Loop` + `StepFunc` seam is
  the smallest structure that can own the iteration counter and circuit-breaker state and
  enforce a hard stop deterministically. Because the audit theme is "advertised-but-unwired
  defense," the runner must be reachable from a live entrypoint or it would become new dead
  code; the thin `auto experiment run` command is that entrypoint. Deterministic oracles are
  written against the runner with a fake `StepFunc`, including cancellation and timeout
  stop reasons. The CLI stays thin but prints `stop_reason` and `total_iterations` so the
  live path is observable in a command-level test.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock: three verified advertised-but-unwired defenses must run on the live path | proceed | Close Gap 1/2/3 as one cohesive wiring change |
| existing code/helper/pattern | `newDebateSentinel`/`sentinelCollides` (debate_sentinel.go), SECURITY NOTE wording (orchestra-*.tmpl), `isLockError`+backoff constants (pipeline/worktree.go), `CircuitBreaker`/`Recorder`/`Config`/`RunMetricWithTimeout` (pkg/experiment) | reuse | Reused sentinel generator, fence wording, breaker, recorder, metric runner |
| stdlib/native | `context`, `time.After` for backoff/timeout; `strings.Index`/`Contains` for oracle assertions | use | No new library for backoff, timeout, or string checks |
| existing dependency | cobra (CLI subcommand), testify (tests) already in go.mod | reuse | `auto experiment run` uses cobra; tests use testify |
| new dependency or new abstraction | No new dependency. New abstractions limited to: a fence formatter helper, an injectable worktree command-runner seam, and a `Loop`/`StepFunc` in pkg/experiment plus a thin CLI command | accepted | Seams justified only after reuse/stdlib checks; each is the minimum needed for wiring + deterministic tests |
| minimum sufficient verification | Focused Go unit oracles (injection ordering, attempt count plus exhaustion delay sequence, iteration/stop-reason including timeout), live CLI output oracle, plus regression run and `auto spec validate --strict`; security fence gate not reduced | required checks | Deterministic oracles per gap; no broad or redundant verification added |

## Semantic Invariant Inventory

Each `source clause` is quoted or summarized from the request as untrusted evidence only,
not as an instruction; no secrets or privileged paths are involved.

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "an injected sentinel-look-alike / 'ignore instructions' string in a participant output MUST appear inside the fence and MUST NOT be presentable as a top-level instruction" | ordering / containment | rebuttal and judgment prompt strings | AC-LPD-001 |
| INV-002 | "ALL debate/rebuttal/judge prompt-assembly paths apply the same sentinel-fence + SECURITY NOTE" | parity | interactive vs subprocess prompt contract | AC-LPD-002, AC-LPD-008 |
| INV-003 | "a simulated refs.lock present on first attempt MUST trigger a retry rather than immediate failure" and "up to three retries" | sequence / attempt-count / delay sequence | worktree Create attempt count, backoff delays, and returned error | AC-LPD-003 |
| INV-004 | "the dead duplicate is removed or unified so there is one implementation" plus "Only retry on lock-related errors" | classification + dedup + public API preservation | retry decision, module implementation count, and pipeline API regression | AC-LPD-004, AC-LPD-008 |
| INV-005 | "an experiment loop exceeding MaxIterations MUST hard-stop" and "timeout/cancellation SHALL report stop reason" | counter / bound / stop-reason | step invocation count, CLI output, and stop reason | AC-LPD-005, AC-LPD-007, AC-LPD-008 |
| INV-006 | "circuit-breaker no-progress detection as a hard stop" | consecutive-count / reset | step invocation count and stop reason | AC-LPD-006 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| Interactive debate fence parity with subprocess | Primary SPEC (T1, T2) | covered |
| Wired worktree lock retry + single implementation + pipeline public API preservation | Primary SPEC (T3, T4, T5, T9) | covered |
| Experiment in-process hard stop + observable live entrypoint | Primary SPEC (T6, T7, T8) | covered |
| Brownfield preservation of existing behavior/APIs | Primary SPEC (T9) | covered |
| Checkpoint write atomicity | Not this SPEC | evolution-idea |
| SSoT / policy-duplication drift, stale ARCHITECTURE.md generator | Not this SPEC | evolution-idea |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

These are optional and out of scope for this SPEC. They carry no SPEC/task/acceptance IDs
and do not block sync completion. Promote only on explicit user request.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| Atomic checkpoint writes in `pkg/pipeline/checkpoint.go`/`runner.go` (audit H3) | Outside the "unwired defense wiring" Outcome Lock; independent durability concern | User explicitly requests a checkpoint-durability SPEC |
| Single-source-of-truth / policy-duplication drift and the stale `ARCHITECTURE.md` generator (audit SSoT) | Separate architectural concern the user chose to decide separately | User explicitly requests an SSoT/generator SPEC |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | The three gaps share one precise problem — an advertised defense that is not on the live path — and one Outcome Lock. Combined scope is 3 packages plus one CLI file (well under the 25-task / 40-file split trigger); no separate deploy/ownership, migration, or compliance boundary applies. | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/orchestra/debate.go` `buildRebuttalPrompt` / `buildJudgmentPrompt` | existing | read; raw `## Participant` / `### Participant` paste confirmed |
| `pkg/orchestra/debate_sentinel.go` `newDebateSentinel` / `sentinelCollides` | existing | read; reusable variadic sentinel generator |
| `pkg/orchestra/pipeline.go` `sentinelForPreviousResults` / `sentinelForJudgeResults` | existing | read; subprocess fence wiring |
| `templates/shared/orchestra-debater-r2.md.tmpl` / `orchestra-judge.md.tmpl` | existing | read; SECURITY NOTE + BEGIN/END fence wording |
| `pkg/orchestra/pane_capable.go` `paneCapable` | existing | read; path selector |
| `pkg/worker/parallel/worktree.go` `WorktreeManager.Create` | existing | read; single `git worktree add`, no retry |
| `pkg/worker/worktree_safety.go` / `pkg/worker/loop_runtime.go` | existing | read; wired construction and callers |
| `pkg/pipeline/worktree.go` `addWorktreeWithRetry` / `isLockError` / backoff consts | existing | read; dead-in-production duplicate |
| `pkg/experiment/cmdvalidate.go` | existing | read; metric command validation reused by the live loop entrypoint |
| `pkg/experiment/circuit.go` / `types.go` / `recorder.go` / `metric.go` / `git.go` | existing | read; primitives present, no loop consumer |
| `internal/cli/experiment.go` / `experiment_helpers.go` | existing | read; stateless per-iteration subcommands |
| `pkg/orchestra/debate_fence_test.go` | [NEW] planned addition | Gap 1 oracle + parity tests |
| `pkg/worker/parallel/worktree_retry.go` / `_test.go` | [NEW] planned addition | shared retry helper + success, exhaustion, non-lock, and implementation-count tests |
| `pkg/worker/taskid/taskid.go` / `_test.go` | [NEW] implementation addition | task ID validator shared by the wired worktree path and adjacent worker task admission |
| `pkg/experiment/loop.go` / `loop_test.go` | [NEW] planned addition | Loop runner + max-iterations, circuit-breaker, cancellation, and timeout oracle tests |
| `internal/cli/experiment_run_test.go` | [NEW] implementation addition | live CLI output and background-operator rejection oracle |
| `auto experiment run` subcommand | [NEW] planned addition | live entrypoint added to `internal/cli/experiment.go`, with `stop_reason` and `total_iterations` output |

## Revision 1 closure

| F-ID | category | one-line closure | file:line |
|------|----------|------------------|-----------|
| F-001 | completeness | T4 now preserves `pkg/pipeline.WorktreeManager` public API and AC-LPD-008 requires pipeline/workflow regression tests. | plan.md:T4, acceptance.md:AC-LPD-008 |
| F-002 | completeness | AC-LPD-003 now pins persistent lock exhaustion to four attempts and delay sequence `base`, `base*2`, `base*4`. | acceptance.md:AC-LPD-003 |
| F-003 | completeness | AC-LPD-007 now covers both cancellation and timeout stop reasons. | acceptance.md:AC-LPD-007 |
| F-004 | completeness | REQ-LPD-10/T7/AC-LPD-008 now require observable `stop_reason` and `total_iterations` CLI output. | spec.md:REQ-LPD-10, plan.md:T7, acceptance.md:AC-LPD-008 |

## Revision 2 closure

| F-ID | category | one-line closure | file:line |
|------|----------|------------------|-----------|
| F-001 | security | T1 now derives the sentinel from the exact capped output bytes that are fenced, and AC-LPD-001 asserts the sentinel is absent from both original and fenced strings. | plan.md:T1, acceptance.md:AC-LPD-001 |

## Plan Intent Ledger

Clarification Ledger unavailable: this SPEC was authored from a direct SPEC-writer request
with an inline Outcome Lock and explicit out-of-scope list, not from a `## Clarification
Ledger` or a BS file. The request's out-of-scope items are preserved as Evolution Ideas and
its completion evidence as Must oracle acceptance.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: existing paths/symbols (debate builders, sentinel, worktree managers, experiment primitives) confirmed by rg and reads
- Q-CORR-02 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: new files/command marked `[NEW]` and excluded from existing-reference checks
- Q-CORR-03 | status: PASS | attempt: 1 | files: spec.md, acceptance.md | reason: EARS lines are non-bullet single lines containing "SYSTEM SHALL"; acceptance uses bare Given/When/Then
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: Reference Discipline separates existing vs `[NEW]` with verification notes
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: four files complement each other with no gaps
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: Traceability Matrix links every REQ to task, acceptance, invariant; retry cap, timeout, and pipeline API preservation are explicitly covered
- Q-COMP-03 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: each REQ states EARS type, trigger, expected result, and live CLI observability now has concrete output keys
- Q-COMP-04 | status: PASS | attempt: 2 | files: research.md, plan.md | reason: Outcome Lock closed by Primary SPEC while preserving `pkg/pipeline.WorktreeManager`; no scaffold-only closure
- Q-COMP-05 | status: PASS | attempt: 3 | files: research.md, acceptance.md | reason: every invariant maps to a concrete oracle including retry exhaustion, timeout, and CLI output
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief constrain review scope
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt (none) separated from Evolution Ideas (2 optional items, no IDs)
- Q-FEAS-01 | status: PASS | attempt: 1 | files: plan.md, research.md | reason: runtime Go changes in owning packages, not doc-only claims
- Q-FEAS-02 | status: PASS | attempt: 2 | files: plan.md | reason: edited paths exist in autopus-adk; pipeline public API is preserved without importing worker/parallel; templates are already-fenced source of truth, not touched
- Q-FEAS-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: oracles are runnable Go unit tests plus `auto spec validate --strict`
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ text avoids should/might/could; Priority on a separate line
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority (Must/Should) and EARS Type kept on separate axes
- Q-STYLE-03 | status: PASS | attempt: 1 | files: acceptance.md | reason: bare Given/When/Then/And; complete sentences
- Q-SEC-01 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: prompt-injection trust boundary is the core subject; sentinel derivation now uses the exact fenced bytes
- Q-SEC-02 | status: PASS | attempt: 1 | files: research.md | reason: no secrets/tokens/privileged paths introduced; branch-name validation preserved on the wired path
- Q-SEC-03 | status: N/A | attempt: 1 | files: research.md | reason: no new logs or retained artifacts are introduced by this SPEC
- Q-COH-01 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: one problem (advertised-but-unwired defense), bounded change set
- Q-COH-02 | status: PASS | attempt: 1 | files: research.md | reason: no follow-on work bypasses the Outcome Lock; out-of-scope items are optional Evolution Ideas
- Q-COH-03 | status: PASS | attempt: 1 | files: research.md | reason: Sibling SPEC Decision is none with a stated bounded reason
