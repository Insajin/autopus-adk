package adapter

import (
	"os"
	"path/filepath"
	"strings"
)

// SupportsRootGitHooks reports whether .git/hooks can be addressed under root.
// Only a real root-local gitdir qualifies, proven by a readable .git/HEAD:
//
//   - Linked worktrees store .git as a gitdir file, so root-local .git/hooks is
//     not a valid path there.
//   - When .git is absent, writing .git/hooks/* would MkdirAll a .git directory
//     and fabricate a repository marker in a checkout that is not a repository
//     (meta workspaces hosting sibling repos). Git never honors those hooks, and
//     the fabricated directory makes the root look like a repo to other tooling.
//   - A .git directory without HEAD is that same residue, not a repository.
//
// A root that is initialized later picks the hooks up on the next `auto update`.
func SupportsRootGitHooks(root string) bool {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(gitPath, "HEAD")); err != nil {
		return false
	}
	return true
}

// IsRootGitHookPath reports whether path targets root-local .git/hooks.
func IsRootGitHookPath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.HasPrefix(clean, ".git/hooks/")
}

// FilterUnsupportedRootGitHookFiles removes root-local git hook writes in linked worktrees.
func FilterUnsupportedRootGitHookFiles(root string, files []FileMapping) []FileMapping {
	if SupportsRootGitHooks(root) {
		return files
	}
	filtered := files[:0]
	for _, file := range files {
		if IsRootGitHookPath(file.TargetPath) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

// FilterUnsupportedRootGitHookRemoves removes root-local git hook prunes in linked worktrees.
func FilterUnsupportedRootGitHookRemoves(root string, removes []TransactionRemove) []TransactionRemove {
	if SupportsRootGitHooks(root) {
		return removes
	}
	filtered := removes[:0]
	for _, remove := range removes {
		if IsRootGitHookPath(remove.Path) {
			continue
		}
		filtered = append(filtered, remove)
	}
	return filtered
}
