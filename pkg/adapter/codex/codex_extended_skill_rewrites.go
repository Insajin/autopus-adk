package codex

import "strings"

func normalizeCodexExtendedSkill(name, body string) string {
	switch name {
	case "agent-teams":
		return strings.TrimSpace(codexAgentTeamsSkillBody()) + "\n"
	case "agent-pipeline":
		return strings.TrimSpace(codexAgentPipelineSkillBody()) + "\n"
	case "worktree-isolation":
		return strings.TrimSpace(codexWorktreeIsolationSkillBody()) + "\n"
	case "subagent-dev":
		return strings.TrimSpace(codexSubagentDevSkillBody()) + "\n"
	case "prd":
		return strings.TrimSpace(rewriteCodexPRDSkillBody(body)) + "\n"
	default:
		return body
	}
}

func rewriteCodexPRDSkillBody(body string) string {
	body = strings.ReplaceAll(
		body,
		"PRD 작성 전에 6개 핵심 질문으로 컨텍스트를 수집합니다. 사용자 입력이 불충분할 경우 AskUserQuestion으로 확인:",
		"PRD 작성 전에 6개 핵심 질문으로 컨텍스트를 수집합니다. 사용자 입력이 불충분하면 active tool list에 있는 Codex `request_user_input`을 반드시 사용하고, 없을 때만 메인 세션이 짧은 plain-text 질문으로 직접 확인합니다:",
	)
	body = strings.ReplaceAll(body, "AskUserQuestion", "Codex `request_user_input` when available, otherwise a short plain-text question")
	return body
}
