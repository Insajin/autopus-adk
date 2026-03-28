package selfupdate

import (
	"fmt"
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

	// @AX:NOTE: [AUTO] os.Rename is atomic only within the same filesystem — will fail cross-device (e.g., tmpfs → /usr/local/bin)
	if err := os.Rename(newBinaryPath, targetPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied replacing %s: %w. Check directory permissions", targetPath, err)
		}
		return err
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

	return nil
}
