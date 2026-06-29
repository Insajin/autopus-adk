package workflow

import "fmt"

// DepthProfile is the bounded per-phase execution depth derived from a quality
// tier. It governs how many verify votes and fan-out branches a phase may use,
// whether a synthesis pass runs, and the retry budget. All values are capped
// (REQ-004, S4) so a quality tier can never request unbounded depth.
type DepthProfile struct {
	VerifyVotes int
	FanOutCap   int
	Synthesis   bool
	Retry       int
}

// Depth caps. These are hard ceilings enforced at the parse boundary; values
// above them are REJECTED (fail-closed), never silently clamped.
const (
	MaxVerifyVotes = 3
	MaxFanOut      = 5
	MaxRetry       = 3
	// MaxPhases bounds the declared phase count. The team workflow generator
	// labels segments with string(rune('A'+segmentIndex)); since segments never
	// exceed phases, capping phases at 26 keeps every segment label within the
	// safe A..Z charset — closing the one generated-JS token not otherwise behind
	// a charset whitelist (defense-in-depth at the SoT parse boundary).
	MaxPhases = 26
)

// ResolveDepth maps a quality tier to a bounded DepthProfile. "ultra" runs the
// deepest allowed profile (max verify votes, full fan-out, synthesis on);
// "balanced" and any other/unknown value fall back to the conservative default.
func ResolveDepth(quality string) DepthProfile {
	switch quality {
	case "ultra":
		return DepthProfile{VerifyVotes: 3, FanOutCap: MaxFanOut, Synthesis: true}
	default:
		// "balanced" and any unrecognized tier resolve to the safe default.
		return DepthProfile{VerifyVotes: 1, FanOutCap: MaxFanOut, Synthesis: false}
	}
}

// validateDepthCaps fails closed when a phase declares depth above the hard
// ceilings. It names the offending field so the parse error is actionable. A
// nil return means every value is within bounds.
func validateDepthCaps(phaseID string, verifyVotes, fanOutCap, retry int) error {
	if verifyVotes > MaxVerifyVotes {
		return fmt.Errorf("parse workflow schema: phase %q verify_votes %d exceeds cap %d", phaseID, verifyVotes, MaxVerifyVotes)
	}
	if fanOutCap > MaxFanOut {
		return fmt.Errorf("parse workflow schema: phase %q fan_out_cap %d exceeds cap %d", phaseID, fanOutCap, MaxFanOut)
	}
	if retry > MaxRetry {
		return fmt.Errorf("parse workflow schema: phase %q retry %d exceeds cap %d", phaseID, retry, MaxRetry)
	}
	return nil
}
