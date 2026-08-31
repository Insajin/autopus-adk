package run

import "strings"

// guiMutatingActions is the exact set of Playwright methods the injected guard
// patches, and therefore the only action names it can block by name.
//
// It lives in Go so the evidence layer can tell a caller which declared
// forbidden actions the runtime is unable to enforce. TestGUIGuardScriptCovers
// MutatingActions pins it against the embedded script so the two cannot drift.
func guiMutatingActions() []string {
	return []string{
		"click", "dblclick", "tap", "fill", "press",
		"check", "uncheck", "selectoption", "setinputfiles", "dragto",
	}
}

// guiMutationActionClass is the one policy label with real enforcement power: it
// expands to every method in guiMutatingActions.
const guiMutationActionClass = "mutation"

// unenforceableForbiddenActions reports declared forbidden actions the runtime
// cannot act on.
//
// The preload observes Playwright method names, never business intent, so a label
// like `payment` or `email_send` blocks nothing. Silently accepting it would let a
// pack advertise a guarantee the harness does not provide, which is the same
// failure mode as an artifact kind with no producer. This does not block the
// journey — an existing pack keeps running — but the gap becomes visible in the
// check evidence and therefore in the rendered report.
func unenforceableForbiddenActions(declared []string) []string {
	enforceable := map[string]bool{guiMutationActionClass: true}
	for _, action := range guiMutatingActions() {
		enforceable[action] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, action := range declared {
		name := strings.ToLower(strings.TrimSpace(action))
		if name == "" || enforceable[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
