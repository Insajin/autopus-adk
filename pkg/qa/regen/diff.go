package regen

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// LoadExistingPacks reads every pack YAML under
// <projectDir>/.autopus/qa/journeys/*.yaml with journey.LoadFile (parse only, no
// validation) so an invalid existing pack does not abort diff computation. Packs
// are returned keyed by ID; the last pack wins on duplicate IDs after sorting.
func LoadExistingPacks(projectDir string) (map[string]journey.Pack, error) {
	pattern := filepath.Join(projectDir, ".autopus", "qa", "journeys", "*.yaml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	packs := make(map[string]journey.Pack, len(paths))
	for _, path := range paths {
		pack, loadErr := journey.LoadFile(path)
		if loadErr != nil {
			// Skip unparseable files; diff stays best-effort over readable packs.
			continue
		}
		if strings.TrimSpace(pack.ID) == "" {
			continue
		}
		packs[pack.ID] = pack
	}
	return packs, nil
}

// ComputeDiff classifies accepted synthesized packs against existing packs by
// Pack.ID. A synthesized pack with no matching existing pack is added; with a
// matching pack whose compared fields differ it is changed; an existing pack
// that this synthesis did not produce is unmatched. Entries within each
// category are sorted by JourneyID and categories are emitted in fixed order,
// so json.Marshal is byte-identical across runs over identical inputs.
//
// Only accepted (non-excluded) synthesized packs participate: excluded packs
// appear in neither added nor changed.
//
// An analysis that synthesized nothing produces an entirely empty diff. With no
// reference set, "unmatched" would be a vacuous claim over every pack the
// project has — which on a Go/CLI project (PresentSurfaces recognizes only
// web/desktop/mobile) is every pack qa init scaffolded. Silence is the only
// honest output when nothing was analyzed.
func ComputeDiff(synthesized []SynthesizedPack, existing map[string]journey.Pack) Diff {
	diff := Diff{
		Added:     []DiffEntry{},
		Changed:   []DiffEntry{},
		Unmatched: []DiffEntry{},
	}
	if len(synthesized) == 0 {
		return diff
	}
	accepted := acceptedByID(synthesized)
	for id, pack := range accepted {
		prior, ok := existing[id]
		if !ok {
			diff.Added = append(diff.Added, DiffEntry{JourneyID: id, Category: "added"})
			continue
		}
		if changes := comparePacks(prior, pack); len(changes) > 0 {
			diff.Changed = append(diff.Changed, DiffEntry{
				JourneyID:     id,
				Category:      "changed",
				ChangedFields: changes,
			})
		}
	}
	for id := range existing {
		if _, ok := accepted[id]; !ok {
			diff.Unmatched = append(diff.Unmatched, DiffEntry{JourneyID: id, Category: "unmatched"})
		}
	}
	sortEntries(diff.Added)
	sortEntries(diff.Changed)
	sortEntries(diff.Unmatched)
	diff.AddedCount = len(diff.Added)
	diff.ChangedCount = len(diff.Changed)
	diff.UnmatchedCount = len(diff.Unmatched)
	return diff
}

func acceptedByID(synthesized []SynthesizedPack) map[string]journey.Pack {
	accepted := make(map[string]journey.Pack, len(synthesized))
	for _, sp := range synthesized {
		if sp.Excluded {
			continue
		}
		accepted[sp.Pack.ID] = sp.Pack
	}
	return accepted
}

func sortEntries(entries []DiffEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].JourneyID < entries[j].JourneyID
	})
}
