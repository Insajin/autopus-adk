//go:build darwin

package selfupdate

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
