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
// @AX:NOTE: [AUTO] magic constants — "PASS", "APPROVE" token matching is implicit AI output contract
// EvaluateGate evaluates the phase output against the given gate type and
// returns VerdictPass or VerdictFail.
func EvaluateGate(gate GateType, output string) GateVerdict {
	switch gate {
	case GateNone:
		return VerdictPass

	case GateValidation:
		// Uppercase PASS token required; lowercase "pass" is not sufficient.
		if strings.Contains(output, "PASS") {
			return VerdictPass
		}
		return VerdictFail

	case GateReview:
		// APPROVE or APPROVED signals acceptance.
		if strings.Contains(output, "APPROVE") {
			return VerdictPass
		}
		return VerdictFail
	}

	// Unknown gate type defaults to pass to avoid blocking the pipeline.
	return VerdictPass
}
