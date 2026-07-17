package cli

import (
	"fmt"
	"sort"
	"strings"
)

type phaseGroup struct {
	RepoPath    string
	Files       []string
	AddFiles    []string
	UpdateFiles []string
	StagedOnly  []string
}

type partitionedPath struct {
	RepoPath string
	Rel      string
	Reason   string
}

type workspaceClassification struct {
	PhaseA       []phaseGroup
	PhaseB       phaseGroup
	Blocked      []partitionedPath
	Unclassified []partitionedPath
}

func classifyWorkspace(repos []repoDirty) workspaceClassification {
	result := workspaceClassification{PhaseB: phaseGroup{RepoPath: "."}}
	for _, repo := range repos {
		dirtyByPath := make(map[string]dirtyFile, len(repo.Files))
		for _, file := range repo.Files {
			dirtyByPath[file.Rel] = file
		}
		trackedIgnored := make(map[string]bool, len(repo.TrackedIgnored))
		for _, rel := range repo.TrackedIgnored {
			trackedIgnored[rel] = true
		}
		paths := make(map[string]bool, len(repo.Files)+len(repo.TrackedIgnored))
		for _, file := range repo.Files {
			paths[file.Rel] = true
		}
		for _, rel := range repo.TrackedIgnored {
			paths[rel] = true
		}

		var candidates []string
		for rel := range paths {
			switch {
			case trackedIgnored[rel]:
				result.Blocked = append(result.Blocked, partitionedPath{repo.Path, rel, "tracked-but-ignored"})
			case isGeneratedRuntime(rel):
				result.Blocked = append(result.Blocked, partitionedPath{repo.Path, rel, "generated/runtime"})
			case !isSafeRepoPath(repo.Path) || !isSafePlanPath(rel):
				result.Unclassified = append(result.Unclassified, partitionedPath{repo.Path, rel, "unsafe-plan-path"})
			case repo.IsRoot && isRootTracked(rel):
				candidates = append(candidates, rel)
			case repo.IsRoot:
				result.Unclassified = append(result.Unclassified, partitionedPath{repo.Path, rel, "outside canonical root keep set"})
			case strings.HasPrefix(rel, ".autopus/project/"):
				result.Unclassified = append(result.Unclassified, partitionedPath{repo.Path, rel, "root-scoped meta path in module"})
			default:
				candidates = append(candidates, rel)
			}
		}
		sort.Strings(candidates)
		group := buildPhaseGroup(repo.Path, candidates, dirtyByPath)
		if repo.IsRoot {
			result.PhaseB = group
		} else if len(candidates) > 0 {
			result.PhaseA = append(result.PhaseA, group)
		}
	}
	sort.Slice(result.PhaseA, func(i, j int) bool { return result.PhaseA[i].RepoPath < result.PhaseA[j].RepoPath })
	sortPartitioned(result.Blocked)
	sortPartitioned(result.Unclassified)
	return result
}

func buildPhaseGroup(repoPath string, candidates []string, dirtyByPath map[string]dirtyFile) phaseGroup {
	group := phaseGroup{RepoPath: repoPath, Files: candidates}
	for _, rel := range candidates {
		file := dirtyByPath[rel]
		switch {
		case !file.Missing:
			group.AddFiles = append(group.AddFiles, rel)
		case file.Unstaged:
			group.UpdateFiles = append(group.UpdateFiles, rel)
		default:
			group.StagedOnly = append(group.StagedOnly, rel)
		}
	}
	return group
}

func sortPartitioned(items []partitionedPath) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].RepoPath != items[j].RepoPath {
			return items[i].RepoPath < items[j].RepoPath
		}
		return items[i].Rel < items[j].Rel
	})
}

func classifyPhases(repos []repoDirty) ([]phaseGroup, phaseGroup) {
	classified := classifyWorkspace(repos)
	return classified.PhaseA, classified.PhaseB
}

func partitionWarnings(classified workspaceClassification) []string {
	warnings := make([]string, 0, len(classified.Blocked)+len(classified.Unclassified))
	for _, item := range classified.Blocked {
		warnings = append(warnings, fmt.Sprintf(
			"WARN  blocked-path: repo %s path %s excluded (%s)",
			diagnosticRepoLabel(item.RepoPath), displayPath(item.Rel), item.Reason))
	}
	for _, item := range classified.Unclassified {
		warnings = append(warnings, fmt.Sprintf(
			"WARN  unclassified-path: repo %s path %s excluded (%s)",
			diagnosticRepoLabel(item.RepoPath), displayPath(item.Rel), item.Reason))
	}
	return warnings
}

func detectMixing(repos []repoDirty) []string {
	var warnings []string
	for _, repo := range repos {
		var staged, unstaged []string
		for _, file := range repo.Files {
			if file.Staged {
				staged = append(staged, file.Rel)
			}
			if file.Unstaged {
				unstaged = append(unstaged, file.Rel)
			}
		}
		if len(staged) == 0 || len(unstaged) == 0 {
			continue
		}
		sort.Strings(staged)
		sort.Strings(unstaged)
		warnings = append(warnings, fmt.Sprintf(
			"WARN  mixed-staging: repo %s has staged and unstaged changes coexisting (staged: %s; unstaged: %s)",
			diagnosticRepoLabel(repo.Path), displayPaths(staged), displayPaths(unstaged)))
	}
	return warnings
}

func detectMisplacedMeta(repos []repoDirty) []string {
	var warnings []string
	for _, repo := range repos {
		if repo.IsRoot {
			continue
		}
		var misplaced []string
		for _, file := range repo.Files {
			if strings.HasPrefix(file.Rel, ".autopus/project/") {
				misplaced = append(misplaced, file.Rel)
			}
		}
		if len(misplaced) > 0 {
			sort.Strings(misplaced)
			warnings = append(warnings, fmt.Sprintf(
				"WARN  misplacement: module %s carries root-scoped meta docs (%s) -> expected root .autopus/project/",
				diagnosticRepoLabel(repo.Path), displayPaths(misplaced)))
		}
	}
	return warnings
}

func detectViolations(repos []repoDirty, modules map[string]bool, classified workspaceClassification) ([]string, error) {
	specWarnings, err := detectSpecViolations(repos, modules)
	if err != nil {
		return nil, err
	}
	warnings := partitionWarnings(classified)
	warnings = append(warnings, specWarnings...)
	warnings = append(warnings, detectMisplacedMeta(repos)...)
	warnings = append(warnings, detectMixing(repos)...)
	return warnings, nil
}
