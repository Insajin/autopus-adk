package codex

func codexAgentPipelineSkillBodyCompletion() string {
	return `
## Parallelism Rules

| Condition | Execution |
|----------|-----------|
| Non-overlapping ownership | Parallel workers allowed |
| Shared file or same migration directory | Sequential execution |
| Order dependency between tasks | Sequential execution |
| One worker blocked on another's output | Wait, integrate, then continue |

## Retry Policy

- Validation: up to 3 retries, or 5 with ` + "`--loop`" + `
- Review verification: up to 2 retries, or 3 with ` + "`--loop`" + `
- Repeated worker failure: shrink scope or fall back to the main session
- Repeated unchanged review finding: stop and surface the blocker instead of rediscovering the whole patch
- While review retries remain, unresolved findings are not a terminal handoff. Do not suggest ` + "`@auto go --continue`" + ` or manual review yet.

## Result Integration

Each worker should return:

- owned paths
- changed files
- verification run
- blockers or assumptions
- next required step

The main session owns final integration, status updates, and the decision to continue, retry, or stop.

### Sync Readiness Gate

Before the terminal ` + "`@auto sync`" + ` handoff, build a sync handoff package. Do not assume sync will discover remaining implementation work.

Required fields:

- ` + "`completion_verdict_preview`" + `: Outcome Lock, mandatory requirements, Must acceptance, Completion Debt, and Evolution Ideas summary using the same shape as sync's Completion Verdict
- ` + "`sync_ready`" + `: ` + "`yes`" + ` only when Outcome Lock is satisfied, all mandatory requirements and Must acceptance are met, and Completion Debt is ` + "`none`" + `
- ` + "`sync_blockers`" + `: ` + "`none`" + ` or concrete blockers that prevent setting the SPEC to ` + "`implemented`" + `
- ` + "`spec_status_after_go`" + `: ` + "`implemented`" + ` on success; do not use ` + "`done`" + ` or ` + "`completed`" + ` because ` + "`completed`" + ` is reserved for ` + "`@auto sync`" + `
- ` + "`sync_evidence_refs`" + `: changed files, verification commands, review verdict, @AX annotation result or ` + "`@AX: no-op`" + `

If ` + "`sync_ready`" + ` is not ` + "`yes`" + `, stop before the workflow lifecycle bar and report the blocker. Only set the SPEC status to ` + "`implemented`" + ` and hand off to ` + "`@auto sync`" + ` when the implementation scope is closed.

## Pre-Completion Checklist

Before you stop, ensure:

- the next required step is either complete or explicitly blocked
- validation status is known
- open review findings are either resolved or explicitly carried forward
- terminal handoff is used only after the final review outcome is known
- Sync Readiness Gate passed with ` + "`completion_verdict_preview`" + `, ` + "`sync_ready`" + `, ` + "`sync_blockers`" + `, and ` + "`spec_status_after_go`" + ` recorded
- the final response names the changed scope, verification, and any unresolved blockers
`
}
