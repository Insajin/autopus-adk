package codex

import "strings"

func normalizeCodexStandaloneToolNames(body string) string {
	return strings.NewReplacer(
		"AskUserQuestion", "request_user_input",
		"TeamCreate", "spawn_agent",
		"TeamDelete", "interrupt_agent",
		"SendMessage", "send_message",
	).Replace(body)
}
