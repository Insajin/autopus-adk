package codex

import "strings"

func normalizeCodexStandaloneToolNames(body string) string {
	return strings.NewReplacer(
		"AskUserQuestion", "request_user_input",
		"TeamCreate", "spawn_agent",
		"TeamDelete", "close_agent",
		"SendMessage", "send_input",
	).Replace(body)
}
