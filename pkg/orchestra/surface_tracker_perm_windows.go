//go:build windows

package orchestra

import "os"

// surfaceDirSecure is a compile-only fallback on Windows, where POSIX uid and
// permission-bit checks do not apply (ACL-based model) and the orchestra pane
// backends (tmux/cmux) are unix-only anyway. It only verifies the path is an
// existing directory; the unix build carries the real ownership/mode check.
func surfaceDirSecure(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
