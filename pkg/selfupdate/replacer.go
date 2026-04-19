package selfupdate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// Replacer atomically replaces a binary file.
type Replacer struct{}

// NewReplacer creates a new Replacer.
func NewReplacer() *Replacer {
	return &Replacer{}
}

// Replace atomically replaces the target binary with the new one.
func (r *Replacer) Replace(newBinaryPath, targetPath string) error {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)

	// Move the running binary out of the way to prevent ETXTBSY on Linux
	// and SIGKILL on macOS when replacing an executing binary.
	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("rename old binary: %w", err)
	}

	// Try to rename first (atomic on same filesystem)
	err = os.Rename(newBinaryPath, targetPath)
	if err != nil {
		// Cross-device link fallback (EXDEV)
		err = copyFile(newBinaryPath, targetPath)
		if err != nil {
			// Restore the old binary if replacing fails.
			_ = os.Rename(oldPath, targetPath)
			return fmt.Errorf("새 바이너리 교체 실패: %w", err)
		}
	}

	if err := os.Chmod(targetPath, targetInfo.Mode().Perm()); err != nil {
		return err
	}

	// macOS: clear extended attributes and ad-hoc codesign to prevent Gatekeeper from killing the binary.
	// com.apple.quarantine and com.apple.provenance both trigger SIGKILL on macOS Sequoia+.
	if runtime.GOOS == "darwin" {
		_ = exec.Command("xattr", "-cr", targetPath).Run()
		_ = exec.Command("codesign", "--force", "--sign", "-", targetPath).Run()
	}

	// Cleanup old binary (best-effort, might fail on Windows if still running)
	_ = os.Remove(oldPath)

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
