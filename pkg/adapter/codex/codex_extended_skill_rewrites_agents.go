package codex

func codexAgentTeamsSkillBody() string {
	return `
# Agent Teams Skill

## Overview

This document is a reserved placeholder for a future native Codex multi-agent / teams surface.

**Activation flag**: ` + "`@auto go SPEC-ID --team`" + `

Today, Codex should continue to use the default ` + "`spawn_agent(...)`" + ` subagent pipeline. Do not reinterpret ` + "`--team`" + ` as extra parallel orchestration in the harness.

## Current Behavior

- ` + "`@auto go`" + ` without flags: use the default subagent pipeline, but if runtime policy blocks implicit spawning, ask before proceeding
- ` + "`@auto go --auto`" + `: treat this as explicit approval to run the default subagent pipeline without an extra confirmation round
- ` + "`@auto go --solo`" + `: disable subagents and stay in the main session
- ` + "`@auto go --team`" + `: keep compatibility with future native multi-agent naming, but continue with the default subagent pipeline for now

## Why This Is Reserved

- Codex already supports subagents natively via ` + "`spawn_agent(...)`" + `
- Public Codex docs do not define a separate local CLI Team API equivalent to Claude Code Agent Teams
- Overloading ` + "`--team`" + ` to mean "extra ` + "`spawn_agent(...)`" + ` fan-out" would conflict with the likely future meaning of native multi-agent support

## What To Use Instead

- Use ` + "`.codex/skills/agent-pipeline.md`" + ` for the default execution model
- Use ` + "`.codex/agents/*.toml`" + ` as the role source of truth for spawned workers
- Use ` + "`.codex/skills/worktree-isolation.md`" + ` when parallel ownership boundaries are explicit

## Revisit Condition

Enable a real ` + "`--team`" + ` route only when Codex exposes a documented native multi-agent surface that is distinct from ordinary subagent spawning.
`
}

