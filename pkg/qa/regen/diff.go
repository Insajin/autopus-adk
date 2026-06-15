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
// with no corresponding synthesized pack is removed. Entries within each
// category are sorted by JourneyID and categories are emitted in fixed order,
// so json.Marshal is byte-identical across runs over identical inputs.
//
// Only accepted (non-excluded) synthesized packs participate: excluded packs
// appear in neither added nor changed.
func ComputeDiff(synthesized []SynthesizedPack, existing map[string]journey.Pack) Diff {
	accepted := acceptedByID(synthesized)
	diff := Diff{
		Added:   []DiffEntry{},
		Changed: []DiffEntry{},
		Removed: []DiffEntry{},
	}
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
			diff.Removed = append(diff.Removed, DiffEntry{JourneyID: id, Category: "removed"})
		}
	}
	sortEntries(diff.Added)
	sortEntries(diff.Changed)
	sortEntries(diff.Removed)
	diff.AddedCount = len(diff.Added)
	diff.ChangedCount = len(diff.Changed)
	diff.RemovedCount = len(diff.Removed)
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
