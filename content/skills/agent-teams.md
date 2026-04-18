---
name: agent-teams
description: Role-based team composition skill for Claude Code Agent Teams mode
triggers:
  - agent teams
  - teams
  - 에이전트 팀
  - 팀 구성
category: agentic
level1_metadata: "Agent Teams, role-based, Lead-Builder-Guardian, SendMessage, worktree isolation"
---

# Agent Teams Skill

## Overview

Agent Teams mode (`--team`) enables role-based team collaboration via Claude Code Agent Teams. Instead of spawning ephemeral subagents per task, this mode creates persistent teammates that communicate directly, share a task list, and self-coordinate through the pipeline.

**Activation flag**: `/auto go SPEC-ID --team`

## Activation

### Prerequisites

| Requirement | Value | How to verify |
|-------------|-------|---------------|
| Claude Code version | v2.1.32 or later | `claude --version` |
| Environment variable | `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` | `echo $CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` |
| Feature status | **Experimental** — disabled by default | Official: https://code.claude.com/docs/en/agent-teams |

### Environment Setup

```bash
export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
```

The Autopus harness injects this variable via `.claude/settings.json` automatically.

### Failure Modes

If the variable is not set OR Claude Code is below v2.1.32, the pipeline MUST error with:

```
Error: Agent Teams mode unavailable
  Claude Code version: {detected} (required: v2.1.32+)
  CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS: {set|not set}
Fallback: Run without --team to use the subagent pipeline mode.
```

## Team Constraints (Official)

Agent Teams enforces these rules at the Claude Code layer — violating them fails at runtime:

| Constraint | Rule |
|-----------|------|
| Nested teams | Teammates MUST NOT call `TeamCreate` — only the top-level session can create a team |
| Cleanup authority | Only the Lead (team creator) may delete the team; teammates request cleanup via `SendMessage` |
| Recommended size | **3–5 teammates** per team (official guidance). Autopus default (Lead + 1–2 Builders + Guardian) fits this range |
| Persistence | Team config persists in `~/.claude/teams/{team-name}/config.json`; task list in `~/.claude/tasks/{team-name}/` |

## Team Roles

### Lead (1 agent)

**Responsibilities**: planner + reviewer

- Creates the team and assigns tasks via `SendMessage`
- Runs Phase 1 (Planning) to produce the execution plan
- Assigns tasks to Builder(s) and Guardian
- Monitors task list and consolidates results
- Runs Phase 4 (Review) and finalizes output
- Re-assigns or falls back to subagent if a teammate fails

### Builder (1–2 agents)

**Responsibilities**: executor + tester + annotator + frontend-specialist

- Implements code following TDD (RED → GREEN → REFACTOR)
- Writes tests in Phase 1.5 (Test Scaffold) before implementation
- Executes Phase 2 (Implementation) in an isolated worktree
- Applies `@AX` annotation tags in Phase 2.5 (Annotation)
- Communicates validation requests to Guardian via `SendMessage`
- Reports completion to Lead via `SendMessage`

### Guardian (1 agent)

**Responsibilities**: validator + security-auditor + perf-engineer

- Executes Gate 2 (Validation): coverage, lint, race conditions
- Performs security audit on modified files
- Monitors performance regressions
- Responds to partial validation requests from Builder
- Reports validation results to Lead via `SendMessage`

## Team Creation Pattern

Teammates are spawned with the standard `Agent()` tool plus `team_name` and `name` parameters. Each teammate loads its agent definition from `.claude/agents/autopus/` and inherits its frontmatter (tools, model, skills, permissionMode).

```python
# Lead creates the team at pipeline start
TeamCreate(team_name=f"team-{spec_id}")

# Spawn teammates via Agent() with team_name + name
Agent(subagent_type="planner",   team_name=f"team-{spec_id}", name="lead",     model="opus")
Agent(subagent_type="executor",  team_name=f"team-{spec_id}", name="builder-1")  # LOW complexity stays on sonnet
Agent(subagent_type="validator", team_name=f"team-{spec_id}", name="guardian")
```

The `name` field becomes the addressable handle for `SendMessage({to: "<name>"})`.

Task assignment via `SendMessage`:

```python
# Lead → Builder
SendMessage(to="builder", message={
    "phase": "Phase 2",
    "tasks": [...],
    "worktree": "<path>"
})

# Lead → Guardian
SendMessage(to="guardian", message={
    "phase": "Gate 2",
    "target_branch": "<branch>",
    "coverage_threshold": 85
})
```

## Execution Flow

```
Lead: Phase 1 (Planning)
  → Assigns tasks to Builder(s) and Guardian

Builder: Phase 1.5 (Test Scaffold)
  → Writes failing tests first (RED)

Builder: Phase 2 (Implementation)
  → GREEN phase in isolated worktree
  → Merge back after completion

Builder: Phase 2.5 (Annotation)
  → Applies @AX tags to modified files

Guardian: Gate 2 (Validation)
  → go test -race ./...
  → Coverage check (85%+)
  → golangci-lint run
  → Security audit

Lead: Phase 4 (Review)
  → Consolidates all results
  → Final quality check
  → Produces review report
```

## Builder-Guardian Direct Communication (P1-R3)

Builder can request partial validation from Guardian without waiting for Lead coordination:

```python
# Builder → Guardian (partial validation request)
SendMessage(to="guardian", message={
    "type": "partial_validation",
    "files": ["pkg/foo/bar.go"],
    "reason": "security-sensitive change"
})

# Guardian → Builder (validation result)
SendMessage(to="builder", message={
    "type": "validation_result",
    "status": "PASS",  # or FAIL
    "issues": []
})
```

All direct interactions are logged in the pipeline log:

```
[P1-R3] builder → guardian: partial_validation request (pkg/foo/bar.go)
[P1-R3] guardian → builder: PASS
```

## Subagent Fallback Strategy

| Scenario | Action |
|----------|--------|
| `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` not set | Error + fallback guidance to use subagent pipeline |
| Builder teammate fails mid-task | Lead re-assigns task to another Builder or spawns a subagent |
| Guardian teammate fails | Lead falls back to subagent validator with `Agent(subagent_type="validator")` |
| Team creation fails | Abort and fall back to default subagent pipeline |

## Worktree Isolation

The same worktree isolation rules (R1–R5 from `worktree-isolation.md`) apply in Agent Teams mode:

- Each Builder teammate works in an independent git worktree
- Maximum 5 simultaneous worktrees
- GC suppression: `git -c gc.auto=0 <command>` required during parallel execution
- Exponential backoff on shared resource lock contention (3s → 6s → 12s)
- Failed worktrees cleaned up with `git worktree remove --force <path>`

**Ref**: SPEC-WORKTREE-001, `@.claude/skills/autopus/worktree-isolation.md`
