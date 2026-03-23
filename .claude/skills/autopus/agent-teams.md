---
name: agent-teams
description: Specialized agent-based team composition skill for Claude Code Agent Teams mode
triggers:
  - agent teams
  - teams
  - 에이전트 팀
  - 팀 구성
category: agentic
level1_metadata: "Agent Teams, specialized subagent_type, Phase-based spawning, SendMessage, worktree isolation"
---

# Agent Teams Skill

## Overview

Agent Teams mode (`--team`) enables specialized agent collaboration via Claude Code Agent Teams. Each teammate is spawned with a `subagent_type` that loads the corresponding agent definition from `.claude/agents/autopus/`, inheriting its tools, skills, model, and domain expertise. Teammates communicate directly via `SendMessage`, share a task list, and self-coordinate through the pipeline.

**Activation flag**: `/auto go SPEC-ID --team`

## Activation

Requires the experimental environment variable:

```bash
export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
```

If this variable is not set, the pipeline MUST error with:

```
Error: Agent Teams mode requires CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
Fallback: Run without --team to use the subagent pipeline mode.
```

## Team Composition

Each teammate maps 1:1 to a specialized agent definition. No role bundling.

### Core Members (always spawned)

| Name | subagent_type | Agent Definition | Phase |
|------|---------------|------------------|-------|
| `lead` | `planner` | planner.md (tools: Read,Grep,Glob,Bash,WebSearch,WebFetch,sequentialthinking) | Phase 1, Gate 1, coordination |
| `builder-1` | `executor` | executor.md (tools: Read,Write,Edit,Grep,Glob,Bash,TodoWrite) | Phase 2 |
| `tester` | `tester` | tester.md (tools: Read,Write,Edit,Grep,Glob,Bash) | Phase 1.5, Phase 3 |
| `guardian` | `validator` | validator.md (tools: Read,Grep,Glob,Bash) | Gate 2 |

### Phase-Dependent Members (spawned when needed)

| Name | subagent_type | Agent Definition | Condition |
|------|---------------|------------------|-----------|
| `builder-2` | `executor` | executor.md | Parallel tasks exist (2+ independent tasks) |
| `annotator` | `annotator` | annotator.md | Gate 2 PASS → Phase 2.5 |
| `auditor` | `security-auditor` | security-auditor.md | Phase 4 (parallel with reviewer) |
| `reviewer` | `reviewer` | reviewer.md | Phase 4 (parallel with auditor) |
| `ux-verifier` | `frontend-specialist` | frontend-specialist.md | Phase 3.5, only if .tsx/.jsx changed |

### Responsibilities per Member

**lead** (planner):
- Creates the team and task list
- Runs Phase 1 (Planning) to produce the execution plan
- Assigns tasks to teammates via `SendMessage`
- Monitors progress and consolidates results
- Handles Gate 1 (Approval) coordination
- Acts as Consolidator for Phase 4 review verdicts

**builder-1/2** (executor):
- Implements code following TDD GREEN → REFACTOR
- Works in an isolated worktree for parallel tasks
- Reports completion to lead via `SendMessage`

**tester** (tester):
- Phase 1.5: Creates failing test skeletons (RED state)
- Phase 3: Raises coverage to 85%+, adds edge case tests

**guardian** (validator):
- Gate 2: Runs go build, go test -race, golangci-lint, go vet
- Checks file size limits (300 lines)
- Reports Gate Verdict to lead

**annotator** (annotator):
- Phase 2.5: Applies @AX tags to modified files
- Validates per-file limits (ANCHOR ≤ 3, WARN ≤ 5)

**auditor** (security-auditor):
- Phase 4: OWASP Top 10 security audit on changed files
- Reports Verdict to lead in parallel with reviewer

**reviewer** (reviewer):
- Phase 4: TRUST 5 code review on changed files
- Reports Verdict to lead in parallel with auditor

## Team Creation Pattern

```python
# Step 1: Create team
TeamCreate(team_name="team-{SPEC_ID}")

# Step 2: Spawn core members
# model parameter follows Quality Mode rules (see below)

# Lead — always first
Agent(
    subagent_type="planner",
    team_name="team-{SPEC_ID}",
    name="lead",
    model="opus",          # Ultra: "opus", Balanced: omit (frontmatter default: opus)
    prompt="You are the team lead for SPEC-{SPEC_ID}. Read the SPEC, decompose tasks, and coordinate the team."
)

# Tester — Phase 1.5 (Test Scaffold)
Agent(
    subagent_type="tester",
    team_name="team-{SPEC_ID}",
    name="tester",
    prompt="Create failing test skeletons for SPEC-{SPEC_ID} P0/P1 requirements."
)

# Builder(s) — Phase 2 (Implementation)
Agent(
    subagent_type="executor",
    team_name="team-{SPEC_ID}",
    name="builder-1",
    isolation="worktree",  # Parallel tasks get worktree isolation
    prompt="Implement tasks assigned by lead. SPEC: .autopus/specs/SPEC-{SPEC_ID}/spec.md"
)

# Guardian — Gate 2 (Validation)
Agent(
    subagent_type="validator",
    team_name="team-{SPEC_ID}",
    name="guardian",
    prompt="Validate implementation: build, test, lint, coverage, file size."
)
```

### Phase 4: Spawn Review Members

