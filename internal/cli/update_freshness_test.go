package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

func TestUpdateFreshness_CurrentReleaseProceedsAfterOneCheck(t *testing.T) {
	checks := 0
	gate := updateFreshnessGate{deps: freshnessTestDeps("0.50.90", func(current string) (*selfupdate.ReleaseInfo, error) {
		checks++
		assert.Equal(t, "0.50.90", current)
		return nil, nil
	})}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := gate.enforce(cmd, false, "")

	require.NoError(t, err)
	assert.Equal(t, updateFreshnessProceed, outcome)
	assert.Equal(t, 1, checks)
}

func TestUpdateFreshness_StalePreviewMakesDecisionWithoutMutation(t *testing.T) {
	installed, reexecuted, resolved := false, false, false
	deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
	deps.resolveBinary = func() (binaryPathInfo, error) {
		resolved = true
		return binaryPathInfo{}, nil
	}
	deps.install = func(*cobra.Command, string, *selfupdate.ReleaseInfo, binaryPathInfo) error {
		installed = true
		return nil
	}
	deps.reexec = func(*cobra.Command, string, []string, string) error {
		reexecuted = true
		return nil
	}
	cmd, output := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, true, "")

	require.NoError(t, err)
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.Contains(t, output.String(), "v0.50.90 → v0.50.91")
	assert.Contains(t, output.String(), "re-exec")
	assert.False(t, resolved)
	assert.False(t, installed)
	assert.False(t, reexecuted)
}

func TestUpdateFreshness_StaleReleaseInstallsThenReexecutesOnce(t *testing.T) {
	var calls []string
	deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
	deps.resolveBinary = func() (binaryPathInfo, error) {
		calls = append(calls, "resolve")
		return binaryPathInfo{ExecutablePath: "/usr/local/bin/auto", ResolvedPath: "/opt/auto/0.50.90/auto"}, nil
	}
	deps.newNonce = func() (string, error) {
		calls = append(calls, "nonce")
		return strings.Repeat("a", updateFreshnessNonceHexLength), nil
	}
	deps.install = func(_ *cobra.Command, current string, info *selfupdate.ReleaseInfo, path binaryPathInfo) error {
		calls = append(calls, "install")
		assert.Equal(t, "0.50.90", current)
		assert.Equal(t, "v0.50.91", info.TagName)
		assert.Equal(t, "/opt/auto/0.50.90/auto", path.ManagedPath())
		return nil
	}
	deps.invocationArgs = func() []string {
		return []string{"update", "--dir", "/workspace"}
	}
	deps.reexec = func(_ *cobra.Command, binary string, args []string, nonce string) error {
		calls = append(calls, "reexec")
		assert.Equal(t, "/opt/auto/0.50.90/auto", binary)
		assert.Equal(t, strings.Repeat("a", updateFreshnessNonceHexLength), nonce)
		assert.Equal(t, []string{
			"update", "--dir", "/workspace",
			"--" + updateFreshnessReexecFlag + "=" + nonce,
		}, args)
		return nil
	}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

	require.NoError(t, err)
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.Equal(t, []string{"resolve", "nonce", "install", "reexec"}, calls)
}

func TestUpdateFreshness_CheckFailureStopsBeforeAnyWrite(t *testing.T) {
	sentinel := errors.New("release service unavailable")
	mutated := false
	deps := freshnessTestDeps("0.50.90", func(string) (*selfupdate.ReleaseInfo, error) {
		return nil, sentinel
	})
	deps.resolveBinary = func() (binaryPathInfo, error) {
		mutated = true
		return binaryPathInfo{}, nil
	}
	deps.install = func(*cobra.Command, string, *selfupdate.ReleaseInfo, binaryPathInfo) error {
		mutated = true
		return nil
	}
	deps.reexec = func(*cobra.Command, string, []string, string) error {
		mutated = true
		return nil
	}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.False(t, mutated)
}

func TestUpdateFreshness_StaleDesktopManagedBinaryRequiresManager(t *testing.T) {
	installed := false
	deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
	deps.resolveBinary = func() (binaryPathInfo, error) {
		return binaryPathInfo{
			ExecutablePath: "/.vol/16777231/99123",
			ResolvedPath:   "/.vol/16777231/99123",
		}, nil
	}
	deps.install = func(*cobra.Command, string, *selfupdate.ReleaseInfo, binaryPathInfo) error {
		installed = true
		return nil
	}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "manager-required")
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.False(t, installed)
}

