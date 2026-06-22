package workflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// isSafePath returns true iff relpath is a relative path that does not escape
// the working directory. It rejects empty strings, absolute paths, and paths
// that resolve to a "../" escape after filepath.Clean.
func isSafePath(relpath string) bool {
	if relpath == "" {
		return false
	}
	if filepath.IsAbs(relpath) {
		return false
	}
	cleaned := filepath.Clean(relpath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

// ensureWithin verifies that target stays within dir even after resolving
// symlinked path components. It resolves the longest existing ancestor of
// target via EvalSymlinks and requires it to remain under dir. This blocks a
// pre-existing symlink in the destination tree from redirecting a write outside
// the working directory (defence against symlinked-parent escape).
func ensureWithin(dir, target string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	cur, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	cur = filepath.Clean(cur)
	for {
		if _, err := os.Lstat(cur); err == nil {
			resolved, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return err
			}
			if resolved != root && !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
				return fmt.Errorf("path escapes working dir: %s", target)
			}
			return nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil
		}
		cur = parent
	}
}

// copyFile copies <srcDir>/<relpath> to <dstDir>/<relpath> atomically and
// safely. The caller (changedFiles) has already filtered out symlinks, deleted,
// and non-regular sources, but copyFile re-verifies defensively:
//   - relpath must be lexically safe (no escape);
//   - src must be a regular file, never a symlink (blocks reading a symlink
//     target such as ~/.ssh/id_ed25519 into the repo — exfil defence);
//   - dst must not be (or sit under) a symlink that escapes dstDir;
//   - src and dst must not be the same file (blocks self-truncation).
func copyFile(srcDir, relpath, dstDir string) error {
	if !isSafePath(relpath) {
		return fmt.Errorf("unsafe path rejected: %s", relpath)
	}
	src := filepath.Join(srcDir, relpath)
	dst := filepath.Join(dstDir, relpath)

	si, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if si.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlink source: %s", relpath)
	}
	if !si.Mode().IsRegular() {
		return fmt.Errorf("refusing to copy non-regular source: %s", relpath)
	}

	if err := ensureWithin(dstDir, dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Re-verify after creating parents (a created/symlinked parent could change
	// resolution), and refuse to write through a symlink at dst itself.
	if err := ensureWithin(dstDir, dst); err != nil {
		return err
	}
	if di, err := os.Lstat(dst); err == nil {
		if di.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("dst is a symlink, refusing to follow: %s", relpath)
		}
		if si2, err := os.Stat(dst); err == nil && os.SameFile(si, si2) {
			return fmt.Errorf("src and dst are the same file: %s", relpath)
		}
	}

	return atomicCopy(src, dst)
}

// atomicCopy writes src into dst via a temp file in dst's directory followed by
// an atomic rename, so a partial copy never leaves a truncated dst. It opens src
// with O_NOFOLLOW and re-checks the open fd for a hard link (Nlink>1), an
// authoritative backstop against a symlink-swap or hard-link exfil that races the
// earlier Lstat in changedFiles.
func atomicCopy(src, dst string) error {
	in, err := openNoFollow(src) //nolint:gosec // src validated regular+non-symlink; opened O_NOFOLLOW
	if err != nil {
		return err
	}
	defer in.Close()

	if fi, err := in.Stat(); err == nil && hardLinked(fi) {
		return fmt.Errorf("refusing to copy hard-linked source: %s", src)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".merge-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed; cleans up on failure

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
