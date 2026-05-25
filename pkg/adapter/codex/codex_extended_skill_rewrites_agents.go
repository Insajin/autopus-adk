package codex

func codexAgentTeamsSkillBody() string {
	return `
# Codex Team Mode Skill

## Overview

Codex ` + "`--team`" + ` mode is a first-class Autopus team profile built on the native Codex multi-agent tool surface.

**Activation flag**: ` + "`@auto go SPEC-ID --team`" + `

This mode does not use Claude Code Team APIs. It uses ` + "`spawn_agent(...)`" + `, ` + "`send_input(...)`" + `, ` + "`wait_agent(...)`" + `, and ` + "`close_agent(...)`" + ` from the Codex ` + "`multi_agent`" + ` feature.

## Prerequisites

- Codex CLI exposes ` + "`multi_agent`" + ` and ` + "`goals`" + ` as enabled features. Verify with ` + "`codex features list`" + ` when in doubt.
- ` + "`.codex/config.toml`" + ` should contain ` + "`[features] multi_agent = true`" + ` and ` + "`goals = true`" + `.
- The current session must expose ` + "`spawn_agent`" + `. If it does not, stop before Phase 1 with a workflow authenticity blocker and ask for a working multi-agent surface or ` + "`--solo`" + `.

## Mode Semantics

| Invocation | Behavior |
|------------|----------|
| ` + "`@auto go`" + ` | Default subagent pipeline: phase-by-phase specialist spawning |
| ` + "`@auto go --auto`" + ` | Skip confirmation gates and treat subagent spawning as explicitly approved |
| ` + "`@auto go --solo`" + ` | Disable subagents; main session implements directly |
| ` + "`@auto go --team`" + ` | Use the Codex team profile: main session as Lead, Builder/Guardian workers, explicit coordination and teardown |

## Team Composition (Lead/Builder/Guardian)

The main session is always the Lead. Do not spawn a separate lead worker.

- **Lead**: main session. Owns phase/gate state, worker prompts, result integration, final handoff, and goal status.
- **Builder**: one or more ` + "`executor`" + `/` + "`tester`" + ` workers with disjoint write ownership.
- **Guardian**: ` + "`validator`" + `, ` + "`reviewer`" + `, and security-focused workers that verify and review the Builder output.

## Goal Integration

` + "`/goal`" + ` is a Codex thread feature. ` + "`@auto goal`" + ` is only a thin Autopus wrapper over that same thread state.

- Do not create a thread goal for ordinary ` + "`@auto`" + ` requests.
- If the user explicitly starts or asks for a goal, prefer the ` + "`@auto goal \"<objective>\" [--budget N]`" + ` wrapper or use ` + "`create_goal`" + ` with only the objective and any explicit token budget.
- If an active goal exists, call ` + "`get_goal`" + ` at the start of a long ` + "`@auto go`" + ` / ` + "`@auto dev`" + ` run and include the objective in worker prompts.
- Only call ` + "`update_goal(status=\"complete\")`" + ` after the objective is actually achieved.
- Only call ` + "`update_goal(status=\"blocked\")`" + ` when the same blocking condition has repeated for the required consecutive goal turns.

## Spawn Pattern

` + "```python" + `
spawn_agent(
    agent_type="executor",
    message="""
    Role: Builder.
    Own only: <paths>.
    Do not edit: <paths>.
    Completion: implement the assigned task and run focused tests.
    Return: owned_paths, changed_files, verification, blockers, next_required_step.
    """,
)

spawn_agent(
    agent_type="validator",
    message="""
    Role: Guardian.
    Verify the Builder-owned paths only.
    Return: owned_paths, changed_files, verification, blockers, next_required_step.
    """,
)
` + "```" + `

Use ` + "`send_input(...)`" + ` only from the Lead/main session when a worker needs follow-up instructions. Use ` + "`wait_agent(...)`" + ` sparingly, and ` + "`close_agent(...)`" + ` once a worker is no longer needed.

## Completion Evidence

Team mode final output must include:

- ` + "`team_mode: codex_multi_agent`" + `
- ` + "`subagent_dispatch_count`" + `
- ` + "`subagent_roles_dispatched`" + `
- ` + "`goal_status`" + ` when a Codex goal is active, otherwise ` + "`goal_status: none`" + `
- ` + "`degraded-mode`" + ` and ` + "`degraded_mode`" + `
- ` + "`next_required_step`" + `
	`
}
