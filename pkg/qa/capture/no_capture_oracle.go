package capture

import (
	"fmt"
	"strings"
)

// Verify capture modes, mirroring config.VerifyConf.Capture. They select which
// UX oracle a verification run is allowed to lean on: pixels, or the semantic
// observations below.
const (
	VerifyCaptureScreenshot = "screenshot"
	VerifyCaptureNoCapture  = "no-capture"
)

// No-capture oracle kinds. A run without screenshots still has to answer every
// question a screenshot answered, so each kind replaces one of them: DOM
// geometry answers "does anything overlap, overflow, or leave the viewport",
// the accessibility tree answers "is the control present with a usable role and
// name", keyboard navigation answers "can it be reached and is focus visible",
// and state transition answers "did the screen actually change when it should".
//
// Without this set, `no-capture` would mean "verify nothing and report PASS",
// which is worse than taking no screenshot at all.
const (
	OracleDOMGeometry        = "dom_geometry"
	OracleAccessibilityTree  = "accessibility_tree"
	OracleKeyboardNavigation = "keyboard_navigation"
	OracleStateTransition    = "state_transition"
)

// NoCaptureOracleKinds is the required set in the order conformance reports a
// gap. The order is fixed so a producer closing one gap at a time always sees
// the same next missing kind instead of a set that reshuffles per run.
var NoCaptureOracleKinds = []string{
	OracleDOMGeometry,
	OracleAccessibilityTree,
	OracleKeyboardNavigation,
	OracleStateTransition,
}

// MaxOracleEntries caps the oracle list the same way MaxMediaEntries caps
// media: a producer may report one oracle per step and kind, not an unbounded
// stream.
const MaxOracleEntries = 200

// Oracle is one semantic UX observation a no-capture run reports instead of a
// screenshot. It is deliberately body-free - a kind, an optional step, and
// counters - so the DOM, the accessibility tree, and the assertion text never
// reach published evidence and the receipt needs no redaction pass.
type Oracle struct {
	Kind       string `json:"kind"`
	StepID     string `json:"step_id,omitempty"`
	Assertions int    `json:"assertions,omitempty"`
	Findings   int    `json:"findings,omitempty"`
}

// RequiresNoCaptureOracles reports whether the policy traded pixels for the
// semantic oracle set. Enabled() stays false in that case because no media
// stream is expected, so the two predicates answer different questions and a
// caller must not substitute one for the other.
func (p Policy) RequiresNoCaptureOracles() bool {
	return p.Declared() && p.Mode == ModeOff
}

// PolicyFromVerifyConfig derives the capture policy a verification run should
// enforce from `verify.capture`. `no-capture` becomes mode off, which is what
// makes the semantic oracle set mandatory; anything else keeps the screenshot
// contract.
func PolicyFromVerifyConfig(capture string) Policy {
	if strings.TrimSpace(capture) == VerifyCaptureNoCapture {
		return Policy{Mode: ModeOff}
	}
	return Policy{
		Mode:        ModeAlways,
		Streams:     []string{StreamScreenshot},
		Screenshot:  ScreenshotPerStep,
		RetainLocal: true,
	}
}

// conformNoCaptureOracles enforces the substitute contract: all four oracles,
// and no screenshot smuggled in behind a policy that forbids them.
//
// Only the first missing kind is reported. The fixed order plus a single
// finding gives the producer one unambiguous next step instead of a four-line
// wall it has to re-derive an order from.
func conformNoCaptureOracles(index Index, policy Policy) []string {
	if !policy.RequiresNoCaptureOracles() {
		return nil
	}
	present := make(map[string]bool, len(index.Oracles))
	for _, oracle := range index.Oracles {
		present[oracle.Kind] = true
	}
	for _, kind := range NoCaptureOracleKinds {
		if !present[kind] {
			return []string{"missing_no_capture_oracle:" + kind}
		}
	}
	var findings []string
	for _, step := range index.Steps {
		if step.Screenshot != nil {
			findings = append(findings, fmt.Sprintf(
				"no-capture mode forbids screenshots but step %q carries one", step.StepID))
		}
	}
	return findings
}

// validateOracles checks the oracle list against the contract independently of
// any policy, the way validateSteps does for steps.
func validateOracles(oracles []Oracle) error {
	if len(oracles) > MaxOracleEntries {
		return fmt.Errorf("oracles exceed %d entries", MaxOracleEntries)
	}
	for position, oracle := range oracles {
		if !validOracleKind(oracle.Kind) {
			return fmt.Errorf("oracles[%d].kind %q is unsupported", position, oracle.Kind)
		}
		if oracle.StepID != "" && !stepIDRe.MatchString(oracle.StepID) {
			return fmt.Errorf("oracles[%d].step_id must match %s", position, stepIDRe)
		}
		if oracle.Assertions < 0 || oracle.Findings < 0 {
			return fmt.Errorf("oracles[%d] counters must not be negative", position)
		}
	}
	return nil
}

func validOracleKind(kind string) bool {
	for _, known := range NoCaptureOracleKinds {
		if known == kind {
			return true
		}
	}
	return false
}
