package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/setup"
)

type dirtyFile struct {
	Rel      string
	Staged   bool
	Unstaged bool
	Missing  bool
}

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
		status, err := runSyncGit(component.Path, component.AbsPath,
			"status", "--porcelain=v1", "-z", "--untracked-files=all")
		if err != nil {
			return nil, err
		}
		files, err := parsePorcelainXY(status)
		if err != nil {
			return nil, fmt.Errorf("malformed git status for repo %s", diagnosticRepoLabel(component.Path))
		}
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

func runSyncGit(repoLabel, dir string, args ...string) ([]byte, error) {
	gitArgs := append([]string{"--no-optional-locks"}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Git stderr is intentionally discarded: it can contain absolute paths,
	// credentials embedded in remotes, or attacker-controlled local text.
	if err := cmd.Run(); err != nil {
		op := "command"
		if len(args) > 0 {
			op = args[0]
		}
		return nil, fmt.Errorf("read-only git %s failed for repo %s", op, diagnosticRepoLabel(repoLabel))
	}
	return stdout.Bytes(), nil
}

func parsePorcelainXY(raw []byte) ([]dirtyFile, error) {
	var out []dirtyFile
	for offset := 0; offset < len(raw); {
		record, next, ok := nextNULRecord(raw, offset)
		if !ok || len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("invalid porcelain record")
		}
		x, y := record[0], record[1]
		if !validPorcelainCode(x) || !validPorcelainCode(y) {
			return nil, fmt.Errorf("invalid porcelain status")
		}
		out = appendDirtyPath(out, string(record[3:]), x, y, x == 'D' || y == 'D')
		offset = next
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			source, afterSource, sourceOK := nextNULRecord(raw, offset)
			if !sourceOK || len(source) == 0 {
				return nil, fmt.Errorf("rename source missing")
			}
			if x == 'R' || y == 'R' {
				out = appendRenameSource(out, string(source), x, y)
			}
			offset = afterSource
		}
	}
	return mergeDirtyFiles(out), nil
}

func nextNULRecord(raw []byte, offset int) ([]byte, int, bool) {
	if offset >= len(raw) {
		return nil, offset, false
	}
	idx := bytes.IndexByte(raw[offset:], 0)
	if idx < 0 {
		return nil, offset, false
	}
	end := offset + idx
	return raw[offset:end], end + 1, true
}

func validPorcelainCode(code byte) bool {
	return strings.ContainsRune(" MTADRCU?!", rune(code))
}

func appendDirtyPath(files []dirtyFile, rel string, x, y byte, missing bool) []dirtyFile {
	if rel == "" {
		return files
	}
	untracked := x == '?' && y == '?'
	return append(files, dirtyFile{
		Rel:      normalizeGitRel(rel),
		Staged:   x != ' ' && x != '?',
		Unstaged: y != ' ' || untracked,
		Missing:  missing,
	})
}

func appendRenameSource(files []dirtyFile, rel string, x, y byte) []dirtyFile {
	if rel == "" {
		return files
	}
	return append(files, dirtyFile{
		Rel:      normalizeGitRel(rel),
		Staged:   x == 'R',
		Unstaged: y == 'R',
		Missing:  true,
	})
}

func mergeDirtyFiles(files []dirtyFile) []dirtyFile {
	merged := map[string]dirtyFile{}
	for _, file := range files {
		if file.Rel == "" {
			continue
		}
		current, seen := merged[file.Rel]
		current.Rel = file.Rel
		current.Staged = current.Staged || file.Staged
		current.Unstaged = current.Unstaged || file.Unstaged
		if seen {
			current.Missing = current.Missing && file.Missing
		} else {
			current.Missing = file.Missing
		}
		merged[file.Rel] = current
	}
	out := make([]dirtyFile, 0, len(merged))
	for _, file := range merged {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out
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
