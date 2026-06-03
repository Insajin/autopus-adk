package codex

func codexWorktreeIsolationSkillBody() string {
	return `
# Worktree Isolation Skill

Codex parallel work should be isolated by ownership first, and by workspace strategy second.

## Overview

The default isolation model in Codex is the worker's forked workspace created by ` + "`spawn_agent(...)`" + `. Do not assume an implicit worktree flag or automatic git worktree creation.

This skill defines when parallel work is safe and how the main session should integrate results.

## Default Isolation Model

- Each parallel worker gets a disjoint file ownership slice
- Each worker edits only its assigned slice in its own forked workspace
- The main session reviews and integrates returned changes
- Manual git worktrees are optional operator-managed workspaces, not the default Codex worker mechanism

If ownership cannot be separated cleanly, do not parallelize.

## Activation Conditions

Use this guidance when:

- ` + "`@auto go`" + ` is running in default pipeline or Codex ` + "`--team`" + ` mode
- planner marks tasks as parallel
- ownership rules are explicit and non-overlapping

Do not use parallel isolation when:

- tasks touch the same file
- tasks may create SQL migrations in the same owning repo and migration directory
- generated outputs must be serialized
- a task depends on a previous task's concrete output

## Ownership Validation

Before spawning parallel workers, compare ownership patterns:

1. Same literal file: conflict
2. Same directory with overlapping globs: conflict
3. Parent/child directory ownership: conflict
4. Same migration directory where both tasks may create SQL migrations: conflict
5. Different directories with no shared generated output or migration numbering lane: safe

On conflict, downgrade to sequential execution and log the reason.

## Parallel Worker Contract

Every parallel worker prompt should include:

- exact owned paths
- forbidden paths
- expected tests or checks
- required return fields: ` + "`owned_paths`" + `, ` + "`changed_files`" + `, ` + "`verification`" + `, ` + "`blockers`" + `, ` + "`next_required_step`" + `

Example:

` + "```python" + `
spawn_agent(
    agent_type="executor",
    fork_context=True,
    message="""
    Own only: pkg/auth/*, internal/auth/*
    Do not edit: pkg/api/*, migrations/*
    Return owned_paths, changed_files, verification, blockers, next_required_step.
    """,
)
` + "```" + `

## Integration Flow

After workers complete:

1. Review returned file lists and assumptions
2. Integrate changes in the main session
3. Run validation after integration, not before
4. If overlap or regressions appear, continue sequentially
5. Record whether manual git worktrees were used; absence of manual worktrees is normal for Codex spawned workers

## Optional Manual Git Worktree Path

For advanced multi-session workflows, the main session may still create explicit git worktrees with standard git commands. That is an operator choice, not an implicit Codex agent feature.

When using manual git worktrees:

- create them in the main session
- assign one worktree per ownership slice
- merge in a deterministic order
- remove worktrees after successful integration

## Safety Rules

- Prefer ownership separation over git complexity
- Keep validation workers read-only
- Stop parallel execution on merge conflicts or ownership ambiguity
- Never auto-resolve overlapping edits without review

## Multi-Session Guidance

When using multiple terminals or tmux panes:

- one session owns one concern slice
- keep branch names explicit
- merge in a known order after all sessions complete

If these constraints feel heavy for the task, use the default sequential pipeline instead.
`
}

func codexSubagentDevSkillBody() string {
	return `
# Subagent Development Skill

Guide for designing Codex worker roles and orchestrating them safely.

## Codex Primitives

Codex orchestration uses these primitives:

- ` + "`spawn_agent(...)`" + ` for new workers
- ` + "`send_input(...)`" + ` for follow-up instructions
- ` + "`wait_agent(...)`" + ` for synchronization
- ` + "`close_agent(...)`" + ` when a worker is no longer needed

Do not design around Claude-only team primitives or assumptions about direct worker-to-worker messaging.

## Agent Definitions

Harness reference agent definitions live under .codex/agents/. They document role scope, review posture, and expected outputs for roles such as ` + "`planner`" + `, ` + "`executor`" + `, ` + "`tester`" + `, and ` + "`validator`" + `.

Use those definitions as role contracts. The main session is still responsible for choosing the correct ` + "`agent_type`" + ` and passing explicit ownership.

## Design Principles

### Single Responsibility

Each worker should own one clear concern:

- implementation
- testing
- validation
- review

Avoid prompts that ask one worker to plan, implement, review, and secure the same slice.

### Ownership First

Every coding worker prompt should state:

- files or modules it owns
- files it must not edit
- completion criteria
- required return fields: ` + "`owned_paths`" + `, ` + "`changed_files`" + `, ` + "`verification`" + `, ` + "`blockers`" + `, ` + "`next_required_step`" + `

### Context Completeness

Workers do not share mutable session state automatically. Include the SPEC id, acceptance criteria, and any relevant constraints in the prompt or via ` + "`fork_context`" + `.

## Orchestration Patterns

### Fan-Out / Fan-In

Use for independent slices:

` + "```text" + `
main session -> worker A
             -> worker B
             -> worker C
             -> integrate results
` + "```" + `

### Pipeline

Use when each step depends on the previous result:

` + "```text" + `
planner -> executor -> validator -> reviewer
` + "```" + `

### Supervisor

Use the main session as supervisor:

- detect blockers
- respawn narrower workers
- decide when to fall back to sequential execution

## Practical Prompt Pattern

` + "```python" + `
spawn_agent(
    agent_type="executor",
    fork_context=True,
    message="""
    Own only: pkg/auth/*
    Goal: implement token refresh flow
    Tests: update auth service tests only
    Return: owned_paths, changed_files, verification, blockers, next_required_step
    """,
)
` + "```" + `

## Completion Checklist

- Role is narrow and concrete
- Ownership is explicit
- Validation path is assigned
- Retry/fallback behavior is defined
- Parallel workers have disjoint write scopes
`
}
