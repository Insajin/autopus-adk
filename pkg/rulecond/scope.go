package rulecond

import (
	"fmt"
	"strings"
)

// Hook event and Claude Code tool matchers a scope compiles to.
const (
	// EventPreToolUse is the only hook event a `condition` scope maps to.
	EventPreToolUse = "PreToolUse"
	// MatcherBash matches the Bash tool.
	MatcherBash = "Bash"
	// MatcherEdit matches every file-editing tool, whose condition subject is
	// `tool_input.file_path`.
	MatcherEdit = "Edit|Write|MultiEdit"
)

// scopePrefix is the only scope family the ADK compiles. A `file:` scope is an
// omp path scope and belongs in `globs`, so it is rejected rather than guessed.
const scopePrefix = "tool:"

// Trigger is a parsed `scope` value: the hook event plus the Claude Code tool
// matcher a rule fires on, and the optional file glob the scope carried.
type Trigger struct {
	// Event is the hook event name.
	Event string
	// Matcher is the Claude Code tool matcher regex.
	Matcher string
	// Glob is the optional file pattern from `tool:edit(<glob>)`. It is
	// recorded, not evaluated: the condition regex remains the match oracle.
	Glob string
}

// ParseScope parses one `scope` entry of the form `tool:bash` or
// `tool:edit(<glob>)` into a hook event plus a Claude Code tool matcher.
func ParseScope(scope string) (Trigger, error) {
	value := strings.TrimSpace(scope)
	if value == "" {
		return Trigger{}, fmt.Errorf("scope is empty")
	}
	if !strings.HasPrefix(value, scopePrefix) {
		return Trigger{}, fmt.Errorf("scope %q is not a tool scope; a file pattern belongs in globs", value)
	}

	body := strings.TrimPrefix(value, scopePrefix)
	glob := ""
	if open := strings.Index(body, "("); open >= 0 {
		if !strings.HasSuffix(body, ")") {
			return Trigger{}, fmt.Errorf("scope %q has an unterminated tool argument", value)
		}
		glob = body[open+1 : len(body)-1]
		body = body[:open]
	}

	matcher, err := toolMatcher(strings.TrimSpace(body))
	if err != nil {
		return Trigger{}, err
	}
	return Trigger{Event: EventPreToolUse, Matcher: matcher, Glob: strings.TrimSpace(glob)}, nil
}

// toolMatcher maps an omp tool name onto a Claude Code tool matcher.
func toolMatcher(tool string) (string, error) {
	switch strings.ToLower(tool) {
	case "bash", "shell", "command":
		return MatcherBash, nil
	case "edit", "write", "multiedit":
		return MatcherEdit, nil
	default:
		return "", fmt.Errorf("unknown tool scope %q", tool)
	}
}

// ConditionSubject derives the single string a rule's condition regexes are
// matched against for one tool call (REQ-CONDRULE-FIRE-02). Bash uses
// `tool_input.command`, the file-editing tools use `tool_input.file_path`, and
// every other tool has no subject, which makes it a non-match.
func ConditionSubject(tool string, input map[string]any) (string, bool) {
	var key string
	switch tool {
	case "Bash":
		key = "command"
	case "Edit", "Write", "MultiEdit":
		key = "file_path"
	default:
		return "", false
	}

	value, ok := input[key].(string)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}
