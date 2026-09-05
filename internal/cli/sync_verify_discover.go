package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/setup"
)

type repoDirty struct {
	Path           string
	AbsPath        string
	IsRoot         bool
	Files          []dirtyFile
	TrackedIgnored []string
}

func resolveMetaRoot(startDir string) (string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace start directory")
	}

	metaRoot := ""
	for cur := abs; ; cur = filepath.Dir(cur) {
		if info := setup.DetectMultiRepo(cur); info != nil && hasRootComponent(info) {
			metaRoot = cur
		}
		if filepath.Dir(cur) == cur {
			break
		}
	}
	if metaRoot == "" {
		return "", fmt.Errorf("no multi-repo workspace found from the current directory")
	}
	return metaRoot, nil
}

func hasRootComponent(info *setup.MultiRepoInfo) bool {
	for _, component := range info.Components {
		if component.Path == "." {
			return true
		}
	}
	return false
}

func collectDirty(metaRoot string) ([]repoDirty, error) {
	info := setup.DetectMultiRepo(metaRoot)
	if info == nil {
		return nil, fmt.Errorf("workspace is not multi-repo")
	}

	nested := map[string]bool{}
	for _, component := range info.Components {
		if component.Path != "." {
			nested[component.Path] = true
		}
	}

	repos := make([]repoDirty, 0, len(info.Components))
	for _, component := range info.Components {
		snapshot, err := capturePorcelainSnapshot(component.Path, component.AbsPath)
		if err != nil {
			return nil, err
		}
		files := snapshot.Files
		ignoredRaw, err := runSyncGit(component.Path, component.AbsPath,
			"ls-files", "-c", "-i", "--exclude-standard", "-z")
		if err != nil {
			return nil, err
		}
		ignored, err := parseNULPaths(ignoredRaw)
		if err != nil {
			return nil, fmt.Errorf("malformed tracked-but-ignored inventory for repo %s", diagnosticRepoLabel(component.Path))
		}
		if component.Path == "." {
			files = filterNestedRepoEntries(files, nested)
			ignored = filterNestedPaths(ignored, nested)
		}
		repos = append(repos, repoDirty{
			Path:           component.Path,
			AbsPath:        component.AbsPath,
			IsRoot:         component.Path == ".",
			Files:          files,
			TrackedIgnored: ignored,
		})
	}
	return repos, nil
}

func parseNULPaths(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var paths []string
	for offset := 0; offset < len(raw); {
		record, next, ok := nextNULRecord(raw, offset)
		if !ok || len(record) == 0 {
			return nil, fmt.Errorf("invalid NUL path list")
		}
		paths = append(paths, normalizeGitRel(string(record)))
		offset = next
	}
	return uniqueSortedGitPaths(paths), nil
}

func filterNestedRepoEntries(files []dirtyFile, nested map[string]bool) []dirtyFile {
	out := make([]dirtyFile, 0, len(files))
	for _, file := range files {
		if !belongsToNestedRepo(file.Rel, nested) {
			out = append(out, file)
		}
	}
	return out
}

func filterNestedPaths(paths []string, nested map[string]bool) []string {
	var out []string
	for _, rel := range paths {
		if !belongsToNestedRepo(rel, nested) {
			out = append(out, rel)
		}
	}
	return out
}

func belongsToNestedRepo(rel string, nested map[string]bool) bool {
	if nested[rel] {
		return true
	}
	for repo := range nested {
		if strings.HasPrefix(rel, repo+"/") {
			return true
		}
	}
	return false
}

func moduleSet(repos []repoDirty) map[string]bool {
	modules := map[string]bool{}
	for _, repo := range repos {
		if !repo.IsRoot {
			modules[repo.Path] = true
		}
	}
	return modules
}
