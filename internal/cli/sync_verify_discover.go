package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/setup"
)

// repoRole selects the partition policy a repository's dirty paths follow.
type repoRole int

const (
	// repoRoleMeta is the root of a multi-repo workspace: only canonical root
	// documents may be committed there.
	repoRoleMeta repoRole = iota
	// repoRoleModule is a nested repository inside a multi-repo workspace.
	repoRoleModule
	// repoRoleSingle is a whole single-repository project. It carries product
	// code and root documents together, so the canonical root keep set — which
	// exists to push code out of a meta repo — does not apply.
	repoRoleSingle
)

type repoDirty struct {
	Path           string
	AbsPath        string
	Role           repoRole
	Files          []dirtyFile
	TrackedIgnored []string
}

// isRoot reports whether the repository owns the workspace root path ".".
func (r repoDirty) isRoot() bool { return r.Role != repoRoleModule }

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
		role := repoRoleModule
		if component.Path == "." {
			role = repoRoleMeta
			files = filterNestedRepoEntries(files, nested)
			ignored = filterNestedPaths(ignored, nested)
		}
		repos = append(repos, repoDirty{
			Path:           component.Path,
			AbsPath:        component.AbsPath,
			Role:           role,
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
		if !repo.isRoot() {
			modules[repo.Path] = true
		}
	}
	return modules
}
