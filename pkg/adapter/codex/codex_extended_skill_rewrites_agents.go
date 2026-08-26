package codex

// @AX:NOTE [AUTO]: the hardcoded 0.149.1 contract records the exact six-tool Multi-Agent V2 surface; update it with native collaboration schema changes.
func codexAgentTeamsSkillBody() string {
	return `
# Codex Multi-Agent V2 Team Skill

## Activation

Use ` + "`@auto go SPEC-ID --team`" + ` or ` + "`$codex-auto-go SPEC-ID --team`" + `.

## Runtime Contract

Codex 0.149.1 team mode requires ` + "`[features.multi_agent_v2]`" + ` with
` + "`enabled = true`" + ` and ` + "`max_concurrent_threads_per_session = 4`" + `.
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
