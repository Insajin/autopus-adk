package readiness

import "strings"

var supportedTargets = map[string]struct{}{
	"codex":    {},
	"claude":   {},
	"gemini":   {},
	"opencode": {},
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: repair actions only enable for supported targets with failed deterministic evidence that passed redaction.
func DeriveFeedbackAction(e EvidenceForFeedback, target string) FeedbackAction {
	target = strings.ToLower(strings.TrimSpace(target))
	action := FeedbackAction{
		Target:          target,
		Enabled:         false,
		ManifestPath:    e.ManifestPath,
		RedactionStatus: string(e.RedactionStatus),
	}
	if _, ok := supportedTargets[target]; !ok {
		action.DisabledReason = "unsupported_target"
		return action
	}
	if e.Status != StatusFailed {
		action.DisabledReason = "not_failed"
		return action
	}
	if !e.DeterministicAuthority {
		action.DisabledReason = "not_deterministic"
		return action
	}
	if e.RedactionStatus != RedactionPassed {
		action.DisabledReason = "redaction_failed"
		return action
	}
	if unsafeCommandArg(e.ManifestPath) {
		action.DisabledReason = "unsafe_manifest_path"
		return action
	}
	action.Enabled = true
	action.Command = []string{"auto", "qa", "feedback", "--to", target, "--evidence", e.ManifestPath}
	action.CommandDisplay = shellQuoteArgs(action.Command)
	return action
}

func unsafeCommandArg(value string) bool {
	return value == "" ||
		unsafeStringClass(value, "manifest_path") != "" ||
		strings.HasPrefix(value, "/") ||
		strings.Contains(value, "://") ||
		strings.Contains("/"+value+"/", "/../") ||
		strings.ContainsAny(value, "\x00\r\n\t:;&|$`<>?")
}

func shellQuoteArgs(args []string) string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, shellQuote(arg))
	}
	return strings.Join(out, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	for _, r := range arg {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_./:@", r)) {
			return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
	}
	return arg
}
