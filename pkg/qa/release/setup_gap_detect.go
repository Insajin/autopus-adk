package release

import (
	"strings"

	qalane "github.com/insajin/autopus-adk/pkg/qa/lane"
	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

// laneSetupGap classifies why a release lane has no executable Journey Pack.
//
// Deferred lanes are classified exactly like blocking lanes. Whether a gap
// BLOCKS the gate is the lane policy's decision downstream; hiding the gap row
// instead let a lane with genuinely unmet preconditions report verdict=pass and
// setup_gap_class=none, which is not the same thing as "does not block".
func laneSetupGap(projectDir, lane string) (SetupGapClass, string) {
	switch lane {
	case "canary-explicit":
		return SetupGapCanaryTemplate, "explicit safe canary command is required"
	case "mobile-readiness":
		return mobileReadinessSetupGap(projectDir)
	}
	if !qalane.HasExecutor(lane) {
		return SetupGapLaneNotImplemented, "no adapter implements the " + lane + " lane"
	}
	return SetupGapMissingJourneyPack, "project-local Journey Pack is required"
}

// mobileReadinessSetupGap names the readiness inputs the project is actually
// missing, so the gate reports the same reason codes a direct
// `auto qa run --lane mobile-readiness` fail-closes with.
func mobileReadinessSetupGap(projectDir string) (SetupGapClass, string) {
	readiness := mobile.Assess(projectDir)
	if len(readiness.SetupGaps) == 0 {
		return SetupGapMissingJourneyPack, "project-local mobile Journey Pack is required"
	}
	codes := make([]string, 0, len(readiness.SetupGaps))
	for _, gap := range readiness.SetupGaps {
		codes = append(codes, gap.ReasonCode)
	}
	return SetupGapEnvMissing, "mobile readiness incomplete: " + strings.Join(codes, ", ")
}
