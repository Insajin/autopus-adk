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

Adaptive Quality is a sub-extension of Quality Mode. In **Balanced mode only**, task complexity informs the role profile selected for each `Agent()` call. High-judgment tasks use the strongest assigned tier, while routine tasks stay on the standard path. The shared order is `fable` > `opus` > `sonnet` > `haiku`; the shipped Ultra and Balanced presets do not assign Haiku.

## Relationship to Quality Mode

| Mode | Behavior |
|------|----------|
| **Ultra** | Uses the fixed role profile: reasoning core=`fable`, remaining roles=`opus`. Complexity is IGNORED. |
| **Balanced** | Uses `fable`/`opus`/`sonnet` by role and task complexity. Adaptive Quality applies. |
| **Solo** | No Agent() calls. Not applicable. |

Adaptive Quality is **not** a replacement for Quality Mode — it is a refinement that operates exclusively within Balanced mode.

### Provider-specific mode selection

Mixed Claude Code and Codex projects can select Ultra or Balanced independently while retaining
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

| Assignment | Balanced | Ultra |
|-----------|----------|-------|
| Strategic judgment | `fable`: planner, architect, security-auditor | `fable`: planner, architect, spec-writer, security-auditor, reviewer, debugger, deep-worker |
| Implementation, review, and deep work | `opus`: spec-writer, deep-worker, executor, reviewer, debugger | `opus`: every role outside the seven-role reasoning core |
| Routine and default work | `sonnet`: remaining roles | follows the Ultra role profile; complexity does not lower the tier |

Platform note:
- Claude maps `fable`, `opus`, `sonnet`, and `haiku` to Claude Fable 5.1, Opus 5, Sonnet 5, and Haiku 4.5. The shipped presets do not assign Haiku.
- Codex maps `fable` to Astra/`max`, `opus` to Sol/`xhigh`, `sonnet` to Terra with role effort, and `haiku` to Luna with role effort. Quality-managed supervisors and orchestras use Astra.
- Gemini maps `fable`/`opus`/`sonnet` to `gemini-3.1-pro` and `haiku` to `gemini-3.8-flash`.
- OpenCode keeps the configured default runtime model; tier changes act as reasoning-profile hints until explicit model overrides are available.

## Claude Fable 5.1 / Opus 5

Autopus projects the top two shared tiers onto fixed Claude model IDs. The
`fable` tier resolves to `claude-fable-5-1`; the `opus` tier resolves to
`claude-opus-5`. This keeps generated agent definitions and cost estimates
deterministic even when Claude Code aliases change over time.

| Tier | Full model ID | Claude Code aliases | Minimum Claude Code version | Price per million tokens |
|------|---------------|---------------------|-----------------------------|--------------------------|
| `fable` | `claude-fable-5-1` | `fable`, `best` | `2.1.170` | $10 input / $50 output |
| `opus` | `claude-opus-5` | `opus` | `2.1.219` | $5 input / $25 output |

`best` is entitlement-dependent and can resolve to the latest Opus when Fable
is unavailable. Deterministic routing and pricing therefore use
`claude-fable-5-1`, not the dynamic alias. The legacy full ID
`claude-fable-5` remains accepted for existing workflow definitions.

Claude Code's `opus` alias is provider- and version-dependent:

| Claude Code provider | `opus` on v2.1.219+ | Before v2.1.219 |
|----------------------|---------------------|-----------------|
| Anthropic API | Opus 5 | Opus 4.8 on v2.1.154–v2.1.218 |
| Claude Platform on AWS | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.7 before v2.1.207 |
| Amazon Bedrock | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |
| Google Cloud Agent Platform | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |
| Microsoft Foundry | Opus 4.6 | Opus 4.6 |

Opus 5 is a drop-in upgrade from Opus 4.8 at the same standard price.
Adaptive thinking is enabled by default and the default effort is `high`. If a
direct API integration explicitly sends `thinking: {"type": "disabled"}`, effort must be
`high` or lower: disabled thinking with `xhigh` or `max` returns HTTP 400.
Autopus does not add a thinking-disable flag to Claude Code argv.

