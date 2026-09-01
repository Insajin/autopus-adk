package run

import (
	"fmt"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/adapter"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qalane "github.com/insajin/autopus-adk/pkg/qa/lane"
)

// LaneErrorCode is the stable error code for a --lane value the harness cannot
// honour. The CLI envelope carries a per-command code, so the message repeats
// this one to keep it machine-greppable.
const LaneErrorCode = "qa_run_lane_unknown"

// LaneError reports a lane name that neither the release gate nor the project
// declares. It is an error rather than a fallback because auto-detected
// journeys would otherwise run a project's real test suite and report a pass
// for a lane that does not exist.
type LaneError struct {
	Code  string
	Lane  string
	Known []string
}

func (e *LaneError) Error() string {
	return fmt.Sprintf(
		"%s: unknown lane %q; valid lanes: %s",
		e.Code, e.Lane, strings.Join(e.Known, ", "),
	)
}

// validateLane rejects a lane the harness has no definition for. Valid means a
// canonical release lane, a lane a shipped adapter declares, or a lane at least
// one project Journey Pack declares. Matching is case-insensitive to stay
// consistent with journey.HasLane.
func validateLane(lane string, packs []journey.Pack) error {
	known := knownLanes(packs)
	for _, candidate := range known {
		if strings.EqualFold(candidate, lane) {
			return nil
		}
	}
	return &LaneError{Code: LaneErrorCode, Lane: lane, Known: known}
}

// knownLanes returns every lane the harness can be asked to run, sorted for a
// stable error message.
func knownLanes(packs []journey.Pack) []string {
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		seen[strings.ToLower(value)] = struct{}{}
	}
	for _, lane := range qalane.Release() {
		add(lane)
	}
	for _, metadata := range adapter.Registry() {
		for _, lane := range metadata.DefaultLanes {
			add(lane)
		}
	}
	for _, pack := range packs {
		for _, lane := range pack.Lanes {
			add(lane)
		}
	}
	lanes := make([]string, 0, len(seen))
	for lane := range seen {
		lanes = append(lanes, lane)
	}
	sort.Strings(lanes)
	return lanes
}

// laneHasExecutor reports whether anything can actually run the lane: either the
// harness ships an execution path, or the project declares a Journey Pack for
// it. A lane that fails both must not be satisfied by auto-detected journeys.
func laneHasExecutor(lane string, packs []journey.Pack) bool {
	if qalane.HasExecutor(lane) {
		return true
	}
	for _, pack := range packs {
		if journey.HasLane(pack, lane) {
			return true
		}
	}
	return false
}

// applyLaneWithoutExecutor drops every selection for a lane nothing implements
// and records why, so the run reports "warning" with a visible setup gap instead
// of "passed" with an empty check list.
func applyLaneWithoutExecutor(plan *Plan, lane string) {
	plan.SelectedJourneys = []string{}
	plan.SelectedAdapters = []string{}
	plan.ManifestOutputPreviewPaths = []string{}
	plan.ArtifactPreviewRefs = []ArtifactPreview{}
	plan.SetupGaps = append(plan.SetupGaps, SetupGap{
		Adapter: lane,
		Reason:  "no adapter implements the " + lane + " lane",
	})
}
