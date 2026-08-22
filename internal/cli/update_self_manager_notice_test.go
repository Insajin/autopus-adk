package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

// TestWarnManagerOwnedSelfUpdate_DisclosesTheOwningManager pins the disclosure a
// --check caller needs. The ownership gate sits in the replacement path, which
// --check returns before reaching, so without this notice the probe advertises a
// release the very next `--self` run refuses — and on an install that predates
// the gate, invites overwriting a manager-owned binary.
func TestWarnManagerOwnedSelfUpdate_DisclosesTheOwningManager(t *testing.T) {
	slotPath := filepath.Join(
		t.TempDir(), "co.autopus.desktop", "managed-adk", "current", "auto")

	stubBinaryPath(t, slotPath)

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(&out)

	warnManagerOwnedSelfUpdate(cmd)

	assert.Contains(t, out.String(), "manager-required")
	assert.Contains(t, out.String(), slotPath,
		"the operator must be told which install is blocked")
}

// TestWarnManagerOwnedSelfUpdate_StaysSilentOnASelfOwnedBinary keeps the notice
// off the ordinary path: a binary the CLI can replace must print nothing.
func TestWarnManagerOwnedSelfUpdate_StaysSilentOnASelfOwnedBinary(t *testing.T) {
	stubBinaryPath(t, filepath.Join(t.TempDir(), "auto"))

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(&out)

	warnManagerOwnedSelfUpdate(cmd)

	assert.Empty(t, out.String())
}

// TestWarnManagerOwnedSelfUpdate_StaysSilentWhenPathResolutionFails keeps a probe
// a probe: an unreadable executable path must not become check output.
func TestWarnManagerOwnedSelfUpdate_StaysSilentWhenPathResolutionFails(t *testing.T) {
	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})
	currentExecutablePath = func() (string, error) { return "", assert.AnError }
	evalBinarySymlinks = func(path string) (string, error) { return path, nil }

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(&out)

	warnManagerOwnedSelfUpdate(cmd)

	assert.Empty(t, out.String())
}

// TestReportSelfUpdateAvailability_NeverAnnouncesAloneOnAManagedInstall binds the
// two lines together. Dropping the disclosure from the report puts the misleading
// bare "업데이트 가능" back on a Homebrew or Autopus Desktop install.
func TestReportSelfUpdateAvailability_NeverAnnouncesAloneOnAManagedInstall(t *testing.T) {
	slotPath := filepath.Join(t.TempDir(), "managed-adk", "current", "auto")
	stubBinaryPath(t, slotPath)

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(&out)

	// Deliberately not a release coordinate: this test asserts report shape, and
	// the repo's coordinate transition sweeps every literal that looks like one.
	reportSelfUpdateAvailability(cmd, "0.1.0", &selfupdate.ReleaseInfo{TagName: "v0.2.0"})

	report := out.String()
	require.Contains(t, report, "업데이트 가능: v0.1.0 → v0.2.0")
	assert.Contains(t, report, "manager-required",
		"an announced release the CLI cannot install must say who owns the binary")
	assert.Less(t, strings.Index(report, "업데이트 가능"), strings.Index(report, "manager-required"),
		"the disclosure qualifies the announcement, so it follows it")
}

// TestReportSelfUpdateAvailability_SaysNothingWithoutARelease keeps --check quiet
// when the checker found no newer release.
func TestReportSelfUpdateAvailability_SaysNothingWithoutARelease(t *testing.T) {
	stubBinaryPath(t, filepath.Join(t.TempDir(), "managed-adk", "current", "auto"))

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(&out)

	reportSelfUpdateAvailability(cmd, "0.1.0", nil)

	assert.Empty(t, out.String())
}

// stubBinaryPath points the ownership seams at path for one test.
func stubBinaryPath(t *testing.T, path string) {
	t.Helper()
	originalExecutable := currentExecutablePath
	originalEval := evalBinarySymlinks
	t.Cleanup(func() {
		currentExecutablePath = originalExecutable
		evalBinarySymlinks = originalEval
	})
	currentExecutablePath = func() (string, error) { return path, nil }
	evalBinarySymlinks = func(candidate string) (string, error) { return candidate, nil }
}
