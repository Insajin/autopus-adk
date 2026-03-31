---
spec: SPEC-ORCH-018
type: plan
---

# SPEC-ORCH-018 Implementation Plan

## Task Assignment Table

| Task ID | Description | Agent | Mode | File Ownership | Profile | Complexity | Priority | Dependencies |
|---------|-------------|-------|------|----------------|---------|------------|----------|--------------|
| T1 | sendPromptWithRetry retry-first logic | executor | worktree | `pkg/orchestra/surface_manager.go` | go-backend | M | P0 | none |
| T2 | pollUntilPrompt timeout 10s->30s | executor | worktree | `pkg/orchestra/interactive_debate.go` | go-backend | S | P0 | none |
| T3 | ReadScreen scrollback depth + config field | executor | worktree | `pkg/orchestra/interactive.go`, `pkg/orchestra/types.go` | go-backend | S | P0 | none |
| T4 | --no-judge flag + skip judge logic | executor | worktree | `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/interactive_debate_helpers.go`, `pkg/orchestra/types.go` | go-backend | M | P0 | none |
| T5 | --yield-rounds mode + JSON output | executor | worktree | `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/yield.go`, `pkg/orchestra/types.go` | go-backend | L | P0 | T4 |
| T6 | `auto orchestra collect` command | executor | worktree | `internal/cli/orchestra_collect.go`, `internal/cli/orchestra.go` | go-backend | M | P0 | T5, T8 |
| T7 | `auto orchestra cleanup` command | executor | worktree | `internal/cli/orchestra_cleanup.go`, `internal/cli/orchestra.go` | go-backend | S | P0 | T8 |
| T8 | Session persistence file | executor | worktree | `pkg/orchestra/session.go` | go-backend | M | P2->P0* | none |
| T9 | --context flag + context-aware prompt | executor | worktree | `internal/cli/orchestra_brainstorm.go`, `pkg/orchestra/debate.go` | go-backend | M | P1 | none |
| T10 | Structured JSON output for --no-judge | executor | worktree | `pkg/orchestra/yield.go` | go-backend | S | P1 | T5 |
| T11 | idea.md skill update | executor | inline | `.claude/skills/autopus/idea.md` | skill-writer | M | P1 | T5, T6, T7 |
| T12 | `auto orchestra inject` command | executor | worktree | `internal/cli/orchestra_inject.go`, `internal/cli/orchestra.go` | go-backend | M | P2 | T8 |
| T13 | Unit tests for T1-T4 | tester | worktree | `pkg/orchestra/*_test.go` | go-test | M | P0 | T1, T2, T3, T4 |
| T14 | Unit tests for T5-T8 | tester | worktree | `pkg/orchestra/*_test.go`, `internal/cli/*_test.go` | go-test | M | P0 | T5, T6, T7, T8 |

*T8 is P2 in the SPEC but promoted to P0 in the plan because T5, T6, T7 depend on session persistence.

## Execution Phases

### Phase 1: Stability Fixes (Parallel)

Tasks T1, T2, T3 can run in parallel -- they modify different files with no overlap.

```
T1 (surface_manager.go)  ──┐
T2 (interactive_debate.go) ──┼── merge
T3 (interactive.go + types.go) ──┘
```

### Phase 2: Core Infrastructure (Sequential then Parallel)

T4 and T8 can run in parallel (different files). T5 depends on T4 (needs NoJudge field in types.go).

```
T4 (--no-judge) ──┐
T8 (session.go)  ──┼── merge ── T5 (--yield-rounds) ── merge
```

### Phase 3: Commands (Parallel after T5+T8)

T6 and T7 can run in parallel after T5 and T8 are merged.

```
T6 (collect cmd)  ──┐
T7 (cleanup cmd)  ──┼── merge
```

### Phase 4: P1 Enhancements (Parallel)

T9, T10, T11 after Phase 3 merge. T9 is independent. T10 depends on T5. T11 depends on T5+T6+T7.

```
T9 (--context)  ──┐
T10 (JSON output) ──┼── merge
T11 (idea.md)    ──┘
```

### Phase 5: P2 + Tests

T12 after Phase 3. T13, T14 after their respective dependencies.

```
T12 (inject cmd) ── merge
T13 (tests P0)   ──┐
T14 (tests P1)   ──┼── merge
```

## types.go Field Additions

The following fields will be added to `OrchestraConfig` in `pkg/orchestra/types.go`:

```go
NoJudge        bool   // R4: skip judge phase when true
YieldRounds    bool   // R5: yield after round 1 with JSON output
ScrollbackLines int   // R3: ReadScreen scrollback depth (default 500)
ContextMode    bool   // R8: load project context into brainstorm prompt
```

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| types.go merge conflict (T3, T4, T5 all add fields) | T3 runs first, T4+T5 sequential after merge |
| Session file race condition | Session ID includes timestamp + random suffix |
| pollUntilPrompt 30s may still be too short for slow providers | Log warning at 20s, configurable via provider-level override (future) |
| yield.go new file may overlap with existing output logic | Keep yield.go focused on JSON serialization only |