func codexAgentPipelineSkillBody() string {
	return `
# Agent Pipeline Skill

Default multi-agent execution model for ` + "`@auto go`" + ` in Codex.

## Activation

This skill is the default for ` + "`@auto go SPEC-ID`" + `.

| Flag | Mode | Codex meaning |
|------|------|---------------|
| none | Subagent pipeline | Main session orchestrates specialists phase-by-phase |
| ` + "`--team`" + ` | Reserved compatibility flag | Keep the default subagent pipeline until Codex ships a documented native multi-agent surface |
| ` + "`--solo`" + ` | Single session | No worker spawning; implement directly in the main session |
| ` + "`--multi`" + ` | Multi-provider review | Run additional review/validation passes when configured, prefer orchestra-backed review when available |

See .codex/skills/agent-teams.md for the reserved ` + "`--team`" + ` policy and .codex/skills/worktree-isolation.md for parallel ownership rules.

## Codex Auto Semantics

- In Codex, ` + "`--auto`" + ` means "skip approval gates" and also counts as explicit approval for the default ` + "`spawn_agent(...)`" + ` subagent pipeline.
- Without ` + "`--auto`" + `, if the runtime policy blocks implicit worker spawning, the main session must explain the constraint and ask before switching to subagents.
- ` + "`--team`" + ` remains a reserved compatibility flag until Codex ships a distinct native multi-agent surface.

## Supervisor Contract

Treat the main session as a state-machine supervisor, not a passive router.

- Parse flags first.
- Decide whether the run is read-heavy or write-heavy.
- Spawn only workers with explicit ownership.
- Keep the main thread focused on requirements, decisions, and final synthesis.
- Do not treat subagents as autonomous teammates that negotiate among themselves.

## Phase 0.5: Autonomy Policy

Before spawning workers, decide whether the pipeline can proceed autonomously:

- If ` + "`--auto`" + ` is set, continue without confirmation and treat it as explicit approval for the default subagent pipeline.
- If user intent is ambiguous, ask one concise plain-text question in the main session.
- Do not rely on Claude-only permission or question APIs.
- If the task is write-heavy and ownership is unclear, stop parallel fan-out and resolve ownership first.

## Pipeline Overview

` + "```text" + `
Step 0:    Triage          -> main session
Phase 1:   Planning        -> planner
Phase 1.5: Test Scaffold   -> tester        (optional)
Gate 1:    Approval        -> main session  (skip with --auto)
Phase 1.8: Doc Fetch       -> main session  (fetch current docs if needed)
Phase 2:   Implementation  -> executor x N  (parallel only with disjoint ownership)
Gate 2:    Validation      -> validator
Phase 2.5: Annotation      -> annotator
Phase 3:   Testing         -> tester
Phase 3.5: UX Verify       -> frontend-specialist (optional)
Phase 4A:  Review          -> reviewer + security-auditor (+ optional multi-provider discovery)
Phase 4B:  Verify Fixes    -> reviewer/security follow-up, diff-only
` + "```" + `

## Quality Mode

Quality mode influences model choice, not platform semantics:

- Ultra: pass ` + "`model=\"opus\"`" + ` to spawned workers
- Balanced: use each role's default model
- Adaptive: choose stronger models only for high-complexity tasks

Reference: .codex/skills/adaptive-quality.md

## Triage Pattern

Use this decision rule before you fan out:

| Task Shape | Preferred Pattern |
|-----------|-------------------|
| Read-heavy exploration, triage, review, docs verification | Parallel fan-out / fan-in |
| Write-heavy implementation with disjoint ownership | Parallel workers with strict ownership |
| Shared file, migration, or dependency chain | Sequential pipeline |
| Ambiguous scope or unclear ownership | Main session resolves scope first |

## Phase Guidance

### Step 0: Triage

Before Phase 1, classify the work:

- Read-heavy: prefer ` + "`explorer`" + `, ` + "`reviewer`" + `, docs/research workers in parallel.
- Write-heavy: require explicit owned paths before spawning executors.
- Mixed: do read-heavy mapping first, then implement sequentially or with guarded parallelism.

### Phase 1: Planning

Spawn a planner when the task has enough scope to justify decomposition.

` + "```python" + `
spawn_agent(
    agent_type="planner",
    fork_context=True,
    message="""
    Read SPEC-XXX.
    Produce an execution table with task id, owner role, mode, and file ownership.
    Mark only truly independent tasks as parallel.
    """,
)
` + "```" + `

Planner output should make the next action obvious. Require:

- task id
- owner role
- owned paths
- dependencies
- execution mode
- next required step

### Worker Prompt Contract

Every spawned worker should receive a prompt that states:

- exact owned paths or modules
- files it must not edit
- completion criteria
- expected verification
- required return fields

Recommended return fields:

- ` + "`owned_paths`" + `
- ` + "`changed_files`" + `
- ` + "`verification`" + `
- ` + "`blockers`" + `
- ` + "`next_required_step`" + `

### Phase 1.5: Test Scaffold

When enabled, spawn a tester to write failing tests before implementation. Generated scaffold tests are read-only for later executors unless the plan explicitly reassigns them.

### Phase 1.8: Doc Fetch

This phase stays in the main session. Try Context7 MCP first for external libraries, then fall back to targeted web search when Context7 is unavailable, returns no match, or the docs query is incomplete. Inject only the relevant excerpts into later worker prompts.

Doc-fetch rules:

- detect external libraries from the task, affected imports, and config files
- prefer Context7 MCP as the first source for current API and migration guidance
- if Context7 fails, use web search with official docs, release notes, and API references first
- cache only the minimum relevant excerpts under a ` + "`## Reference Documentation`" + ` section
- do not block implementation when both Context7 and web fallback fail

### Phase 2: Implementation

Parallel implementation is valid only with disjoint ownership. Prefer narrow workers over broad ones.

` + "```python" + `
spawn_agent(
    agent_type="executor",
    fork_context=True,
    message="""
    Own only: pkg/auth/*.
    Follow TDD for task T1.
    Return changed files, tests run, and unresolved issues.
    """,
)
` + "```" + `

When workers return, review and integrate their results in the main session. Do not assume Codex auto-merges worktree branches.

Write-heavy rules:

- Parallel writes are allowed only when owned paths do not overlap.
- If two workers touch the same file or migration chain, switch to sequential execution.
- If a worker returns blockers that change scope, re-plan before spawning more writers.

### Gate 2: Validation

Spawn a validator after implementation lands. If validation fails, respawn a focused fixer instead of rerunning the full pipeline blindly.

Validation retries should be narrow:

- retry the smallest failing slice
- keep the same acceptance target
- stop after the retry budget and surface the blocker

### Phase 2.5: Annotation

Run annotator after validation PASS. Harness-only markdown changes may skip this phase.

### Phase 3 / 3.5: Testing and UX Verification

- Tester raises coverage and adds edge-case tests
- Frontend-specialist runs only when changed files include frontend UI

### Phase 4A: Review Discovery

Run reviewer and security-auditor in parallel when the change scope justifies both. When ` + "`--multi`" + ` is set, concentrate the extra multi-provider pass here, during discovery, not during every retry.

Discovery output should be frozen into a checklist of open findings with:

- finding id
- category
- file references
- required fix or proof of non-issue

If discovery returns actionable findings and the review retry budget is not exhausted yet:

- convert the frozen checklist into the next focused executor task immediately
- do not ask the user to manually fix, rerun, or confirm while the next repair step is still actionable inside the current ` + "`@auto go`" + ` invocation
- keep the checklist stable unless the patch meaningfully changes scope

### Phase 4B: Review Verification

After fixes land, do a diff-only verification pass against the frozen checklist.

- Verify whether each open finding is resolved.
- Do not restart full discovery unless the patch meaningfully changed scope.
- Stop review retries when the same unresolved finding repeats without material code change.
- If findings remain actionable and retry budget remains, loop back to a focused fixer, then re-run validation/testing only for the touched scope before returning here.
- Under ` + "`--auto --loop`" + `, keep this repair -> validate -> verify cycle inside the same session. Only stop for a real blocker, exhausted retry budget, or circuit break.

## Parallelism Rules

| Condition | Execution |
|----------|-----------|
| Non-overlapping ownership | Parallel workers allowed |
| Shared file or shared migration | Sequential execution |
| Order dependency between tasks | Sequential execution |
| One worker blocked on another's output | Wait, integrate, then continue |

## Retry Policy

- Validation: up to 3 retries, or 5 with ` + "`--loop`" + `
- Review verification: up to 2 retries, or 3 with ` + "`--loop`" + `
- Repeated worker failure: shrink scope or fall back to the main session
- Repeated unchanged review finding: stop and surface the blocker instead of rediscovering the whole patch
- While review retries remain, unresolved findings are not a terminal handoff. Do not suggest ` + "`@auto go --continue`" + ` or manual review yet.

## Result Integration

Each worker should return:

- owned paths
- changed files
- verification run
- blockers or assumptions
- next required step

The main session owns final integration, status updates, and the decision to continue, retry, or stop.

## Pre-Completion Checklist

Before you stop, ensure:

- the next required step is either complete or explicitly blocked
- validation status is known
- open review findings are either resolved or explicitly carried forward
- terminal handoff is used only after the final review outcome is known
- the final response names the changed scope, verification, and any unresolved blockers
`
}
