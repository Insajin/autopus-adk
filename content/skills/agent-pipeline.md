---
name: agent-pipeline
description: Multi-agent pipeline orchestration skill
triggers:
  - pipeline
  - multi-agent
  - 파이프라인
  - 멀티에이전트
category: agentic
level1_metadata: "5-Phase pipeline, automatic agent delegation, quality gates"
---

# Agent Pipeline Skill

A 5-Phase multi-agent pipeline orchestration skill. This is the **default** execution mode for `/auto go`.

## Activation

This skill is the default for `/auto go SPEC-ID`.

| 플래그 | 모드 | 설명 |
|--------|------|------|
| (없음) | **서브에이전트 파이프라인** | Agent tool로 서브에이전트 스폰 (이 스킬). 메인 세션이 파이프라인 흐름 제어 |
| `--team` | **Agent Teams** | Claude Code Agent Teams 사용. 팀원 간 직접 통신, 공유 태스크 리스트, 자기 조율 |
| `--solo` | **단일 세션** | 메인 세션이 직접 TDD 구현. 서브에이전트 없음 |
| `--multi` | **멀티프로바이더** | Review Phase에서 orchestra engine 사용. 다른 모드와 조합 가능 |

@.claude/skills/autopus/worktree-isolation.md

## Pipeline Overview

```
Phase 1: Planning        → planner     (model: depends on quality mode, plan)
Gate 1:  Approval        → skipped if --auto
Phase 2: Implementation  → executor×N  (sonnet, acceptEdits, parallel with worktree isolation)
Phase 2.1: Worktree Merge → main session (merge worktree branches into working branch)
Gate 2:  Validation      → validator   (haiku,  plan)  — retry up to 3x on FAIL
Phase 2.5: Annotation    → executor    (sonnet, bypassPermissions) — @AX tags on modified files
Phase 3: Testing         → tester      (sonnet, acceptEdits)
Gate 3:  Coverage        → verify 85%+ coverage
Phase 3.5: UX Verify    → ux-validator (sonnet, plan)  — optional, frontend only
Phase 4: Review          → reviewer    (sonnet, plan)  — retry up to 2x on REQUEST_CHANGES
```

> The model assignments above are for Balanced mode. In Ultra mode, all agents run with opus.

## Quality Mode

The quality mode — determined by the `--quality` flag or interactive selection — controls the `model` parameter in Agent() calls.

### Ultra Mode

Add `model: "opus"` parameter to all Agent() calls:

```
Agent(
  subagent_type = "executor",
  model = "opus",
  prompt = "..."
)
```

### Balanced Mode

Omit the `model` parameter in Agent() calls to use each agent definition's frontmatter model:

```
Agent(
  subagent_type = "executor",
  prompt = "..."
)
```

### Agents Not in Preset

If an agent is not defined in the selected preset, omit the `model` parameter (use frontmatter default).

## Agent Spawning per Phase

### Phase 1: Planning

```
Agent(
  subagent_type = "planner",
  prompt = """
    Load the SPEC file and decompose tasks.
    Return an agent assignment table:
    | Task ID | Agent    | Mode       | File Ownership  |
    |---------|----------|------------|-----------------|
    | T1      | executor | parallel   | *.go            |
    | T2      | executor | parallel   | *_test.go       |
  """
)
```

### Phase 2: Implementation

Tasks that can run in parallel are spawned with multiple Agent() calls in a single message.

Parallel tasks use `isolation: "worktree"` so each executor works in an independent git worktree (R1). Max 5 concurrent worktrees; overflow tasks are queued.

```
# Parallel execution example — with worktree isolation
# Ultra: add model="opus", Balanced: omit model
Agent(subagent_type="executor", prompt="Implement T1: ...", permissionMode="bypassPermissions", isolation="worktree")  # Balanced
Agent(subagent_type="executor", model="opus", prompt="Implement T1: ...", permissionMode="bypassPermissions", isolation="worktree")  # Ultra
Agent(subagent_type="executor", prompt="Implement T2: ...", permissionMode="bypassPermissions", isolation="worktree")  # Balanced
Agent(subagent_type="executor", model="opus", prompt="Implement T2: ...", permissionMode="bypassPermissions", isolation="worktree")  # Ultra
```

Collect `worktree_path` and `branch` from each return value for Phase 2.1 merge.

Sequential tasks do NOT use `isolation: "worktree"` and merge immediately after completion before the next dependent task is spawned (R3).

```
# Sequential execution example — immediate merge after each task
result_t1 = Agent(subagent_type="executor", prompt="Implement T1: ...")
# merge T1 worktree branch immediately (if isolation was used), then spawn T2
Agent(subagent_type="executor", prompt="Implement T2. T1 result: {result_t1}")
```

### Phase 2.1: Worktree Merge

WHEN all parallel executors complete, THE SYSTEM SHALL merge their worktree branches into the working branch before proceeding to Gate 2.

**Sequential tasks**: Already merged immediately after each task completion during Phase 2.

**Parallel tasks (batch merge)**:
1. Collect all worktree branches with changes
2. Merge in task-ID order (T1 → T2 → T3 ...)
3. For each branch: `git -c gc.auto=0 merge <branch>` → on success: `git worktree remove <path>`
4. On merge conflict: `git merge --abort` → abort pipeline → report error

See @.claude/skills/autopus/worktree-isolation.md for full merge strategy and safety rules.

### Gate 2: Validation

```
Agent(
  subagent_type = "validator",
  prompt = """
    Validate the implementation result. Return format:
    Verdict: PASS | FAIL
    Issues: <list of issues>
    Recommended Agent: executor | tester | planner
  """
)
```

