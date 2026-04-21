package setup

import (
	"fmt"
	"path/filepath"
	"sort"
)

func buildWorkspaceHints(projectDir string, info *ProjectInfo) []WorkspaceHint {
	if info == nil {
		return nil
	}

	if info.MultiRepo != nil && info.MultiRepo.IsMultiRepo {
		repoName := filepath.Base(projectDir)
		if root := findRootRepo(info.MultiRepo.Components); root != nil && root.Name != "" {
			repoName = root.Name
		}
		return []WorkspaceHint{{
			Kind:          WorkspaceHintKindMultiRepo,
			Repo:          repoName,
			SourceOfTruth: displayChangePath(projectDir, projectDir),
			Message: fmt.Sprintf(
				"multi-repo workspace detected (%d repos); review the owning repo before applying bootstrap changes",
				len(info.MultiRepo.Components),
			),
		}}
	}

	if len(info.Workspaces) == 0 {
		return []WorkspaceHint{{
			Kind:          WorkspaceHintKindSingleRepo,
			Repo:          filepath.Base(projectDir),
			SourceOfTruth: displayChangePath(projectDir, projectDir),
			Message:       "single repository detected; setup changes target this repo directly",
		}}
	}

	types := make([]string, 0, len(info.Workspaces))
	seen := make(map[string]bool, len(info.Workspaces))
	for _, workspace := range info.Workspaces {
		if seen[workspace.Type] {
			continue
		}
		seen[workspace.Type] = true
		types = append(types, workspace.Type)
	}
	sort.Strings(types)

	return []WorkspaceHint{{
		Kind:          WorkspaceHintKindWorkspace,
		Repo:          filepath.Base(projectDir),
		SourceOfTruth: displayChangePath(projectDir, projectDir),
		Message: fmt.Sprintf(
			"workspace-aware repo detected (%d modules via %v); preview applies to the current repo root",
			len(info.Workspaces),
			types,
		),
	}}
}

func findRootRepo(components []RepoComponent) *RepoComponent {
	for i := range components {
		if components[i].Path == "." {
			return &components[i]
		}
	}
	return nil
}