```python
# Reviewer and Auditor spawned in parallel for Phase 4
Agent(
    subagent_type="reviewer",
    team_name="team-{SPEC_ID}",
    name="reviewer",
    prompt="Review all changes using TRUST 5 criteria. Report verdict to lead."
)
Agent(
    subagent_type="security-auditor",
    team_name="team-{SPEC_ID}",
    name="auditor",
    prompt="Audit all changed files for security vulnerabilities. Report verdict to lead."
)
```

## Quality Mode × Model Selection

Quality Mode determines the `model` parameter when spawning each teammate.

### Ultra Mode

Add `model: "opus"` to ALL Agent() calls:

```python
Agent(subagent_type="executor", team_name="...", name="builder-1", model="opus")
Agent(subagent_type="validator", team_name="...", name="guardian",  model="opus")
# ... all opus
```

### Balanced Mode

Omit `model` parameter → each agent uses its frontmatter default:

| Teammate | subagent_type | Frontmatter Model | Rationale |
|----------|---------------|-------------------|-----------|
| lead | planner | opus | Planning requires deep reasoning |
| builder-1/2 | executor | sonnet | Implementation: speed/cost balance |
| tester | tester | sonnet | Test writing |
| guardian | validator | haiku | Build/lint checks are lightweight |
| annotator | annotator | sonnet | Tag analysis |
| auditor | security-auditor | opus | Security requires deep reasoning |
| reviewer | reviewer | sonnet | Code review |

### Balanced + Adaptive Quality

Per-task complexity overrides the model for builder agents:

| Complexity | Model Override |
|-----------|---------------|
| HIGH | `model: "opus"` |
| MEDIUM | omit (sonnet default) |
| LOW | `model: "haiku"` |

```python
# HIGH complexity task
Agent(subagent_type="executor", team_name="...", name="builder-1", model="opus")
# LOW complexity task
Agent(subagent_type="executor", team_name="...", name="builder-2", model="haiku")
```

## Execution Flow

```
Main Session:
  TeamCreate("team-{SPEC_ID}")

lead (planner): Phase 1 — Planning
  → Decomposes SPEC into tasks
  → Creates task list, assigns to teammates

tester (tester): Phase 1.5 — Test Scaffold
  → Writes failing tests (RED state)

builder-1 (executor): Phase 2 — Implementation
  → GREEN phase in isolated worktree
builder-2 (executor): Phase 2 — Implementation (parallel, if needed)
  → GREEN phase in isolated worktree

Main Session: Phase 2.1 — Worktree Merge
  → Merges worktree branches in task-ID order

guardian (validator): Gate 2 — Validation
  → Build, test, lint, coverage, file size checks

annotator (annotator): Phase 2.5 — Annotation
  → Applies @AX tags to modified files

tester (tester): Phase 3 — Testing
  → Raises coverage to 85%+

reviewer (reviewer) + auditor (security-auditor): Phase 4 — Review (parallel)
  → TRUST 5 review + OWASP security audit

lead (planner): Consolidation
  → Merges review verdicts
  → Final pipeline result
```

## Direct Communication Patterns

### Builder → Guardian (Partial Validation)

Builder can request partial validation from guardian without waiting for lead:

```python
# builder-1 → guardian
SendMessage(to="guardian", message="Please validate pkg/foo/bar.go — security-sensitive change, pre-check before Gate 2")

# guardian → builder-1
SendMessage(to="builder-1", message="Validation PASS for pkg/foo/bar.go. No issues found.")
```

### Lead → Reviewer/Auditor (Consolidated Review)

```python
# After reviewer and auditor report verdicts
# lead consolidates and sends unified fix list to builder
SendMessage(to="builder-1", message="""
Consolidated review — REQUEST_CHANGES:
1. [HIGH] SQL injection risk in pkg/db/query.go:42 (from auditor)
2. [MEDIUM] Missing error context in pkg/auth/token.go:15 (from reviewer)
Fix these issues and report back.
""")
```

## Subagent Fallback Strategy

| Scenario | Action |
|----------|--------|
| `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` not set | Error + fallback to subagent pipeline |
| Builder fails mid-task | Lead re-assigns to another builder or spawns `Agent(subagent_type="executor")` |
| Guardian fails | Lead spawns `Agent(subagent_type="validator")` as fallback |
| Reviewer/Auditor fails | Lead spawns corresponding subagent as fallback |
| Team creation fails | Abort and fall back to default subagent pipeline |

## Worktree Isolation

The same worktree isolation rules (R1–R5 from `worktree-isolation.md`) apply:

- Each builder teammate works in an independent git worktree
- Maximum 5 simultaneous worktrees
- GC suppression: `git -c gc.auto=0 <command>` required during parallel execution
- Exponential backoff on shared resource lock contention (3s → 6s → 12s)
- Failed worktrees cleaned up with `git worktree remove --force <path>`

**Ref**: SPEC-WORKTREE-001, `@.claude/skills/autopus/worktree-isolation.md`

## Shutdown Protocol

After pipeline completion, lead sends shutdown requests to all active teammates:

```python
SendMessage(to="builder-1", message={"type": "shutdown_request", "reason": "Pipeline complete"})
SendMessage(to="tester",    message={"type": "shutdown_request", "reason": "Pipeline complete"})
SendMessage(to="guardian",  message={"type": "shutdown_request", "reason": "Pipeline complete"})
# ... for all active teammates
```
