package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

func TestAppendUpdateFreshnessFlag_InsertsBeforeArgumentTerminator(t *testing.T) {
	nonce := "nonce"

	got := appendUpdateFreshnessFlag(
		[]string{"update", "--dir", "/workspace", "--", "literal"},
		nonce,
	)

	assert.Equal(t, []string{
		"update", "--dir", "/workspace",
		"--" + updateFreshnessReexecFlag + "=" + nonce,
		"--", "literal",
	}, got)
}

func TestUpdateFreshness_MalformedLatestReleaseFailsClosed(t *testing.T) {
	mutated := false
	deps := freshnessTestDeps("0.50.90", func(string) (*selfupdate.ReleaseInfo, error) {
		return &selfupdate.ReleaseInfo{TagName: "nightly"}, nil
	})
	deps.resolveBinary = func() (binaryPathInfo, error) {
		mutated = true
		return binaryPathInfo{}, nil
	}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stable")
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.False(t, mutated)
}

func TestUpdateFreshness_PrivilegedInstallFailureStopsBeforeReexec(t *testing.T) {
	sentinel := errors.New("privileged-update-required")
	reexecuted := false
	deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
	deps.install = func(
		*cobra.Command,
		string,
		*selfupdate.ReleaseInfo,
		binaryPathInfo,
	) error {
		return sentinel
	}
	deps.reexec = func(*cobra.Command, string, []string, string) error {
		reexecuted = true
		return nil
	}
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.False(t, reexecuted)
}

func TestUpdateFreshness_FailureBranchesStopBeforeHarness(t *testing.T) {
	sentinel := errors.New("injected failure")
	tests := []struct {
		name   string
		mutate func(*updateFreshnessDeps)
	}{
		{
			name: "orphan nonce environment",
			mutate: func(deps *updateFreshnessDeps) {
				deps.lookupEnv = func(string) (string, bool) { return "orphan", true }
			},
		},
		{
			name: "binary resolution",
			mutate: func(deps *updateFreshnessDeps) {
				deps.resolveBinary = func() (binaryPathInfo, error) {
					return binaryPathInfo{}, sentinel
				}
			},
		},
		{
			name: "nonce entropy",
			mutate: func(deps *updateFreshnessDeps) {
				deps.newNonce = func() (string, error) { return "", sentinel }
			},
		},
		{
			name: "invalid generated nonce",
			mutate: func(deps *updateFreshnessDeps) {
				deps.newNonce = func() (string, error) { return "invalid", nil }
			},
		},
		{
			name: "self update",
			mutate: func(deps *updateFreshnessDeps) {
				deps.install = func(
					*cobra.Command,
					string,
					*selfupdate.ReleaseInfo,
					binaryPathInfo,
				) error {
					return sentinel
				}
			},
		},
		{
			name: "new binary reexec",
			mutate: func(deps *updateFreshnessDeps) {
				deps.reexec = func(*cobra.Command, string, []string, string) error {
					return sentinel
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
			test.mutate(&deps)
			cmd, _ := newFreshnessTestCommand()

			outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, "")

			require.Error(t, err)
			assert.Equal(t, updateFreshnessStop, outcome)
		})
	}
}

func TestUpdateFreshness_ReexecNonceUnsetFailureIsFatal(t *testing.T) {
	nonce := strings.Repeat("a", updateFreshnessNonceHexLength)
	sentinel := errors.New("cannot consume environment")
	deps := freshnessTestDeps("0.50.90", staleReleaseCheck)
	deps.lookupEnv = func(string) (string, bool) { return nonce, true }
	deps.unsetEnv = func(string) error { return sentinel }
	cmd, _ := newFreshnessTestCommand()

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, nonce)

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, updateFreshnessStop, outcome)
}

func TestUpdateFreshness_CallerSuppliedMatchingNonceCannotBypassLatestCheck(t *testing.T) {
	nonce := strings.Repeat("b", updateFreshnessNonceHexLength)
	sentinel := errors.New("official release check unavailable")
	checks := 0
	mutated := false
	deps := freshnessTestDeps("0.50.90", func(string) (*selfupdate.ReleaseInfo, error) {
		checks++
		return nil, sentinel
	})
	deps.lookupEnv = func(string) (string, bool) { return nonce, true }
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

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, nonce)

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.Equal(t, 1, checks)
	assert.False(t, mutated)
}

func TestUpdateFreshness_ReexecMarkerStopsStaleChildWithoutSecondUpdate(t *testing.T) {
	nonce := strings.Repeat("c", updateFreshnessNonceHexLength)
	checks := 0
	mutated := false
	deps := freshnessTestDeps("0.50.90", func(current string) (*selfupdate.ReleaseInfo, error) {
		checks++
		return staleReleaseCheck(current)
	})
	deps.lookupEnv = func(string) (string, bool) { return nonce, true }
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

	outcome, err := (updateFreshnessGate{deps: deps}).enforce(cmd, false, nonce)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "official latest")
	assert.Equal(t, updateFreshnessStop, outcome)
	assert.Equal(t, 1, checks)
	assert.False(t, mutated)
}

func TestUpdateFreshness_VersionAndNonceValidators(t *testing.T) {
	currentTests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{raw: "v0.50.90-dirty", want: "0.50.90", ok: true},
		{raw: "dev", ok: false},
		{raw: "0.50", ok: false},
		{raw: "0.x.90", ok: false},
		{raw: "0..90", ok: false},
	}
	for _, test := range currentTests {
		got, ok := stableCurrentVersion(test.raw)
		assert.Equal(t, test.ok, ok, test.raw)
		assert.Equal(t, test.want, got, test.raw)
	}

	latestTests := []struct {
		raw string
		ok  bool
	}{
		{raw: "v0.50.91", ok: true},
		{raw: "0.50.91", ok: true},
		{raw: "v0.50.91-rc1", ok: false},
		{raw: " v0.50.91", ok: false},
		{raw: "", ok: false},
	}
	for _, test := range latestTests {
		_, ok := stableLatestVersion(test.raw)
		assert.Equal(t, test.ok, ok, test.raw)
	}

	assert.True(t, validUpdateFreshnessNonce(strings.Repeat("f", updateFreshnessNonceHexLength)))
	assert.False(t, validUpdateFreshnessNonce(strings.Repeat("F", updateFreshnessNonceHexLength)))
	assert.False(t, validUpdateFreshnessNonce("short"))
}

func TestUpdateFreshness_NonceAndFlagMetadata(t *testing.T) {
	nonce, err := newUpdateFreshnessNonce()
	require.NoError(t, err)
	assert.True(t, validUpdateFreshnessNonce(nonce))

	flag := &updateFreshnessNonceFlag{}
	assert.Equal(t, "", flag.String())
	assert.Equal(t, "nonce", flag.Type())
}
