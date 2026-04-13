package codex

import "strings"

func normalizeCodexExtendedSkill(name, body string) string {
	switch name {
	case "agent-teams":
		return strings.TrimSpace(codexAgentTeamsSkillBody()) + "\n"
	case "agent-pipeline":
		return strings.TrimSpace(codexAgentPipelineSkillBody()) + "\n"
	case "worktree-isolation":
		return strings.TrimSpace(codexWorktreeIsolationSkillBody()) + "\n"
	case "subagent-dev":
		return strings.TrimSpace(codexSubagentDevSkillBody()) + "\n"
	case "prd":
		return strings.TrimSpace(rewriteCodexPRDSkillBody(body)) + "\n"
	default:
		return body
	}
}

func rewriteCodexPRDSkillBody(body string) string {
	body = strings.ReplaceAll(
		body,
		"PRD 작성 전에 6개 핵심 질문으로 컨텍스트를 수집합니다. 사용자 입력이 불충분할 경우 AskUserQuestion으로 확인:",
		"PRD 작성 전에 6개 핵심 질문으로 컨텍스트를 수집합니다. 사용자 입력이 불충분하면 메인 세션이 짧은 plain-text 질문으로 직접 확인합니다:",
	)
	body = strings.ReplaceAll(body, "AskUserQuestion", "a short plain-text question")
	return body
}

func codexAgentTeamsSkillBody() string {
	return `
# Agent Teams Skill

## Overview

In Codex, ` + "`--team`" + ` is an orchestration pattern, not a native Team API. The main session acts as the lead coordinator and manages parallel workers with ` + "`spawn_agent(...)`" + `, ` + "`send_input(...)`" + `, and ` + "`wait_agent(...)`" + `.

**Activation flag**: ` + "`@auto go SPEC-ID --team`" + `

## Activation

Use this mode only when the work can be partitioned into disjoint ownership slices.

- Good fit: multiple packages, clear ownership boundaries, independent validation tasks
- Poor fit: one-file changes, tightly coupled refactors, debugging a single failing path

If decomposition is unclear, fall back to the default pipeline or ` + "`--solo`" + `.

## Role Model

### Lead (main session)

- Loads the SPEC and decides the split strategy
- Spawns workers with explicit ownership and completion criteria
- Tracks progress, merges findings, and resolves conflicts
- Runs the final integration step after worker results return

### Builder workers

- Usually ` + "`executor`" + ` or ` + "`tester`" + `
- Own a disjoint file set or concern slice
- Implement RED → GREEN → REFACTOR inside their forked workspace
- Return changed file paths, tests run, and unresolved blockers

### Guardian workers

- Usually ` + "`validator`" + `, ` + "`reviewer`" + `, ` + "`security-auditor`" + `
- Review builder output without sharing mutable state
- Report PASS / FAIL or APPROVE / REQUEST_CHANGES with actionable evidence

## Coordination Pattern

Use the main session as the message bus:

` + "```python" + `
builder = spawn_agent(
    agent_type="executor",
    fork_context=True,
    message="""
    Own only: pkg/auth/*, internal/auth/*
    Do not edit tests outside pkg/auth/.
    Return changed files, tests run, and blockers.
    """,
)

guardian = spawn_agent(
    agent_type="validator",
    fork_context=True,
    message="""
    Read-only review for pkg/auth/* after implementation lands.
    Report Verdict, Issues, and missing tests.
    """,
)

wait_agent(targets=[builder], timeout_ms=180000)
send_input(
    target=guardian,
    message="Builder completed. Validate the auth slice and focus on regressions.",
)
wait_agent(targets=[guardian], timeout_ms=180000)
` + "```" + `

## Partial Validation

Builders can request focused validation through the main session:

1. Builder returns a checkpoint or blocker.
2. Main session forwards a narrow ask to a validator/reviewer with ` + "`send_input(...)`" + `.
3. Guardian responds with issues scoped to the owned slice.
4. Main session decides whether to respawn the builder or continue.

This preserves Codex's actual control flow without pretending workers can directly message each other.

## Failure Handling

| Scenario | Codex handling |
|----------|----------------|
| Builder blocked | Respawn a narrowed worker or handle the blocker in the main session |
| Validator disagrees with builder | Main session consolidates findings and issues a focused remediation task |
| Ownership overlap detected | Cancel team mode for that slice and rerun sequentially |
| Too much coordination overhead | Fall back to default pipeline |

## Isolation Rules

Team mode in Codex still depends on strict ownership isolation:

- One worker, one write scope
- Shared files move back to the main session or sequential execution
- Validation workers remain read-only

For deeper guidance on parallel file ownership and branch hygiene, see .codex/skills/worktree-isolation.md.
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
| ` + "`--team`" + ` | Parallel team pattern | Main session coordinates multiple workers in parallel |
| ` + "`--solo`" + ` | Single session | No worker spawning; implement directly in the main session |
| ` + "`--multi`" + ` | Multi-provider review | Run additional review/validation passes when configured |

See .codex/skills/agent-teams.md for the Codex interpretation of ` + "`--team`" + ` and .codex/skills/worktree-isolation.md for parallel ownership rules.

## Phase 0.5: Autonomy Policy

Before spawning workers, decide whether the pipeline can proceed autonomously:

- If ` + "`--auto`" + ` is set, continue without confirmation.
- If user intent is ambiguous, ask one concise plain-text question in the main session.
- Do not rely on Claude-only permission or question APIs.

## Pipeline Overview

` + "```text" + `
Phase 1:   Planning        -> planner
Phase 1.5: Test Scaffold   -> tester        (optional)
Gate 1:    Approval        -> main session  (skip with --auto)
Phase 1.8: Doc Fetch       -> main session  (fetch current docs if needed)
Phase 2:   Implementation  -> executor x N  (parallel only with disjoint ownership)
Gate 2:    Validation      -> validator
Phase 2.5: Annotation      -> annotator
Phase 3:   Testing         -> tester
Phase 3.5: UX Verify       -> frontend-specialist (optional)
Phase 4:   Review          -> reviewer + security-auditor
` + "```" + `

## Quality Mode

Quality mode influences model choice, not platform semantics:

- Ultra: pass ` + "`model=\"opus\"`" + ` to spawned workers
- Balanced: use each role's default model
- Adaptive: choose stronger models only for high-complexity tasks

Reference: .codex/skills/adaptive-quality.md

## Phase Guidance

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

### Phase 1.5: Test Scaffold

When enabled, spawn a tester to write failing tests before implementation. Generated scaffold tests are read-only for later executors unless the plan explicitly reassigns them.

### Phase 1.8: Doc Fetch

This phase stays in the main session. Use current documentation tools available in the session and inject only the relevant excerpts into later worker prompts.

### Phase 2: Implementation

Parallel implementation is valid only with disjoint ownership.

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

### Gate 2: Validation

Spawn a validator after implementation lands. If validation fails, respawn a focused fixer instead of rerunning the full pipeline blindly.

### Phase 2.5: Annotation

Run annotator after validation PASS. Harness-only markdown changes may skip this phase.

### Phase 3 / 3.5: Testing and UX Verification

- Tester raises coverage and adds edge-case tests
- Frontend-specialist runs only when changed files include frontend UI

### Phase 4: Review

Run reviewer and security-auditor in parallel when the change scope justifies both. Consolidate findings in the main session.

## Parallelism Rules

| Condition | Execution |
|----------|-----------|
| Non-overlapping ownership | Parallel workers allowed |
| Shared file or shared migration | Sequential execution |
| Order dependency between tasks | Sequential execution |
| One worker blocked on another's output | Wait, integrate, then continue |

## Retry Policy

- Validation: up to 3 retries, or 5 with ` + "`--loop`" + `
- Review: up to 2 retries, or 3 with ` + "`--loop`" + `
- Repeated worker failure: shrink scope or fall back to the main session

## Result Integration

Each worker should return:

- changed files
- verification run
- blockers or assumptions

The main session owns final integration, status updates, and the decision to continue, retry, or stop.
`
}

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

