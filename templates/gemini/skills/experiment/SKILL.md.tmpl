---
name: auto-experiment
---

# Experiment Loop Skill (XLOOP)

Skill for running automated iterative improvement loops that optimize a measurable metric
while keeping changes simple and reversible.

## Overview

The experiment loop (`auto experiment`) runs an agent-driven cycle:

1. Measure baseline metric
2. Ask an executor agent to make one focused change
3. Measure the new metric
4. Decide: keep (commit) or discard (reset)
5. Check circuit breaker and simplicity gate
6. Repeat until MaxIterations or circuit break

## Setup

```bash
# Initialize experiment branch (requires clean worktree)
auto experiment init --session-id my-session

# Verify metric command works
auto experiment metric \
  --metric 'go test -bench=. ./... | grep ns/op | awk "{print \"{\\\"metric\\\":\"$3\"}\"}"' \
  --metric-key metric
```

## Full Loop Configuration

```bash
auto experiment init --session-id opt-$(date +%s)

# Then invoke the loop via the agent skill below
```

## Agent Orchestration Pattern

The experiment loop is driven by invoking an executor agent repeatedly.
Pass the full history context on each call so the agent can learn from past iterations.

```
## Experiment Loop — Iteration {N}

### Config
- MetricCmd: {cmd}
- Direction: {minimize|maximize}
- Target: {files}
- Scope: {files or "same as target"}
- SimplicityThreshold: {threshold}

### History (last 5 results)
{JSON array of recent Result objects from `auto experiment record`}

### Baseline: {baseline_value} {unit}
### Best so far: {best_value} at iteration {best_iter}

### Your task
Make ONE focused change to the target files that should improve the metric.
Do NOT modify files outside the allowed scope.
After making your change, run:
  auto experiment commit --iteration {N} --description "your change description"
Then output the description in the last line.
```

## Keep / Discard Decision

After each executor run, measure the metric and decide:

```
new_value = RunMetricMedian(cfg, cmd)
simplicity = CalculateSimplicity(baseline, new_value, linesAdded, linesRemoved, direction)

if direction.IsBetter(new_value, best_value):
    if simplicity >= cfg.SimplicityThreshold:
        status = "keep"
        best_value = new_value
        circuit_breaker.Record(true)
    else:
        status = "discard"   # improvement too small relative to code complexity
        ResetToCommit(last_keep_hash)
        circuit_breaker.Record(false)
else:
    status = "discard"
    ResetToCommit(last_keep_hash)
    circuit_breaker.Record(false)
```

Record the result:

```bash
auto experiment record \
  --iteration {N} \
  --status {keep|discard} \
  --metric-value {value} \
  --description "{description}"
```

## Crash Handling

When the executor agent's change causes a build failure or test crash:

### Trivial crash (compile error, test panic)
- Reset immediately: `auto experiment reset --commit {last_keep_hash}`
- Record status `crash`
- Continue to next iteration
- Do NOT count as circuit breaker failure (the code was broken, not the approach)

### Fundamental crash (3+ consecutive crashes on same approach)
- Treat as circuit breaker failure
- Record status `crash`
- Call `circuit_breaker.Record(false)`

### Abort condition
- If crash prevents metric measurement entirely for 5+ consecutive iterations: abort loop

## Circuit Breaker

The circuit breaker trips when N consecutive iterations show no improvement.
Default N = 10 (configurable via `--circuit-breaker-n`).

```
if circuit_breaker.IsTripped():
    print("Circuit breaker tripped after {N} consecutive non-improvements")
    print("Best result: iteration {best_iter}, metric={best_value}")
    auto experiment summary
    exit loop
```

The breaker resets automatically when a keep is recorded.

## Simplicity Gate

The simplicity score penalizes large code changes relative to metric improvement.
A change that improves the metric by 0.1% but adds 500 lines is likely not worth keeping.

```
score = improvement_ratio / lines_changed
threshold = cfg.SimplicityThreshold  # default 0.001

if score < threshold:
    # Discard even if metric improved — change is too complex
    status = "discard"
```

Special cases:
- `score < 0`: metric got worse → always discard
- `score == 0`: no metric change → always discard
- Zero lines changed with improvement: score is high → always keep

## Scope Validation

After each executor commit, verify changes stay within allowed scope:

```bash
# Check scope (called internally by loop logic)
# Uses git.CheckScope(baseCommit, allowedPaths)
```

If out-of-scope files are modified:
- Record status `scope-violation`
- Reset to last keep hash
- Do NOT count as circuit breaker failure

## SIGINT Graceful Shutdown

When the user presses Ctrl+C during the loop:

1. Finish the current metric measurement if in progress
2. Do NOT start a new executor agent call
3. If last commit was a "discard" candidate: reset to last keep hash
4. Print summary: `auto experiment summary`
5. Exit cleanly

The experiment branch is preserved — the user can resume later.

## Summary

At loop end (natural completion, circuit break, or SIGINT):

```bash
auto experiment summary
```

Outputs:
```
total=25 keep=12 discard=13 best=1.2340 (iter 18)
```

## Example: Benchmark Optimization

```bash
# 1. Initialize
auto experiment init --session-id bench-opt-001

# 2. Run loop (orchestrated by this skill)
# Config:
#   --metric 'go test -bench=BenchmarkFoo -benchtime=3s ./pkg/foo/ | grep BenchmarkFoo | awk "{print \"{\\\"metric\\\":\"$3\"}"}'
#   --direction minimize
#   --target pkg/foo/foo.go
#   --max-iterations 30
#   --metric-runs 3

# 3. After loop completes
auto experiment summary
```

## Ref

- SPEC-XLOOP-001
- pkg/experiment/ — core loop types and utilities
- internal/cli/experiment.go — CLI entry points
