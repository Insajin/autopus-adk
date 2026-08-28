package claude

import "strings"

// stickyHookInvocation is the stable part of the command the sticky compiler
// emits, so a previously installed entry stays recognizable across a flag
// change and is never orphaned by one.
const stickyHookInvocation = "auto rules sticky"

// isStickyCommand reports whether one settings command is autopus invoking its
// own sticky runtime, as opposed to a user command that merely names it.
//
// The invocation has to be anchored at the start of the command and followed by
// a word boundary. A bare substring test claims any user hook that mentions the
// command anywhere — a DLP or secret-scanning hook naming it in an exclusion
// argument is the realistic case — and regeneration then silently deletes that
// hook, which is precisely the user-configuration loss entry-level ownership
// exists to prevent. Equality with the exact emitted command is too narrow in
// the other direction: an entry installed by an earlier generation carries
// different flags and must still be retracted rather than left orphaned.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the ownership predicate for a shared, user-owned settings file — it decides which entries regeneration may delete.
// @AX:REASON: Widening this to a substring match makes autopus delete third-party hooks that reference the command; narrowing it to equality orphans every previously installed entry whose flags have changed.
func isStickyCommand(command string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(command), stickyHookInvocation)
	if !ok {
		return false
	}
	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")
}

// appendHookEntry adds one autopus entry to an event key while preserving a
// value this writer does not understand.
//
// settings.json is user-owned and hand-editable, so UserPromptSubmit can decode
// as a single object or a bare string rather than the array Claude Code expects.
// A type assertion to []any yields nil for those, and assigning over the result
// destroys the user's value. Carrying it alongside the new entry keeps both
// halves: autopus still registers its hook, and nothing the user wrote is lost
// to a shape this code did not anticipate.
func appendHookEntry(existing any, entry map[string]any) []any {
	switch typed := existing.(type) {
	case nil:
		return []any{entry}
	case []any:
		return append(typed, entry)
	default:
		return []any{typed, entry}
	}
}

// entryHookCommands collects the command of every hook inside one entry,
// accepting both the decoded settings.json shape and the shape this package
// builds before serialization.
func entryHookCommands(raw any) []string {
	var hooks []any
	switch typed := raw.(type) {
	case []any:
		hooks = typed
	case []map[string]any:
		for _, hook := range typed {
			hooks = append(hooks, hook)
		}
	default:
		return nil
	}

	commands := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		obj, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if command, ok := obj["command"].(string); ok {
			commands = append(commands, command)
		}
	}
	return commands
}
