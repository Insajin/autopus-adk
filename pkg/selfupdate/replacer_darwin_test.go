//go:build darwin

package selfupdate

import (
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestReplace_DarwinPreservesDownloadedSignatureBytes(t *testing.T) {
	fixtureID := "co.autopus.selfupdate.fixture"
	executable, err := os.Executable()
	require.NoError(t, err)

	newBinaryPath := filepath.Join(t.TempDir(), "signed-auto")
	require.NoError(t, copyFile(executable, newBinaryPath))
	require.NoError(t, exec.Command(
		"codesign", "--force", "--sign", "-", "--identifier", fixtureID, newBinaryPath,
	).Run())
	expected, err := os.ReadFile(newBinaryPath)
	require.NoError(t, err)

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0755))

	require.NoError(t, NewReplacer().Replace(newBinaryPath, targetPath))

	actual, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(expected), sha256.Sum256(actual),
		"replacement must preserve the downloaded code signature bytes")
	require.NoError(t, exec.Command("codesign", "--verify", "--strict", targetPath).Run())
	details, err := exec.Command("codesign", "-dv", "--verbose=2", targetPath).CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(details), "Identifier="+fixtureID)
}

func TestReplace_DarwinRemovesOnlyUpdateBlockingXattrs(t *testing.T) {
	const (
		customAttr     = "co.autopus.selfupdate.keep"
		quarantineAttr = "com.apple.quarantine"
	)

	newBinaryPath := filepath.Join(t.TempDir(), "auto-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new binary"), 0755))
	require.NoError(t, unix.Setxattr(newBinaryPath, customAttr, []byte("keep"), 0))
	require.NoError(t, unix.Setxattr(
		newBinaryPath,
		quarantineAttr,
		[]byte("0081;00000000;Autopus;"),
		0,
	))

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0755))
	require.NoError(t, NewReplacer().Replace(newBinaryPath, targetPath))

	customValue := make([]byte, 16)
	size, err := unix.Getxattr(targetPath, customAttr, customValue)
	require.NoError(t, err)
	require.Equal(t, "keep", string(customValue[:size]))
	_, err = unix.Getxattr(targetPath, quarantineAttr, nil)
	require.ErrorIs(t, err, unix.ENOATTR)
}

func TestReplace_DarwinCrossDeviceCopyPreservesUnrelatedXattrs(t *testing.T) {
	const customAttr = "co.autopus.selfupdate.crossdevice"

	newBinaryPath := filepath.Join(t.TempDir(), "auto-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new binary"), 0755))
	require.NoError(t, unix.Setxattr(newBinaryPath, customAttr, []byte("keep"), 0))
	require.NoError(t, unix.Setxattr(
		newBinaryPath,
		"com.apple.quarantine",
		[]byte("0081;00000000;Autopus;"),
		0,
	))
	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0755))
	ops := defaultReplaceOps()
	ops.rename = func(_, _ string) error {
		return &os.LinkError{Op: "rename", Err: unix.EXDEV}
	}

	require.NoError(t, replaceWithOps(newBinaryPath, targetPath, ops))
	customValue := make([]byte, 16)
	size, err := unix.Getxattr(targetPath, customAttr, customValue)
	require.NoError(t, err)
	require.Equal(t, "keep", string(customValue[:size]))
	_, err = unix.Getxattr(targetPath, "com.apple.quarantine", nil)
	require.ErrorIs(t, err, unix.ENOATTR)
}

func TestClearUpdateXattrs_DarwinUsesExactNames(t *testing.T) {
	var removed []string
	err := clearUpdateXattrsWith("auto", func(_, name string) error {
		removed = append(removed, name)
		return unix.ENOATTR
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"com.apple.quarantine",
		"com.apple.provenance",
	}, removed)
}

func TestClearUpdateXattrs_DarwinAllowsUnsupportedFilesystem(t *testing.T) {
	err := clearUpdateXattrsWith("auto", func(_, _ string) error {
		return unix.ENOTSUP
	})
	require.NoError(t, err)
}

func TestClearUpdateXattrs_DarwinRejectsUnexpectedRemovalError(t *testing.T) {
	sentinel := errors.New("injected removal failure")
	err := clearUpdateXattrsWith("auto", func(_, _ string) error {
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
}

func TestValidateXattrSize_DarwinRejectsOversizedRetry(t *testing.T) {
	require.NoError(t, validateXattrSize("value", maxPreservedXattrBytes))
	require.ErrorContains(
		t,
		validateXattrSize("value", maxPreservedXattrBytes+1),
		"xattr value exceeds",
	)
}
