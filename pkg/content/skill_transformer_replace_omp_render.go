package content

import (
	"strconv"
	"strings"
)

func parseOMPLegacyCalls(body string) []ompLegacyDispatch {
	calls := ompLegacyCallRe.FindAllStringSubmatch(body, -1)
	dispatches := make([]ompLegacyDispatch, 0, len(calls)+1)
	for _, call := range calls {
		if len(call) < 2 {
			continue
		}
		dispatches = append(dispatches, ompLegacyDispatch{
			agent:    firstOMPFieldValue(ompLegacyRoleRe.FindStringSubmatch(call[1])),
			task:     firstOMPFieldValue(ompLegacyTaskRe.FindStringSubmatch(call[1])),
			isolated: ompIsolationRe.MatchString(call[1]),
		})
	}

	spawns := ompSpawnStartRe.FindAllStringSubmatchIndex(body, -1)
	for i, match := range spawns {
		end := len(body)
		if i+1 < len(spawns) {
			end = spawns[i+1][0]
		}
		segment := body[match[0]:end]
		agent := ""
		if match[2] >= 0 && match[3] >= 0 {
			agent = body[match[2]:match[3]]
		}
		dispatches = append(dispatches, ompLegacyDispatch{
			agent:    agent,
			task:     firstOMPFieldValue(ompSpawnTaskRe.FindStringSubmatch(segment)),
			isolated: ompIsolationRe.MatchString(segment),
		})
	}

	if len(dispatches) == 0 {
		dispatches = append(dispatches, ompLegacyDispatch{})
	}
	return dispatches
}

func firstOMPFieldValue(match []string) string {
	if len(match) < 2 {
		return ""
	}
	for _, value := range match[1:] {
		if value != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func renderOMPTaskBatch(dispatches []ompLegacyDispatch) string {
	var b strings.Builder
	b.WriteString("```json\n{\n")
	b.WriteString("  \"i\": \"Dispatching bounded OMP work\",\n")
	b.WriteString("  \"context\": \"Shared goal, constraints, owned-path boundaries, and cross-task contracts.\",\n")
	b.WriteString("  \"tasks\": [\n")
	for i, dispatch := range dispatches {
		agent := strings.TrimSpace(dispatch.agent)
		defaultAgent := agent == "" || agent == "task"
		name := agent
		if defaultAgent {
			name = "Worker"
		}
		taskText := strings.TrimSpace(dispatch.task)
		if taskText == "" {
			taskText = "Complete the assigned work and return the required receipt."
		}
		if len(dispatches) > 1 {
			name += "-" + strconv.Itoa(i+1)
		}
		b.WriteString("    {\n")
		b.WriteString("      \"name\": " + strconv.Quote(name) + ",\n")
		if !defaultAgent {
			b.WriteString("      \"agent\": " + strconv.Quote(agent) + ",\n")
		}
		b.WriteString("      \"task\": " + strconv.Quote(taskText) + ",\n")
		writeOMPReceiptSchema(&b, "      ")
		b.WriteString(",\n      \"schemaMode\": \"strict\"")
		b.WriteString("\n")
		b.WriteString("    }")
		if i+1 < len(dispatches) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ]\n}\n```")
	return b.String()
}

func writeOMPReceiptSchema(b *strings.Builder, indent string) {
	b.WriteString(indent + "\"outputSchema\": {\n")
	b.WriteString(indent + "  \"type\": \"object\",\n")
	b.WriteString(indent + "  \"additionalProperties\": false,\n")
	b.WriteString(indent + "  \"required\": [\"owned_paths\", \"changed_files\", \"verification\", \"blockers\", \"next_required_step\"],\n")
	b.WriteString(indent + "  \"properties\": {\n")
	b.WriteString(indent + "    \"owned_paths\": {\"type\": \"array\", \"items\": {\"type\": \"string\"}},\n")
	b.WriteString(indent + "    \"changed_files\": {\"type\": \"array\", \"items\": {\"type\": \"string\"}},\n")
	b.WriteString(indent + "    \"verification\": {\"type\": \"array\", \"items\": {\"type\": \"string\"}},\n")
	b.WriteString(indent + "    \"blockers\": {\"type\": \"array\", \"items\": {\"type\": \"string\"}},\n")
	b.WriteString(indent + "    \"next_required_step\": {\"type\": \"string\"}\n")
	b.WriteString(indent + "  }\n")
	b.WriteString(indent + "}")
}

func appendOMPCoordinationContract(body string) string {
	if strings.Contains(body, "## OMP Coordination Contract") {
		return body
	}
	contract := strings.Join([]string{
		"## OMP Coordination Contract",
		"",
		"### Ownership gate",
		"",
		"- Choose exactly one topology before dispatch: `OMP-local` or `Orca-supervised`.",
		"- `OMP-local` is the default. The current OMP session is the sole DAG owner and uses its native `task`, `hub`, and `todo` tools.",
		"- `Orca-supervised` is allowed only when the user explicitly selects a supervised or durable topology. Before any Orca orchestration, run and read `orca skills get orchestration --full`.",
		"- The single DAG owner invariant is mandatory: when Orca owns the DAG, the OMP session does not dispatch a competing DAG; when OMP owns it, Orca does not dispatch one.",
		"",
		"### Native field contracts",
		"",
		renderOMPTaskBatch([]ompLegacyDispatch{{
			task:     "Complete one self-contained assignment and return only the required receipt.",
			isolated: true,
		}}),
		"",
		"- Inspect the current dynamic `task` schema before dispatch. Use the shown batch shape only when it exposes top-level `context` and `tasks`; otherwise use the discovered flat shape and place shared context in `local://`.",
		"- Every model-authored `task`, `hub`, and `todo` call includes a concise top-level `i` while `tools.intentTracing` is enabled.",
		"- Every `tasks` item uses `name` when a stable agent id is useful and carries per-item `task`, `outputSchema`, and `schemaMode`. Set `agent` only to select a custom agent type; omit it for OMP's default general worker.",
		"- `isolated` and `effort` are conditional dynamic fields. Add `isolated` or `effort` only after the current schema exposes that exact field; otherwise omit it.",
		"- `outputSchema` is the strict five-field receipt JSON Schema shown in the normalized batch: `owned_paths`, `changed_files`, `verification`, `blockers`, and `next_required_step`.",
		"- Retain the agent id returned by `task`. For a non-isolated or otherwise revivable worker, every follow-up goes to that same id with `hub` send fields `{\"i\":\"Following up with an existing worker\",\"op\":\"send\",\"to\":\"<same agent id>\",\"message\":\"<follow-up>\"}`; do not create a replacement merely to continue revivable work.",
		"- An isolated worker is terminal after workspace cleanup and cannot be revived. A correction is a new explicitly named `task` item with freshly declared ownership and context, not a `hub` send to the terminal agent id.",
		"- The parent OMP session owns progress. A `todo` call contains one top-level operation and intent: initialize with `{\"i\":\"Updating parent-owned progress\",\"op\":\"init\",\"list\":[{\"phase\":\"Implementation\",\"items\":[\"...\"]}]}`, advance with `{\"i\":\"Updating parent-owned progress\",\"op\":\"start\",\"task\":\"<exact task content>\"}`, complete with `{\"i\":\"Updating parent-owned progress\",\"op\":\"done\",\"task\":\"<exact task content>\"}`, and block with `{\"i\":\"Updating parent-owned progress\",\"op\":\"block\",\"task\":\"<exact task content>\",\"reason\":\"<reason>\"}`.",
	}, "\n")
	return strings.TrimRight(body, "\n") + "\n\n" + contract + "\n"
}
