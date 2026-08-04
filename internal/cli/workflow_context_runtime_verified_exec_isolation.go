package cli

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

func workflowContextReleaseCanaryExpectedUID() (uint32, error) {
	account, err := user.Lookup("nobody")
	if err != nil || account.Username != "nobody" {
		return 0, errors.New("release canary nobody identity is unavailable")
	}
	value, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return 0, errors.New("release canary nobody UID is invalid")
	}
	return uint32(value), nil
}

func validateWorkflowContextReleaseCanaryIsolation(
	root, executable string,
	expectedUID, trustedUID uint32,
) error {
	effectiveUID, err := workflowContextCanaryEffectiveUID()
	if err != nil || effectiveUID != expectedUID {
		return errors.New("release canary effective user is not nobody")
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("release canary root must be an absolute canonical path")
	}
	if err := validateWorkflowContextCanaryDirectory(root, trustedUID, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"home", "tmp"} {
		if err := validateWorkflowContextCanaryDirectory(filepath.Join(root, name), expectedUID, 0o700); err != nil {
			return err
		}
	}
	if os.Getenv("HOME") != filepath.Join(root, "home") || os.Getenv("TMPDIR") != filepath.Join(root, "tmp") {
		return errors.New("release canary HOME and TMPDIR are not isolated")
	}
	if executable != filepath.Join(root, "omp-darwin-arm64") {
		return errors.New("release canary OMP path is outside the isolated root")
	}
	info, err := os.Lstat(executable)
	uid, uidErr := workflowContextCanaryFileUID(info)
	if err != nil || uidErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != 0o555 || uid != trustedUID {
		return errors.New("release canary OMP ownership or mode is unsafe")
	}
	return nil
}

func validateWorkflowContextCanaryDirectory(path string, expectedUID uint32, expectedMode os.FileMode) error {
	info, err := os.Lstat(path)
	uid, uidErr := workflowContextCanaryFileUID(info)
	if err != nil || uidErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != expectedMode || uid != expectedUID {
		return errors.New("release canary directory ownership or mode is unsafe")
	}
	return nil
}
