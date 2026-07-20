// Package pipeline provides pipeline state management types and persistence.
package pipeline

import "strings"

// GateType identifies the kind of quality gate applied after a phase.
type GateType string

const (
	// GateNone applies no quality gate — the phase always passes.
	GateNone GateType = "none"
	// GateValidation checks for PASS/FAIL markers in the output.
	GateValidation GateType = "validation"
	// GateReview checks for APPROVE/REQUEST_CHANGES markers in the output.
	GateReview GateType = "review"
)

// GateVerdict constants for phase gate evaluation results.
const (
	// VerdictPass indicates the phase output passed the quality gate.
	VerdictPass GateVerdict = "pass"
	// VerdictFail indicates the phase output failed the quality gate.
	VerdictFail GateVerdict = "fail"
)

// @AX:ANCHOR: [AUTO] cross-cutting concern — gate evaluation consumed by sequential runner, parallel runner, and tests (fan-in >= 3)
// @AX:NOTE: [AUTO] magic constants — typed VERDICT values are the phase boundary contract
// EvaluateGate evaluates the phase output against the given gate type and
// returns VerdictPass or VerdictFail.
func EvaluateGate(gate GateType, output string) GateVerdict {
	switch gate {
	case GateNone, GateType(""):
		return VerdictPass
	case GateValidation:
		if verdict, ok := exactTypedVerdict(output, "PASS", "FAIL"); ok && verdict == "PASS" {
			return VerdictPass
		}
		return VerdictFail
	case GateReview:
		if verdict, ok := exactTypedVerdict(output, "APPROVE", "REQUEST_CHANGES"); ok && verdict == "APPROVE" {
			return VerdictPass
		}
		return VerdictFail
	default:
		return VerdictFail
	}
}

// exactTypedVerdict accepts exactly one line of the form "VERDICT: <value>".
// Additional prose is allowed, but duplicate, conflicting, or malformed verdict
// declarations fail closed.
func exactTypedVerdict(output string, allowed ...string) (string, bool) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}

	var found string
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "VERDICT") {
			continue
		}
		if !strings.HasPrefix(line, "VERDICT:") || found != "" {
			return "", false
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "VERDICT:"))
		if _, ok := allowedSet[value]; !ok {
			return "", false
		}
		found = value
	}
	return found, found != ""
}