If ownership cannot be separated cleanly, do not parallelize.

## Activation Conditions

Use this guidance when:

- ` + "`@auto go`" + ` is running in default pipeline or Codex ` + "`--team`" + ` mode
- planner marks tasks as parallel
- ownership rules are explicit and non-overlapping

Do not use parallel isolation when:

- tasks touch the same file
- migrations or generated outputs must be serialized
- a task depends on a previous task's concrete output

## Ownership Validation

Before spawning parallel workers, compare ownership patterns:

1. Same literal file: conflict
2. Same directory with overlapping globs: conflict
3. Parent/child directory ownership: conflict
4. Different directories with no shared generated output: safe

On conflict, downgrade to sequential execution and log the reason.

## Parallel Worker Contract

Every parallel worker prompt should include:

- exact owned paths
- forbidden paths
- expected tests or checks
- required return format

Example:

` + "```python" + `
spawn_agent(
    agent_type="executor",
    fork_context=True,
    message="""
    Own only: pkg/auth/*, internal/auth/*
    Do not edit: pkg/api/*, migrations/*
    Return changed files and tests run.
    """,
)
` + "```" + `

## Integration Flow

After workers complete:

1. Review returned file lists and assumptions
2. Integrate changes in the main session
3. Run validation after integration, not before
4. If overlap or regressions appear, continue sequentially

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
- expected return format

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
    Return: changed files, tests run, unresolved blockers
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
