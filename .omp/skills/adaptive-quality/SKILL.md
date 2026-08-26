---
name: adaptive-quality
description: Per-task execution profile selection based on complexity in Balanced quality mode
compatibility: omp
---

# Adaptive Quality Skill

## Overview

Adaptive Quality is a sub-extension of Quality Mode. In **Balanced mode only**, task complexity determines the execution profile used for each `task` batch call. High-complexity tasks still receive the strongest reasoning path, while routine tasks stay on the standard path. In this workspace, OMP no longer falls back to haiku; OMP maps managed workers to Sol/Terra/Luna, while new projects leave the primary session on the user's model unless `supervisor_model_policy: quality` is selected; OMP keeps its configured default model.

## Relationship to Quality Mode

| Mode | Behavior |
|------|----------|
| **Ultra** | ALL tasks use the premium execution path. Complexity is IGNORED. |
| **Balanced** | Complexity determines the execution profile. Adaptive Quality applies. |
| **Solo** | No task batch items. Not applicable. |

Adaptive Quality is **not** a replacement for Quality Mode — it is a refinement that operates exclusively within Balanced mode.

### Provider-specific mode selection

Mixed OMP and OMP projects can select Ultra or Balanced independently while retaining
the existing global fallback:

```yaml
quality:
  default: balanced
  providers:
    claude: ultra
    codex: balanced
```

Resolve the effective mode in this order: explicit per-run global `--quality`, persisted
`quality.providers.<claude|codex>`, persisted `quality.default`, then `balanced`. The global
per-run flag intentionally overrides both providers without rewriting YAML.

```bash
auto quality provider claude ultra --apply
auto quality provider codex balanced --apply
auto quality provider claude inherit --apply
```

`claude-code` is accepted as a CLI alias for canonical provider key `claude`. Provider-specific
`--apply` refreshes only that configured platform; `auto quality <mode> --apply` continues to
refresh every configured platform.

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
- OMP: HIGH=`opus`, MEDIUM/LOW=`sonnet`
- OMP: Balanced uses Sol/`xhigh` for quality-managed strategic and Opus-tier work, Terra with role effort for Sonnet-tier work, and Luna with role effort for Haiku-tier work. Ultra uses Sol/`ultra` for a quality-managed supervisor and orchestra, Sol/`max` for `planner`, `architect`, and `security-auditor`, and Sol/`xhigh` for every other managed agent. An `inherit` supervisor keeps the user's OMP runtime default in either mode.
- OMP: keep the configured default runtime model; LOW/MEDIUM/HIGH should be differentiated by reasoning effort until user-facing model overrides are added

## OMP Opus 5 (Default Opus Path)

Autopus uses the fixed `claude-opus-5` model ID for OMP Ultra agents,
Balanced strategic agents, and high-complexity OMP routing. OMP's
`opus` alias is provider- and version-dependent:

| OMP provider | `opus` on v2.1.219+ | Before v2.1.219 |
|----------------------|---------------------|-----------------|
| Anthropic API | Opus 5 | Opus 4.8 on v2.1.154–v2.1.218 |
| OMP Platform on AWS | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.7 before v2.1.207 |
| Amazon Bedrock | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |
| Google Cloud Agent Platform | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |
| Microsoft Foundry | Opus 4.6 | Opus 4.6 |

| Surface | Value |
|---------|-------|
| Full model ID | `claude-opus-5` |
| OMP alias | `opus` |
| Minimum OMP version | `2.1.219` |
| Price per million tokens | $5 input / $25 output |
| Native limits | 1M-token context / 128k-token output |
| Effort levels | `low`, `medium`, `high`, `xhigh`, `max` |

Opus 5 is a drop-in upgrade from Opus 4.8 at the same standard price.
Adaptive thinking is enabled by default and the default effort is `high`. If a direct API
integration explicitly sends `thinking: {"type": "disabled"}`, effort must be
`high` or lower: disabled thinking with `xhigh` or `max` returns HTTP 400.
Autopus does not add a thinking-disable flag to OMP argv.

