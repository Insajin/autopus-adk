package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/setup"
)

// dirtyFile is a single working-tree change with staging flags derived from the
// git porcelain XY status codes.
type dirtyFile struct {
	Rel      string // repo-relative slash path
	Staged   bool   // index side (X) carries a change
	Unstaged bool   // worktree side (Y) carries a change, or the entry is untracked
}

// repoDirty holds the read-only dirty inventory for one commit-boundary repo.
type repoDirty struct {
	Path    string // slash path relative to the meta root ("." for the root repo)
	AbsPath string
	IsRoot  bool
	Files   []dirtyFile
}

// resolveMetaRoot walks upward from startDir and returns the outermost git repo
// whose immediate children include at least one nested git repo. That repo is
// the meta workspace root for two-phase sync classification.
func resolveMetaRoot(startDir string) (string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	metaRoot := ""
	cur := abs
	for {
		if info := setup.DetectMultiRepo(cur); info != nil && hasRootComponent(info) {
			metaRoot = cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	if metaRoot == "" {
		return "", fmt.Errorf("no multi-repo workspace found from the current directory (need a git repo with at least one nested repository)")
	}
	return metaRoot, nil
}

// hasRootComponent reports whether the detected workspace treats the scanned
// directory itself as a git repo (the "." component), guarding against picking a
// non-git parent that merely contains repo children.
func hasRootComponent(info *setup.MultiRepoInfo) bool {
	for _, c := range info.Components {
		if c.Path == "." {
			return true
		}
	}
	return false
}

// collectDirty enumerates every commit-boundary repo under metaRoot and gathers
// its dirty files via read-only git status. It performs zero git mutations.
func collectDirty(metaRoot string) ([]repoDirty, error) {
	info := setup.DetectMultiRepo(metaRoot)
	if info == nil {
		return nil, fmt.Errorf("workspace is not multi-repo")
	}

	nested := map[string]bool{}
	for _, c := range info.Components {
		if c.Path != "." {
			nested[c.Path] = true
		}
	}

	var repos []repoDirty
	for _, c := range info.Components {
		lines, err := hygieneGitLines(c.AbsPath, "status", "--porcelain=v1", "--untracked-files=all")
		if err != nil {
			return nil, fmt.Errorf("read-only git status failed for repo %s: %w", c.Path, err)
		}
		files := parsePorcelainXY(lines)
		if c.Path == "." {
			files = filterNestedRepoEntries(files, nested)
		}
		repos = append(repos, repoDirty{
			Path:    c.Path,
			AbsPath: c.AbsPath,
			IsRoot:  c.Path == ".",
			Files:   files,
		})
	}
	return repos, nil
}

// parsePorcelainXY converts `git status --porcelain=v1` lines into dirtyFile
// records, preserving staged/unstaged flags from the XY code columns.
func parsePorcelainXY(lines []string) []dirtyFile {
	var out []dirtyFile
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		x := line[0]
		y := line[1]
		untracked := line[:2] == "??"

		rel := strings.TrimSpace(line[3:])
		if idx := strings.LastIndex(rel, " -> "); idx >= 0 {
			rel = rel[idx+4:]
		}
		rel = normalizeGitRel(strings.Trim(rel, `"`))
		if rel == "" {
			continue
		}

		out = append(out, dirtyFile{
			Rel:      rel,
			Staged:   x != ' ' && x != '?',
			Unstaged: y != ' ' || untracked,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out
}

// filterNestedRepoEntries removes root-repo entries that actually belong to a
// nested repo (git surfaces embedded repos as a single entry), so each dirty
// file is attributed to exactly one repo.
func filterNestedRepoEntries(files []dirtyFile, nested map[string]bool) []dirtyFile {
	var out []dirtyFile
	for _, f := range files {
		if nested[f.Rel] {
			continue
		}
		skip := false
		for n := range nested {
			if strings.HasPrefix(f.Rel, n+"/") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, f)
	}
	return out
}

// moduleSet returns the set of nested module repo paths (excluding the root).
func moduleSet(repos []repoDirty) map[string]bool {
	m := map[string]bool{}
	for _, r := range repos {
		if !r.IsRoot {
			m[r.Path] = true
		}
	}
	return m
}
