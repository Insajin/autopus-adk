package execplane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The account identifier arrives from a subprocess payload, so it is validated
// before it is joined into a path rather than after.
func TestManagedCodexAuthPathRejectsUnsafeAccountID(t *testing.T) {
	t.Parallel()

	unsafe := []string{
		"..",
		"../../etc",
		probeExecAccountID + "/../../..",
		"11111111-1111-4111-8111-111111111111\x00",
		"not-a-uuid",
		"",
	}
	for _, accountID := range unsafe {
		path, err := managedCodexAuthPath("/support", accountID)
		require.ErrorIsf(t, err, ErrUnsafeAccountID, "account id %q", accountID)
		assert.Empty(t, path)
	}

	assert.NotContains(t, managedCodexAuthPathError(t, "../../etc").Error(), "../../etc")
}

func TestManagedCodexAuthPathBuildsManagedHomePath(t *testing.T) {
	t.Parallel()

	path, err := managedCodexAuthPath("/support", probeExecAccountID)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/support", "orca", "codex-accounts",
		probeExecAccountID, "home", "auth.json"), path)

	_, err = managedCodexAuthPath("", probeExecAccountID)
	require.ErrorIs(t, err, ErrCredentialUnavailable)
}

func TestReadManagedCodexAuthRefusesTraversalBeforeTouchingDisk(t *testing.T) {
	t.Parallel()

	payload, err := readManagedCodexAuth("../../../etc/passwd")
	require.ErrorIs(t, err, ErrUnsafeAccountID)
	assert.Nil(t, payload)
}

func TestLocalCodexAuthPathHonorsCodexHome(t *testing.T) {
	t.Parallel()

	path, err := localCodexAuthPath("/custom/codex", "/home/user")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/custom/codex", "auth.json"), path)

	path, err = localCodexAuthPath("", "/home/user")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/home/user", ".codex", "auth.json"), path)

	_, err = localCodexAuthPath("", "")
	require.ErrorIs(t, err, ErrCredentialUnavailable)
}

func TestReadLocalCodexAuthReadsCodexHomeCredential(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv(codexHomeEnv, codexHome)
	probeWriteLocalAuth(t, codexHome, "pro")

	payload, err := readLocalCodexAuth()
	require.NoError(t, err)
	entitlement, err := ParseCodexEntitlement(payload, probeCredentialScope)
	require.NoError(t, err)
	assert.Equal(t, "pro", entitlement.Grade)
}

// A missing credential is reported, not fatal, and the reason names the
// location without echoing a path that embeds the account identifier.
func TestReadCredentialFileReportsMissingFileWithoutLeakingPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), probeExecAccountID, "home", "auth.json")

	payload, err := readCredentialFile(path, executionCredentialScope)
	require.ErrorIs(t, err, ErrCredentialUnavailable)
	assert.Nil(t, payload)
	assert.Contains(t, err.Error(), executionCredentialScope)
	assert.NotContains(t, err.Error(), path)
	assert.NotContains(t, err.Error(), probeExecAccountID)
}

func TestReadCredentialFileRefusesOversizedCredential(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "auth.json")
	require.NoError(t, os.WriteFile(path,
		[]byte(strings.Repeat("x", maxCredentialBytes+1)), 0o600))

	payload, err := readCredentialFile(path, probeCredentialScope)
	require.ErrorIs(t, err, ErrCredentialUnavailable)
	assert.Nil(t, payload)
	assert.Contains(t, err.Error(), "exceeds")
}

func managedCodexAuthPathError(t *testing.T, accountID string) error {
	t.Helper()

	_, err := managedCodexAuthPath("/support", accountID)
	require.Error(t, err)
	return err
}