### Phase 2.5: Annotation (Post-Validation)

WHEN Gate 2 returns PASS, THE SYSTEM SHALL execute an annotation step before proceeding to Phase 3.

The executor agent that implemented the code is re-spawned to apply @AX tags:

```
Agent(
  subagent_type = "executor",
  prompt = """
    Apply @AX tags to modified files based on the ax-annotation skill.
    Reference: pkg/content/ax.go:GenerateAXInstruction() for canonical rules.

    For each modified file:
    1. Scan for NOTE triggers (magic constants, undocumented exports >100 lines)
    2. Scan for WARN triggers (goroutines without context, complexity >= 15, global state mutation)
    3. Scan for ANCHOR triggers (grep for fan_in >= 3 callers)
    4. Scan for TODO triggers (public functions without tests)
    5. Validate per-file limits (ANCHOR max 3, WARN max 5)
    6. Apply overflow strategy if limits exceeded

    All tags MUST include the [AUTO] prefix.
    Modified files: {list of files from Phase 2}
  """,
  permissionMode = "bypassPermissions"
)
```

Annotation is skipped for harness-only tasks (all `.md` files).

### Phase 3.5: UX Verification (Optional)

WHEN the target project contains frontend components (.tsx/.jsx files) AND the pipeline is running in subagent or Agent Teams mode (not `--solo`), THE SYSTEM SHALL execute UX verification between Testing and Review.

```
Agent(
  subagent_type = "general-purpose",
  prompt = """
    Run frontend UX verification on all modified frontend components.
    Reference: .claude/skills/autopus/frontend-verify.md for the full pipeline.

    1. Analyze git diff to identify changed .tsx/.jsx files
    2. Generate or heal Playwright E2E tests for affected components
    3. Execute tests and capture screenshots
    4. Analyze screenshots for visual issues (layout, readability, responsiveness)
    5. Attempt auto-fix for WARN/FAIL items (max 2 attempts)

    Return format:
    Verdict: PASS | WARN | FAIL
    Screenshots: N analyzed
    Issues: <list of issues with file references>
    Fixes: <list of auto-applied fixes>
  """,
  permissionMode = "bypassPermissions"
)
```

Activation conditions:
- Frontend files (.tsx/.jsx) exist in the changed file set
- Skip if all changes are backend-only (.go, .md)

Phase 3.5 does NOT renumber existing phases. Testing remains Phase 3, Review remains Phase 4.

### Phase 3: Testing

```
Agent(
  subagent_type = "tester",
  prompt = """
    Raise coverage to 85%+.
    Add missing edge case tests.
  """,
  permissionMode = "bypassPermissions"
)
```

### Phase 4: Review

```
Agent(
  subagent_type = "reviewer",
  prompt = """
    Perform a code review using TRUST 5 criteria. Return format:
    Verdict: APPROVE | REQUEST_CHANGES
    Issues: <list of issues>
  """
)
```

## Parallel vs Sequential Decision Criteria

| Condition                                     | Execution         | Worktree Isolation |
|-----------------------------------------------|-------------------|--------------------|
| planner specifies Mode = "parallel"           | Parallel          | Yes (`isolation: "worktree"`) |
| planner specifies Mode = "sequential"         | Sequential        | No (main worktree) |
| File ownership conflict detected (R2)         | Switch to sequential | No (main worktree) |
| Task uses previous task result as input       | Sequential        | No (main worktree) |

File ownership conflict always forces sequential execution, even when worktree isolation is available (R2). The planner SHOULD design non-overlapping file ownership to maximize parallel execution with worktree isolation.

## Quality Gate Handling

```
PASS  → Proceed to next Phase
FAIL  → Delegate fix to the Recommended Agent from Gate Verdict → re-validate
```

Retry limits:

- Gate 2 (Validation): maximum 3 retries
- Phase 4 (Review): maximum 2 retries

If the limit is exceeded, abort the pipeline and notify the user:

```
Pipeline aborted: failed to resolve [Gate name] after [N] retries.
Manual intervention required. Last issue: [Issues content]
```

## Agent Failure Handling

| Failure Type              | Handling                                           |
|---------------------------|----------------------------------------------------|
| Exits due to maxTurns     | Detect remaining work → spawn new Agent()          |
| Subagent returns error    | Analyze error content → retry with revised prompt  |
| Retry limit exceeded      | Main session implements directly (fallback)        |

Fallback condition: if a subagent fails 2 consecutive times, the main session handles the task directly.

## Harness-Only Task Handling

When all tasks modify only `.md` files:

- Skip Go build/test validation
- Validator checks only file format (frontmatter YAML, section structure)
- Coverage gate (85%) is not applied

Determination: if all "file ownership" entries in the planner's assignment table are `*.md`, treat as harness-only.

## Result Integration and Completion

Once all Phases are complete:

1. Collect results from each agent and output a final summary
2. Update the SPEC file status to `"done"`
3. Guide next steps: `/auto sync <SPEC-ID>`

### Final Summary Format

```
## Pipeline Completion Summary

SPEC: <SPEC-ID>
Tasks: <completed> / <total>
Coverage: <measured>%
Review: APPROVE

Completed Files:
- <file path 1>
- <file path 2>
```

## Completion Criteria

- [ ] All Phases executed in order
- [ ] PASS verdict received at each Gate
- [ ] Coverage 85%+ confirmed
- [ ] SPEC status = "done" updated
- [ ] Final summary output complete
