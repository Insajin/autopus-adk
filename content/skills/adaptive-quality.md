---
name: adaptive-quality
description: Per-task execution profile selection based on complexity in Balanced quality mode
triggers:
  - adaptive quality
  - 적응형 품질
  - complexity
  - 복잡도
category: agentic
level1_metadata: "adaptive quality, complexity assessment, execution profiles, cost optimization, Balanced mode"
---

# Adaptive Quality Skill

## Overview

Adaptive Quality is a sub-extension of Quality Mode. In **Balanced mode only**, task complexity determines the execution profile used for each `Agent()` call. High-complexity tasks still receive the strongest reasoning path, while routine tasks stay on the standard path. In this workspace, Claude no longer falls back to haiku; Codex uses the current GPT-5.x tier mapping and OpenCode keeps its configured default model.

## Relationship to Quality Mode

| Mode | Behavior |
|------|----------|
| **Ultra** | ALL tasks use the premium execution path. Complexity is IGNORED. |
| **Balanced** | Complexity determines the execution profile. Adaptive Quality applies. |
| **Solo** | No Agent() calls. Not applicable. |

Adaptive Quality is **not** a replacement for Quality Mode — it is a refinement that operates exclusively within Balanced mode.

## Complexity Assessment Criteria

The planner assesses each task before spawning an agent. The assessment considers:

| Factor | Weight |
|--------|--------|
| `file_count` | Number of files to be modified |
| `estimated_lines` | Expected lines of new/changed code |
| `requirement_count` | Number of distinct requirements |
| `dependency_count` | Number of packages/modules involved |

### Complexity Levels

| Level | Criteria |
|-------|----------|
| **HIGH** | 3+ files OR 200+ expected lines OR complex logic/architecture decisions |
| **MEDIUM** | 1–2 files, 50–200 lines, moderate logic |
| **LOW** | 1 file, under 50 lines, simple or mechanical changes |

When criteria overlap (e.g., 1 file but 250 lines), use the highest matching level.

## Execution Profile Table

| Complexity | Balanced | Ultra |
|-----------|----------|-------|
| HIGH | opus | opus |
| MEDIUM | sonnet (default) | opus |
| LOW | sonnet (default) | opus |

Platform note:
- Claude: HIGH=`opus`, MEDIUM/LOW=`sonnet`
- Codex: `opus` resolves to `gpt-5.5`, `sonnet` to `gpt-5.4`, and lightweight work to `gpt-5.4-mini`; LOW/MEDIUM/HIGH should also be differentiated by reasoning effort
- OpenCode: keep the configured default runtime model; LOW/MEDIUM/HIGH should be differentiated by reasoning effort until user-facing model overrides are added

## Effort Mapping (SPEC-CC21-001)

CC21 adds an explicit `effort` tier alongside model selection. Resolve it with this priority:

1. `--effort <value>`
2. `CLAUDE_CODE_EFFORT_LEVEL`
3. agent frontmatter `effort:`
4. Quality Mode mapping
5. settings default (`medium`)

Quality Mode defaults:

| Mode | Model / Tier | Effort |
|------|--------------|--------|
| Ultra | Opus 4.7 | `xhigh` |
| Ultra | Opus 4.6 / Sonnet 4.6 | `high` |
| Ultra | Haiku 4.5 | strip effort |
| Balanced | HIGH complexity | `high` |
| Balanced | MEDIUM / LOW complexity | `medium` |

Unsupported env values must fail open to Quality Mode defaults. Use `auto effort detect` when the runtime needs the resolved value explicitly.

## Agent() Call Pattern

### HIGH complexity

```python
Agent(
    subagent_type="executor",
    model="opus",
    prompt=task_prompt
)
```

### MEDIUM complexity

```python
# No model param — uses pipeline default (sonnet)
Agent(
    subagent_type="executor",
    prompt=task_prompt
)
```

### LOW complexity

```python
# Use the standard model. Lower reasoning effort only on platforms that support it.
Agent(
    subagent_type="executor",
    prompt=task_prompt
)
```

## Configuration Override (`autopus.yaml`)

Override the default model mapping per complexity level:

```yaml
quality:
  presets:
    balanced:
      adaptive:
        high: opus
        medium: sonnet
        low: sonnet
```

To disable adaptive quality and use a fixed model in Balanced mode:

```yaml
quality:
  presets:
    balanced:
      adaptive: false
      model: sonnet
```

## Cost Estimation

### Formula

```
cost = Σ(task_tokens × model_price_per_token)
```

Where `model_price_per_token` is looked up from the pricing table in `pkg/cost/estimator.go`.

### Estimated Savings

| Scenario | Savings vs All-Opus |
|----------|---------------------|
| Typical project (mixed complexity) | 20–40% |
| Mostly LOW tasks (refactoring, docs) | up to 60% |
| Mostly HIGH tasks (new features) | < 5% |

**Reference**: `pkg/cost/estimator.go` for current pricing tables and token estimation logic.

## Planner Integration

The planner executes complexity assessment during Phase 1 and annotates each task:

```
Task T1: Add user authentication
  → file_count: 4, estimated_lines: 280
  → Complexity: HIGH → model: opus

Task T2: Update error message string
  → file_count: 1, estimated_lines: 3
  → Complexity: LOW → standard path (sonnet / lower reasoning effort)
```

The complexity annotation is included in the execution plan and passed to the orchestrator before Agent() calls are made.
