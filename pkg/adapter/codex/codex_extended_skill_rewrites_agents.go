package codex

func codexAgentTeamsSkillBody() string {
	return `
# Agent Teams Skill

## Overview

This document is a reserved placeholder for a future native Codex multi-agent / teams surface.

**Activation flag**: ` + "`@auto go SPEC-ID --team`" + `

Today, Codex should continue to use the default ` + "`spawn_agent(...)`" + ` subagent pipeline. Do not reinterpret ` + "`--team`" + ` as extra parallel orchestration in the harness.

## Current Behavior

- ` + "`@auto go`" + ` without flags: use the default subagent pipeline, but if runtime policy blocks implicit spawning, ask before proceeding
- ` + "`@auto go --auto`" + `: treat this as explicit approval to run the default subagent pipeline without an extra confirmation round
- ` + "`@auto go --solo`" + `: disable subagents and stay in the main session
- ` + "`@auto go --team`" + `: keep compatibility with future native multi-agent naming, but continue with the default subagent pipeline for now

## Why This Is Reserved

- Codex already supports subagents natively via ` + "`spawn_agent(...)`" + `
- Public Codex docs do not define a separate local CLI Team API equivalent to Claude Code Agent Teams
- Overloading ` + "`--team`" + ` to mean "extra ` + "`spawn_agent(...)`" + ` fan-out" would conflict with the likely future meaning of native multi-agent support

## What To Use Instead

- Use ` + "`.codex/skills/agent-pipeline.md`" + ` for the default execution model
- Use ` + "`.codex/agents/*.toml`" + ` as the role source of truth for spawned workers
- Use ` + "`.codex/skills/worktree-isolation.md`" + ` when parallel ownership boundaries are explicit

## Revisit Condition

Enable a real ` + "`--team`" + ` route only when Codex exposes a documented native multi-agent surface that is distinct from ordinary subagent spawning.
	`
}
