package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// orchestraArtifactDir is the orchestrator's own artifact root under the
// working directory. Prompt, response, launch, result, and diagnostics files
// are written there by this process, so it is excluded from the mutation delta.
const orchestraArtifactDir = ".autopus/orchestra"

// workspaceGuard compares the caller's worktree before and after provider
// execution (issue #108). It never rolls anything back: a detected change is
// reported with the changed paths and fails the run.
type workspaceGuard struct {
	root    string
	exclude string // root-relative artifact prefix, slash-terminated
	before  *gitPorcelainSnapshot
}

// newWorkspaceGuard snapshots the git worktree containing workingDir. When the
// directory is not inside a worktree, or the status inventory cannot be read,
// the guard records unavailable evidence and never blocks the run.
func newWorkspaceGuard(workingDir string) workspaceGuard {
	root, err := gitWorktreeRoot(workingDir)
	if err != nil {
		return workspaceGuard{root: workingDir}
	}
	guard := workspaceGuard{root: root, exclude: artifactExclusionPrefix(root, workingDir)}
	snapshot, err := capturePorcelainSnapshot(".", root)
	if err != nil {
		return guard
	}
	guard.before = &snapshot
	return guard
}

// compare captures the post-run snapshot and reports the delta as evidence.
func (g workspaceGuard) compare() orchestra.WorkspaceEvidence {
	evidence := orchestra.WorkspaceEvidence{Root: g.root, Status: orchestra.WorkspaceStatusUnavailable}
	if g.before == nil {
		return evidence
	}
	evidence.SnapshotBefore = summarizeSnapshot(*g.before)
	after, err := capturePorcelainSnapshot(".", g.root)
	if err != nil {
		return evidence
	}
	evidence.SnapshotAfter = summarizeSnapshot(after)
	evidence.ChangedFiles = porcelainDelta(g.before.Files, after.Files, g.exclude)
	evidence.Status = orchestra.WorkspaceStatusClean
	if len(evidence.ChangedFiles) > 0 {
		evidence.MutationDetected = true
		evidence.Status = orchestra.WorkspaceStatusMutated
	}
	return evidence
}

func gitWorktreeRoot(dir string) (string, error) {
	raw, err := runSyncGit(".", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(raw))
	if root == "" {
		return "", errors.New("git worktree root unavailable")
	}
	return root, nil
}

// artifactExclusionPrefix resolves the orchestrator artifact directory relative
// to the worktree root so runs started from a subdirectory still exclude it.
// Both paths are canonicalized first: git reports the physical root while the
// caller's cwd may still contain symlinks (macOS /var vs /private/var).
func artifactExclusionPrefix(root, workingDir string) string {
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(workingDir))
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return orchestraArtifactDir + "/"
	}
	return filepath.ToSlash(filepath.Join(rel, orchestraArtifactDir)) + "/"
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func summarizeSnapshot(snapshot gitPorcelainSnapshot) *orchestra.WorkspaceSnapshot {
	digest := sha256.Sum256(snapshot.Raw)
	return &orchestra.WorkspaceSnapshot{Entries: len(snapshot.Files), SHA256: hex.EncodeToString(digest[:])}
}

// porcelainDelta lists paths whose (path, status) pair differs between the two
// snapshots: new or removed entries and entries whose staged/unstaged/missing
// state changed. Paths under the artifact exclusion prefix are ignored.
func porcelainDelta(before, after []dirtyFile, exclude string) []string {
	excluded := func(rel string) bool { return exclude != "" && strings.HasPrefix(rel, exclude) }
	previous := make(map[string]dirtyFile, len(before))
	for _, file := range before {
		previous[file.Rel] = file
	}
	var changed []string
	seen := make(map[string]struct{}, len(after))
	for _, file := range after {
		seen[file.Rel] = struct{}{}
		if excluded(file.Rel) {
			continue
		}
		if prior, ok := previous[file.Rel]; !ok || prior != file {
			changed = append(changed, file.Rel)
		}
	}
	for _, file := range before {
		if _, ok := seen[file.Rel]; ok || excluded(file.Rel) {
			continue
		}
		changed = append(changed, file.Rel)
	}
	sort.Strings(changed)
	return changed
}
