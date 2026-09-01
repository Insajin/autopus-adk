package domainreadiness

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// Reasons a declared journey_pack_ref could not be honoured.
const (
	JourneyRefGapNotFound   = "journey_pack_not_found"
	JourneyRefGapUnverified = "journey_packs_unreadable"
)

// JourneyRefGap records one journey_pack_ref the compiler could not resolve to a
// Journey Pack the project actually declares.
//
// Nothing used to check these refs, so a catalog naming packs that were never
// generated still compiled to valid=true. The gap is deliberately a gap and not
// an error: the catalog is structurally sound, the project is merely missing the
// pack the scenario promised.
type JourneyRefGap struct {
	ScenarioID     string `json:"scenario_id"`
	JourneyPackRef string `json:"journey_pack_ref"`
	Reason         string `json:"reason"`
	Detail         string `json:"detail,omitempty"`
}

// journeyPackIndex is the project's declared Journey Packs.
//
// A load failure is carried rather than collapsed into an empty set: an
// unreadable pack directory cannot prove a ref is absent, and reporting
// "not found" from it would be the same unearned certainty this cross-check
// exists to remove.
type journeyPackIndex struct {
	packs []journey.Pack
	ids   map[string]struct{}
	err   error
}

func loadJourneyPackIndex(projectDir string) journeyPackIndex {
	packs, err := journey.LoadDir(projectDir)
	if err != nil {
		return journeyPackIndex{err: err}
	}
	ids := make(map[string]struct{}, len(packs))
	for _, pack := range packs {
		if id := strings.TrimSpace(pack.ID); id != "" {
			ids[id] = struct{}{}
		}
	}
	return journeyPackIndex{packs: packs, ids: ids}
}

// gapsFor reports one gap per ref of a single scenario that the project cannot
// back. A scenario declaring no refs yields none: silence asserts nothing, so
// there is nothing to falsify, and manufacturing a gap out of it would make
// every freshly initialized project report a defect it has no way to close.
func (index journeyPackIndex) gapsFor(scenarioID string, refs []string) []JourneyRefGap {
	var gaps []JourneyRefGap
	for _, ref := range nonEmptyStrings(refs) {
		switch {
		case index.err != nil:
			gaps = append(gaps, JourneyRefGap{
				ScenarioID:     scenarioID,
				JourneyPackRef: ref,
				Reason:         JourneyRefGapUnverified,
				Detail:         index.err.Error(),
			})
		default:
			if _, ok := index.ids[ref]; ok {
				continue
			}
			gaps = append(gaps, JourneyRefGap{
				ScenarioID:     scenarioID,
				JourneyPackRef: ref,
				Reason:         JourneyRefGapNotFound,
			})
		}
	}
	return gaps
}

// idsForLanes lists the Journey Packs serving any of the given lanes, in the
// packs' on-disk (sorted) order. Used to seed a starter catalog from what the
// project has instead of from a hardcoded guess that drifts.
func (index journeyPackIndex) idsForLanes(lanes []string) []string {
	refs := []string{}
	for _, pack := range index.packs {
		for _, lane := range nonEmptyStrings(lanes) {
			if journey.HasLane(pack, lane) {
				refs = appendUniqueString(refs, pack.ID)
				break
			}
		}
	}
	return refs
}

// journeyRefGapDetail renders a gap as the one-line setup-gap token carried on a
// scenario plan, so the per-scenario view names the missing pack too.
func journeyRefGapDetail(gap JourneyRefGap) string {
	return gap.Reason + ":" + gap.JourneyPackRef
}