Opus 4.8 remains a valid explicit model and the recommended cybersecurity
fallback for Opus 5 refusals, so compatibility entries for
`claude-opus-4-8` must not be removed. See the official
[Claude Code model configuration](https://code.claude.com/docs/en/model-config),
[models overview](https://platform.claude.com/docs/en/about-claude/models/overview),
and [Opus 5 migration guide](https://platform.claude.com/docs/en/about-claude/models/migration-guide).

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
| Any | Fable 5.1 | `max` |
| Ultra | Opus 5 / Opus 4.8 / Opus 4.7 | `max` |
| Ultra | Opus 4.6 / Sonnet 5 | `high` |
| Balanced | Opus tier | `high` |
| Balanced | Sonnet tier | `medium` |
| Any | Haiku 4.5 | strip effort |

Claude model/API effort is the closed set `low`, `medium`, `high`, `xhigh`, and
`max`. Opus 5 and Fable 5.1 accept all five values and default to `high` outside
the Autopus quality projection. Claude Code also exposes `ultracode` as a
session-only CLI value: it sends actual model effort
`xhigh` and adds dynamic workflow behavior. It is not a sixth model/API or
persisted workflow effort. Main-session `ultracode` requires Claude Code
`2.1.203` or later; reliable propagation to spawned agents and teams requires
`2.1.210` or later. Route-team binding therefore normalizes explicit
`ultracode` to `xhigh`.
See the official [effort reference](https://platform.claude.com/docs/en/build-with-claude/effort),
[Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference), and
[Claude Code model configuration](https://code.claude.com/docs/en/model-config).

Codex-specific rendering:

| Mode | Agent / Tier | `model_reasoning_effort` |
|------|--------------|--------------------------|
| Any | supervisor (`supervisor_model_policy: inherit`, default) | User Codex runtime default |
| Ultra | quality-managed supervisor | Astra + `ultra` |
| Ultra | quality-managed orchestra | Astra + `max` |
| Ultra | Fable-tier worker | Astra + `max` |
| Ultra | Opus-tier worker | Sol + `xhigh` |
| Balanced | quality-managed supervisor / orchestra | Astra + `xhigh` |
| Balanced | Fable-tier worker | Astra + `max` |
| Balanced | Opus-tier worker | Sol + `xhigh` |
| Balanced | Sonnet-tier worker | Terra + declared role effort |
| Balanced | Haiku-tier worker | Luna + declared role effort (capped at `max`) |

Codex custom agent files are loaded when a session starts. Run `auto quality provider codex <mode> --apply` to persist a Codex-only profile and refresh the current project's Codex managed agents, or `auto quality <mode> --apply` to change the global fallback and refresh every configured platform. Then start a new Codex session. `auto quality supervisor inherit --apply` keeps the primary session on the user's configured Codex model. `auto quality supervisor quality --apply` opts an unchanged Autopus-managed root config into the managed Astra profile; user-owned root model or effort assignments remain preserved and take precedence. A per-run `--quality` override can change a managed orchestra launch, but it cannot hot-swap agents already loaded by the current session.

Unsupported env values must fail open to Quality Mode defaults. Use `auto effort detect` when the runtime needs the resolved value explicitly.

## Agent Call Pattern

Claude Code 2.1.246 agent calls inherit the model-and-effort projection from the
installed agent definition. Complexity changes that definition or the selected
role; ordinary calls do not send per-call effort.

### HIGH complexity

```python
Agent(
    description="Implement the high-complexity assigned task",
    prompt=task_prompt,
    subagent_type="executor",
)
```

### MEDIUM complexity

```python
Agent(
    description="Implement the medium-complexity assigned task",
    prompt=task_prompt,
    subagent_type="executor",
)
```

### LOW complexity

```python
Agent(
    description="Implement the low-complexity assigned task",
    prompt=task_prompt,
    subagent_type="executor",
)
```

## Configuration Override (`autopus.yaml`)

Override the default model mapping per complexity level:

```yaml
quality:
  presets:
    balanced:
      adaptive:
        high: fable
        medium: opus
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

Where `model_price_per_token` is looked up in `pkg/cost/pricing.go`.

### Relative Cost

Fable 5.1 costs more per token than Opus 5, while Sonnet 5 costs less. Compare
profiles using the actual role mix and token counts rather than assuming that
either quality mode is uniformly cheaper.

**Reference**: `pkg/cost/pricing.go` for current pricing and
`pkg/cost/estimator.go` for token estimation.

## Planner Integration

The planner executes complexity assessment during Phase 1 and annotates each task:

```
Task T1: Add user authentication
  → file_count: 4, estimated_lines: 280
  → Complexity: HIGH → strategic profile (fable)

Task T2: Update error message string
  → file_count: 1, estimated_lines: 3
  → Complexity: LOW → standard profile (sonnet)
```

The complexity annotation is included in the execution plan and passed to the orchestrator before Agent() calls are made.