Opus 4.8 remains a valid explicit model and the recommended cybersecurity
fallback for Opus 5 refusals, so compatibility entries for
`claude-opus-4-8` must not be removed. See the official
[OMP model configuration](https://code.claude.com/docs/en/model-config),
[models overview](https://platform.claude.com/docs/en/about-claude/models/overview),
and [Opus 5 migration guide](https://platform.claude.com/docs/en/about-claude/models/migration-guide).

## OMP Fable 5 (Explicit Opt-In)

Fable 5 is an explicit OMP choice, not an Autopus quality default. Keep the
Opus/Sonnet mappings above unless the user or provider configuration selects Fable.

| Surface | Value |
|---------|-------|
| Full model ID | `claude-fable-5` |
| OMP aliases | `fable`, `best` |
| `best` behavior | Fable when the organization has access; otherwise the latest Opus |
| Minimum OMP version | `2.1.170` |
| Price per million tokens | $10 input / $50 output |
| Native limits | 1M-token context / 128k-token output |

Fable access is entitlement-dependent and is not available for organizations with
Zero Data Retention (ZDR). The `fable` and `best` aliases are convenient OMP
inputs, but deterministic cost estimates require the resolved full model ID.
See the official [OMP model configuration](https://code.claude.com/docs/en/model-config)
and [Fable 5 introduction](https://platform.claude.com/docs/en/about-claude/models/introducing-claude-fable-5-and-claude-mythos-5).

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
| Ultra | Opus 5 / Opus 4.8 / Opus 4.7 | `max` |
| Ultra | Opus 4.6 / Sonnet 5 | `high` |
| Ultra | Haiku 4.5 | strip effort |
| Balanced | HIGH complexity | `high` |
| Balanced | MEDIUM / LOW complexity | `medium` |

OMP model/API effort is the closed set `low`, `medium`, `high`, `xhigh`, and
`max`. Opus 5 and Fable support all five and default to `high`. OMP also
exposes `ultracode` as a session-only CLI value: it sends actual model effort
`xhigh` and adds dynamic workflow behavior. It is not a sixth model/API or
persisted workflow effort. Main-session `ultracode` requires OMP
`2.1.203` or later; reliable propagation to spawned agents and teams requires
`2.1.210` or later. Route-team binding therefore normalizes explicit
`ultracode` to `xhigh`.
See the official [effort reference](https://platform.claude.com/docs/en/build-with-claude/effort),
[OMP CLI reference](https://code.claude.com/docs/en/cli-reference), and
[OMP model configuration](https://code.claude.com/docs/en/model-config).

OMP-specific rendering:

| Mode | Agent / Tier | `model_reasoning_effort` |
|------|--------------|--------------------------|
| Any | supervisor (`supervisor_model_policy: inherit`, default) | User OMP runtime default |
| Ultra | quality-managed supervisor / orchestra | Sol + `ultra` |
| Ultra | `planner` / `architect` / `security-auditor` | Sol + `max` |
| Ultra | every other managed agent | Sol + `xhigh` |
| Balanced | quality-managed supervisor / orchestra / Opus-tier worker | Sol + `xhigh` |
| Balanced | reviewer (Sonnet tier) | Terra + `high` |
| Balanced | standard Sonnet-tier worker | Terra + `medium` |
| Balanced | Haiku-tier worker | Luna + declared effort (capped at `max`) |

OMP custom agent files are loaded when a session starts. Run `auto quality provider codex <mode> --apply` to persist a OMP-only profile and refresh the current project's OMP managed agents, or `auto quality <mode> --apply` to change the global fallback and refresh every configured platform. Then start a new OMP session. `auto quality supervisor inherit --apply` keeps the primary session on the user's configured OMP model. `auto quality supervisor quality --apply` opts an unchanged Autopus-managed root config into the managed Sol profile; user-owned root model or effort assignments remain preserved and take precedence. A per-run `--quality` override can change a managed orchestra launch, but it cannot hot-swap agents already loaded by the current session.

Unsupported env values must fail open to Quality Mode defaults. Use `auto effort detect` when the runtime needs the resolved value explicitly.

## Agent Call Pattern

OMP 2.1.246 agent calls inherit the model-and-effort projection from the
installed agent definition. Complexity changes that definition or the selected
role; ordinary calls do not send per-call effort.

### HIGH complexity

```json
{
  "i": "Dispatching bounded OMP work",
  "context": "Shared goal, constraints, owned-path boundaries, and cross-task contracts.",
  "tasks": [
    {
      "name": "executor",
      "agent": "executor",
      "task": "Complete the assigned work and return the required receipt.",
      "outputSchema": {
        "type": "object",
        "additionalProperties": false,
        "required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"],
        "properties": {
          "owned_paths": {"type": "array", "items": {"type": "string"}},
          "changed_files": {"type": "array", "items": {"type": "string"}},
          "verification": {"type": "array", "items": {"type": "string"}},
          "blockers": {"type": "array", "items": {"type": "string"}},
          "next_required_step": {"type": "string"}
        }
      },
      "schemaMode": "strict"
    }
  ]
}
```

### MEDIUM complexity

```json
{
  "i": "Dispatching bounded OMP work",
  "context": "Shared goal, constraints, owned-path boundaries, and cross-task contracts.",
  "tasks": [
    {
      "name": "executor",
      "agent": "executor",
      "task": "Complete the assigned work and return the required receipt.",
      "outputSchema": {
        "type": "object",
        "additionalProperties": false,
        "required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"],
        "properties": {
          "owned_paths": {"type": "array", "items": {"type": "string"}},
          "changed_files": {"type": "array", "items": {"type": "string"}},
          "verification": {"type": "array", "items": {"type": "string"}},
          "blockers": {"type": "array", "items": {"type": "string"}},
          "next_required_step": {"type": "string"}
        }
      },
      "schemaMode": "strict"
    }
  ]
}
```

### LOW complexity

```json
{
  "i": "Dispatching bounded OMP work",
  "context": "Shared goal, constraints, owned-path boundaries, and cross-task contracts.",
  "tasks": [
    {
      "name": "executor",
      "agent": "executor",
      "task": "Complete the assigned work and return the required receipt.",
      "outputSchema": {
        "type": "object",
        "additionalProperties": false,
        "required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"],
        "properties": {
          "owned_paths": {"type": "array", "items": {"type": "string"}},
          "changed_files": {"type": "array", "items": {"type": "string"}},
          "verification": {"type": "array", "items": {"type": "string"}},
          "blockers": {"type": "array", "items": {"type": "string"}},
          "next_required_step": {"type": "string"}
        }
      },
      "schemaMode": "strict"
    }
  ]
}
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

The complexity annotation is included in the execution plan and passed to the orchestrator before task batch items are made.

## OMP Coordination Contract

### Ownership gate

- Choose exactly one DAG owner with `--execution-owner omp|orca` before dispatch; omission selects owner `omp`.
- Owner `omp` is the default. The current OMP session is the sole DAG owner and uses its native `task`, `hub`, and `todo` tools.
- Owner `orca` is allowed only when `--execution-owner orca` is explicit. Before any Orca orchestration, run and read `orca skills get orchestration --full`.
- The single DAG owner invariant is mandatory: owner `orca` creates no OMP task DAG, and owner `omp` creates no Orca Run.

### Native field contracts

```json
{
  "i": "Dispatching bounded OMP work",
  "context": "Shared goal, constraints, owned-path boundaries, and cross-task contracts.",
  "tasks": [
    {
      "name": "Worker",
      "task": "Complete one self-contained assignment and return only the required receipt.",
      "outputSchema": {
        "type": "object",
        "additionalProperties": false,
        "required": ["owned_paths", "changed_files", "verification", "blockers", "next_required_step"],
        "properties": {
          "owned_paths": {"type": "array", "items": {"type": "string"}},
          "changed_files": {"type": "array", "items": {"type": "string"}},
          "verification": {"type": "array", "items": {"type": "string"}},
          "blockers": {"type": "array", "items": {"type": "string"}},
          "next_required_step": {"type": "string"}
        }
      },
      "schemaMode": "strict"
    }
  ]
}
```

- Inspect the current dynamic `task` schema before dispatch. Use the shown batch shape only when it exposes top-level `context` and `tasks`; otherwise use the discovered flat shape and place shared context in `local://`.
- Every model-authored `task`, `hub`, and `todo` call includes a concise top-level `i` while `tools.intentTracing` is enabled.
- Every `tasks` item uses `name` when a stable agent id is useful and carries per-item `task`, `outputSchema`, and `schemaMode`. Set `agent` only to select a custom agent type; omit it for OMP's default general worker.
- `isolated` and `effort` are conditional dynamic fields. Add `isolated` or `effort` only after the current schema exposes that exact field; otherwise omit it.
- `outputSchema` is the strict five-field receipt JSON Schema shown in the normalized batch: `owned_paths`, `changed_files`, `verification`, `blockers`, and `next_required_step`.
- Retain the agent id returned by `task`. For a non-isolated or otherwise revivable worker, every follow-up goes to that same id with `hub` send fields `{"i":"Following up with an existing worker","op":"send","to":"<same agent id>","message":"<follow-up>"}`; do not create a replacement merely to continue revivable work.
- An isolated worker is terminal after workspace cleanup and cannot be revived. A correction is a new explicitly named `task` item with freshly declared ownership and context, not a `hub` send to the terminal agent id.
- The parent OMP session owns progress. A `todo` call contains one top-level operation and intent: initialize with `{"i":"Updating parent-owned progress","op":"init","list":[{"phase":"Implementation","items":["..."]}]}`, advance with `{"i":"Updating parent-owned progress","op":"start","task":"<exact task content>"}`, complete with `{"i":"Updating parent-owned progress","op":"done","task":"<exact task content>"}`, and block with `{"i":"Updating parent-owned progress","op":"block","task":"<exact task content>","reason":"<reason>"}`.
