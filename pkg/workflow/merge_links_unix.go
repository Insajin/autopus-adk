//go:build unix

package workflow

import (
	"os"
	"syscall"
)

// hardLinked reports whether info describes a file with more than one hard link.
// A worktree work-file normally has exactly one link; Nlink>1 indicates the file
// is hard-linked to another inode (e.g. an external secret), which the merge step
// refuses to copy to avoid laundering an outside file into the repo.
func hardLinked(info os.FileInfo) bool {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Nlink > 1
	}
	return false
}

// openNoFollow opens src read-only without following a final symlink (O_NOFOLLOW),
// closing the Lstat→Open TOCTOU window against a symlink swap.
func openNoFollow(src string) (*os.File, error) {
	return os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}
