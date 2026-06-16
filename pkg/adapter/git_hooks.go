package adapter

import (
	"os"
	"path/filepath"
	"strings"
)

// SupportsRootGitHooks reports whether .git/hooks can be addressed under root.
// Linked worktrees store .git as a gitdir file, so root-local .git/hooks is not
// a valid path there.
func SupportsRootGitHooks(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return true
	}
	return info.IsDir()
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
