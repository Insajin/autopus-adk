package content

import "strings"

func translateHookCommand(command, event, platform string) string {
	if platform != "antigravity-cli" {
		return command
	}
	switch event {
	case "PreToolUse":
		return antigravityJSONHookCommand(command, `{"decision":"allow"}`)
	case "PostToolUse":
		return antigravityJSONHookCommand(command, `{}`)
	default:
		return command
	}
}

func antigravityJSONHookCommand(command, jsonPayload string) string {
	script := command + " >&2 || true; printf '" + jsonPayload + "\\n'"
	return "sh -c " + shellSingleQuote(script)
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
