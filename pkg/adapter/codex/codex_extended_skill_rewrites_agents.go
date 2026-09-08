package codex

import "strconv"

// @AX:NOTE [AUTO]: the six-tool Multi-Agent V2 surface is a contract, not a release note; update it with native collaboration schema changes.
func codexAgentTeamsSkillBody(spawnedWorkerLimit int) string {
	limit := strconv.Itoa(spawnedWorkerLimit)
	return `
# Codex Multi-Agent V2 Team Skill

## Activation

Use ` + "`@auto go SPEC-ID --team`" + ` or ` + "`$codex-auto-go SPEC-ID --team`" + `.

## Concurrency Contract

Team mode needs ` + "`[features.multi_agent_v2]`" + ` with ` + "`enabled = true`" + `.
The worker ceiling is separate from that switch and comes from
` + "`codex.agents.max_concurrent_threads`" + ` in ` + "`autopus.yaml`" + `, which this
project sets to **` + limit + `**. Autopus writes it into the project
` + "`.codex/config.toml`" + ` as the documented ` + "`[agents]`" + ` table key.

Count semantics: the value bounds **spawned workers only**. The coordinator is
the primary thread and is not counted, so ` + limit + ` workers means up to
` + strconv.Itoa(spawnedWorkerLimit+1) + ` agents including this session.

A configured value is a request, not observed capacity:

- Provider, account, and host limits win over it.
- A config change applies to a **new** session; never interrupt live workers to
  apply one.
- Never infer available capacity from a successful config write. Ask
  ` + "`list_agents()`" + ` what actually exists, and run
  ` + "`auto doctor`" + ` for the requested/loaded/effective breakdown.
- Local resource-heavy test and build jobs are **not** governed by this value.
  Neither is ` + "`orchestra.subprocess.max_concurrent`" + `, which bounds Autopus
  provider subprocesses only. Bound heavy verification yourself, well below the
  worker count, from measured memory and queue time.

The current V2 collaboration surface has exactly six tools:

- ` + "`spawn_agent(task_name, message, ...)`" + ` starts a worker with an explicit role and task.
- ` + "`send_message(...)`" + ` sends coordination without changing the assigned task.
- ` + "`followup_task(...)`" + ` gives a completed or idle worker additional work.
- ` + "`wait_agent()`" + ` waits without a target and returns the next available event.
- ` + "`interrupt_agent(...)`" + ` stops work that is unsafe, obsolete, or blocked.
- ` + "`list_agents()`" + ` inspects current worker state.

Do not use legacy collaboration names or invent lifecycle tools.

## Shared Workspace

All workers use the same shared cwd and filesystem. ` + "`fork_turns`" + ` changes
conversation context only; it does not create another filesystem, worktree, or
branch. Before parallel dispatch, assign disjoint write ownership. If ownership
overlaps, run the writers sequentially.

The main session is the Lead. Builders implement within their owned paths and
Guardians verify after the write phase. Never spawn another Lead.

` + "```python" + `
spawn_agent(
    task_name="builder",
    message="""
    Role: Builder.
    Shared cwd/filesystem: edit only the disjoint owned paths below.
    Own only: <paths>.
    Do not edit: <paths>.
    Return exactly: owned_paths, changed_files, verification, blockers,
    next_required_step.
    """,
)
` + "```" + `

Use ` + "`send_message(...)`" + ` for clarification, ` + "`followup_task(...)`" + `
for a new task, target-less ` + "`wait_agent()`" + ` for the next event,
` + "`list_agents()`" + ` for status, and ` + "`interrupt_agent(...)`" + ` only
when work must stop.

## Completion Receipt

Every worker returns exactly these five fields:
` + "`owned_paths`" + `, ` + "`changed_files`" + `, ` + "`verification`" + `,
` + "`blockers`" + `, and ` + "`next_required_step`" + `.
`
}