func TestUpdateFreshness_ReexecNonceRejectsTamperingAndReplay(t *testing.T) {
	nonce := strings.Repeat("b", updateFreshnessNonceHexLength)
	env := map[string]string{updateFreshnessReexecEnv: nonce}
	checks := 0
	deps := freshnessTestDeps("0.50.90", func(string) (*selfupdate.ReleaseInfo, error) {
		checks++
		return nil, nil
	})
	deps.lookupEnv = func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
	deps.unsetEnv = func(key string) error {
		delete(env, key)
		return nil
	}
	gate := updateFreshnessGate{deps: deps}

	tamperedCmd, _ := newFreshnessTestCommand()
	_, err := gate.enforce(tamperedCmd, false, strings.Repeat("c", updateFreshnessNonceHexLength))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonce")
	assert.Equal(t, 0, checks)

	validCmd, _ := newFreshnessTestCommand()
	outcome, err := gate.enforce(validCmd, false, nonce)
	require.NoError(t, err)
	assert.Equal(t, updateFreshnessProceed, outcome)
	assert.Equal(t, 1, checks)

	replayCmd, _ := newFreshnessTestCommand()
	_, err = gate.enforce(replayCmd, false, nonce)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonce")
	assert.Equal(t, 1, checks)
}

func TestUpdateFreshness_WorkspaceContextChecksOnlyOnce(t *testing.T) {
	checks := 0
	deps := freshnessTestDeps("0.50.90", func(string) (*selfupdate.ReleaseInfo, error) {
		checks++
		return nil, nil
	})
	gate := updateFreshnessGate{deps: deps}
	parent, _ := newFreshnessTestCommand()
	outcome, err := gate.enforce(parent, false, "")
	require.NoError(t, err)
	require.Equal(t, updateFreshnessProceed, outcome)

	nested, _ := newFreshnessTestCommand()
	nested.SetContext(parent.Context())
	outcome, err = gate.enforce(nested, false, "")

	require.NoError(t, err)
	assert.Equal(t, updateFreshnessProceed, outcome)
	assert.Equal(t, 1, checks)
}

func TestUpdateCmd_VersionFlagFailsExplicitly(t *testing.T) {
	cmd := newUpdateCmd()
	cmd.SetArgs([]string{"--version", "0.50.90"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--version")
	assert.Contains(t, err.Error(), "지원되지")
}

func TestUpdateCmd_ReexecFlagCannotRepeat(t *testing.T) {
	nonce := strings.Repeat("d", updateFreshnessNonceHexLength)
	cmd := newUpdateCmd()
	cmd.SetArgs([]string{
		"--" + updateFreshnessReexecFlag + "=" + nonce,
		"--" + updateFreshnessReexecFlag + "=" + nonce,
	})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "한 번")
}

func staleReleaseCheck(current string) (*selfupdate.ReleaseInfo, error) {
	if current != "0.50.90" {
		return nil, fmt.Errorf("unexpected current version %q", current)
	}
	return &selfupdate.ReleaseInfo{
		TagName:      "v0.50.91",
		ArchiveURL:   "https://example.test/auto.tar.gz",
		ChecksumURL:  "https://example.test/checksums.txt",
		SignatureURL: "https://example.test/checksums.txt.signatures",
	}, nil
}

func freshnessTestDeps(
	current string,
	check func(string) (*selfupdate.ReleaseInfo, error),
) updateFreshnessDeps {
	return updateFreshnessDeps{
		currentVersion: func() string { return current },
		checkLatest:    check,
		resolveBinary: func() (binaryPathInfo, error) {
			return binaryPathInfo{ExecutablePath: "/usr/local/bin/auto"}, nil
		},
		install: func(*cobra.Command, string, *selfupdate.ReleaseInfo, binaryPathInfo) error {
			return nil
		},
		reexec: func(*cobra.Command, string, []string, string) error { return nil },
		invocationArgs: func() []string {
			return []string{"update"}
		},
		newNonce: func() (string, error) {
			return strings.Repeat("e", updateFreshnessNonceHexLength), nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		unsetEnv:  func(string) error { return nil },
	}
}

func newFreshnessTestCommand() (*cobra.Command, *bytes.Buffer) {
	output := &bytes.Buffer{}
	cmd := &cobra.Command{Use: "update"}
	cmd.SetOut(output)
	cmd.SetErr(output)
	return cmd, output
}
