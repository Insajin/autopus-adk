package workflow

// This file guards the JS-injection trust boundary (REQ-011, S6). Phase model
// and effort strings are interpolated into the generated workflow JS, so they
// are validated against closed whitelists at the parse boundary. Anything
// outside the whitelist is rejected fail-closed before it can reach the JS
// surface.

// safeAgentModels is the closed set of agent model identifiers allowed to be
// interpolated into generated workflow JS. Extend deliberately; never widen to
// free-form input.
var safeAgentModels = map[string]bool{
	"claude-opus-4-8":   true,
	"claude-opus-4-7":   true,
	"claude-sonnet-4-6": true,
	"claude-haiku-4-5":  true,
}

// isSafeAgentModel reports whether m is a whitelisted model. The empty string is
// allowed because deterministic gate phases carry no model.
func isSafeAgentModel(m string) bool {
	return m == "" || safeAgentModels[m]
}

// safeEfforts is the closed set of effort tiers allowed in generated JS.
var safeEfforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
	"max":    true,
}

// isSafeEffort reports whether e is a whitelisted effort tier. The empty string
// is allowed because phases without an agent carry no effort.
func isSafeEffort(e string) bool {
	return e == "" || safeEfforts[e]
}
