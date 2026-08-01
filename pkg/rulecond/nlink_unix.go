//go:build unix

package rulecond

import (
	"io/fs"
	"syscall"
)

// hasMultipleLinks reports whether a regular file has more than one directory
// entry. filepath.EvalSymlinks cannot see a hardlink: the second entry is the
// same inode, not a redirection, so the resolved path stays inside the body
// root while the bytes are also reachable from outside it. Link count is the
// only available signal.
//
// The generated bodies are written with a plain create-and-write, so a
// legitimate body always has exactly one link.
func hasMultipleLinks(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return stat.Nlink > 1
}
