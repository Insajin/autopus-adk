package adapter

import (
	"path/filepath"
	"sort"
	"strings"
)

const (
	ManifestActionEmit   = "emit"
	ManifestActionRetain = "retain"
	ManifestActionPrune  = "prune"
)

// ManifestDiffEntry describes one compiler-managed manifest transition.
type ManifestDiffEntry struct {
	Path        string
	Action      string
	OldChecksum string
	NewChecksum string
}

// ManifestDiff groups emit/retain/prune transitions.
type ManifestDiff struct {
	Emit   []ManifestDiffEntry
	Retain []ManifestDiffEntry
	Prune  []ManifestDiffEntry
}

// BuildManifestDiff computes emit/retain/prune actions from the old manifest and new files.
func BuildManifestDiff(old *Manifest, newFiles []FileMapping, pruneRoots []string) ManifestDiff {
	next := make(map[string]FileMapping, len(newFiles))
	for _, file := range newFiles {
		next[filepath.ToSlash(file.TargetPath)] = file
	}

	diff := ManifestDiff{
		Emit:   make([]ManifestDiffEntry, 0, len(newFiles)),
		Retain: make([]ManifestDiffEntry, 0, len(newFiles)),
		Prune:  make([]ManifestDiffEntry, 0),
	}

	for path, file := range next {
		if old == nil {
			diff.Emit = append(diff.Emit, ManifestDiffEntry{Path: path, Action: ManifestActionEmit, NewChecksum: file.Checksum})
			continue
		}
		prev, ok := old.Files[path]
		if ok && prev.Checksum == file.Checksum {
			diff.Retain = append(diff.Retain, ManifestDiffEntry{Path: path, Action: ManifestActionRetain, OldChecksum: prev.Checksum, NewChecksum: file.Checksum})
			continue
		}
		oldChecksum := ""
		if ok {
			oldChecksum = prev.Checksum
		}
		diff.Emit = append(diff.Emit, ManifestDiffEntry{Path: path, Action: ManifestActionEmit, OldChecksum: oldChecksum, NewChecksum: file.Checksum})
	}

	if old != nil {
		for path, prev := range old.Files {
			if _, ok := next[path]; ok || !isPruneEligible(path, pruneRoots) {
				continue
			}
			diff.Prune = append(diff.Prune, ManifestDiffEntry{Path: path, Action: ManifestActionPrune, OldChecksum: prev.Checksum})
		}
	}

	sortManifestDiffEntries(diff.Emit)
	sortManifestDiffEntries(diff.Retain)
	sortManifestDiffEntries(diff.Prune)
	return diff
}

func isPruneEligible(path string, roots []string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return false
	}
	if len(roots) == 0 {
		return true
	}
	for _, root := range roots {
		normalizedRoot := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(root)), "/")
		if clean == normalizedRoot || strings.HasPrefix(clean, normalizedRoot+"/") {
			return true
		}
	}
	return false
}

func sortManifestDiffEntries(entries []ManifestDiffEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
}
