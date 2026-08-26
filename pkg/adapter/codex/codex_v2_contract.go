package codex

import "strings"

const codexV2ContractHeading = "## Codex Multi-Agent V2 Contract"

func ensureCodexV2WorkflowContract(body string) string {
	if strings.Contains(body, codexV2ContractHeading) {
		return body
	}
	contract := `
+## Codex Multi-Agent V2 Contract
+
+When delegating, use only ` + "`spawn_agent(task_name, message, ...)`" + `,
+` + "`send_message(...)`" + `, ` + "`followup_task(...)`" + `, target-less
+` + "`wait_agent()`" + `, ` + "`interrupt_agent(...)`" + `, and
+` + "`list_agents()`" + `. All workers use the same shared cwd and filesystem.
+Parallel writers require disjoint write ownership; otherwise run them
+sequentially. Every worker returns exactly ` + "`owned_paths`" + `,
+` + "`changed_files`" + `, ` + "`verification`" + `, ` + "`blockers`" + `, and
+` + "`next_required_step`" + `.
+`
	contract = strings.ReplaceAll(contract, "\n+", "\n")
	return injectAfterFirstHeading(body, strings.TrimSpace(contract))
}
