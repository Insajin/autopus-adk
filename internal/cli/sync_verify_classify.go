package cli

import (
	"fmt"
	"sort"
	"strings"
)

// phaseGroup is a per-repo bucket of files staged for one commit phase.
type phaseGroup struct {
	RepoPath string
	Files    []string
}

// rootTrackedExact lists top-level meta files owned by the root (Phase B) per
// the doc-storage Storage Matrix.
var rootTrackedExact = map[string]bool{
	"ARCHITECTURE.md": true,
	"CLAUDE.md":       true,
	"autopus.yaml":    true,
}

// rootTrackedPrefixes lists root-owned directory trees per the Storage Matrix.
var rootTrackedPrefixes = []string{
	".autopus/project/",
	".autopus/specs/",
	".autopus/brainstorms/",
	".claude/",
}

// isRootTracked reports whether a root-repo relative path belongs to the meta
// (Phase B) commit set. Root files outside this set stay unclassified so they
// are never misattributed to a phase.
func isRootTracked(rel string) bool {
	if rootTrackedExact[rel] {
		return true
	}
	for _, p := range rootTrackedPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	// CHANGELOG*.md at the workspace root (e.g. CHANGELOG-2026H1.md).
	if !strings.Contains(rel, "/") && strings.HasPrefix(rel, "CHANGELOG") && strings.HasSuffix(rel, ".md") {
		return true
	}
	return false
}

// classifyPhases splits dirty files into Phase A module groups (alphabetical by
// repo path) and the Phase B meta group.
func classifyPhases(repos []repoDirty) (phaseA []phaseGroup, phaseB phaseGroup) {
	phaseB.RepoPath = "."
	for _, r := range repos {
		if r.IsRoot {
			var files []string
			for _, f := range r.Files {
				if isRootTracked(f.Rel) {
					files = append(files, f.Rel)
				}
			}
			sort.Strings(files)
			phaseB.Files = files
			continue
		}

		var files []string
		for _, f := range r.Files {
			files = append(files, f.Rel)
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		phaseA = append(phaseA, phaseGroup{RepoPath: r.Path, Files: files})
	}
	sort.Slice(phaseA, func(i, j int) bool { return phaseA[i].RepoPath < phaseA[j].RepoPath })
	return phaseA, phaseB
}

// detectMixing flags repos whose staged and unstaged changes coexist, which is
// a partial-commit hazard during a two-phase sync.
func detectMixing(repos []repoDirty) []string {
	var warnings []string
	for _, r := range repos {
		var staged, unstaged []string
		for _, f := range r.Files {
			if f.Staged {
				staged = append(staged, f.Rel)
			}
			if f.Unstaged {
				unstaged = append(unstaged, f.Rel)
			}
		}
		if len(staged) == 0 || len(unstaged) == 0 {
			continue
		}
		sort.Strings(staged)
		sort.Strings(unstaged)
		warnings = append(warnings, fmt.Sprintf(
			"WARN  mixed-staging: repo %s has staged and unstaged changes coexisting (staged: %s; unstaged: %s)",
			r.Path, strings.Join(staged, ", "), strings.Join(unstaged, ", ")))
	}
	return warnings
}

// detectMisplacedMeta flags nested modules carrying root-scoped project context
// docs, which the Storage Matrix reserves for the meta root.
func detectMisplacedMeta(repos []repoDirty) []string {
	var warnings []string
	for _, r := range repos {
		if r.IsRoot {
			continue
		}
		var misplaced []string
		for _, f := range r.Files {
			if strings.HasPrefix(f.Rel, ".autopus/project/") {
				misplaced = append(misplaced, f.Rel)
			}
		}
		if len(misplaced) == 0 {
			continue
		}
		sort.Strings(misplaced)
		warnings = append(warnings, fmt.Sprintf(
			"WARN  misplacement: module %s carries root-scoped meta docs (%s) -> expected root .autopus/project/",
			r.Path, strings.Join(misplaced, ", ")))
	}
	return warnings
}

// detectViolations aggregates the deterministic warning set across all classes.
func detectViolations(repos []repoDirty, modules map[string]bool) []string {
	var warnings []string
	warnings = append(warnings, detectSpecViolations(repos, modules)...)
	warnings = append(warnings, detectMisplacedMeta(repos)...)
	warnings = append(warnings, detectMixing(repos)...)
	return warnings
}
