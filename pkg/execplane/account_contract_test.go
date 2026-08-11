package execplane_test

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveExecutionAccountPinsTheActiveSelection covers S9 / REQ-003: when
// the process plane has an active selection, that account is the execution
// account. The roster lists the active account second on purpose, so an
// implementation that reaches for accounts[0] fails here rather than passing by
// accident on a single-account fixture.
func TestResolveExecutionAccountPinsTheActiveSelection(t *testing.T) {
	t.Parallel()

	payload := fixtureListing(t, map[string]any{
		execplane.ProviderCodex: fixtureProviderBlock("acct-second", "host-cli@example.test",
			fixtureAccountEntry("acct-first", "first@example.test", "org-first"),
			fixtureAccountEntry("acct-second", "second@example.test", "org-second"),
		),
	})

	resolution, err := execplane.ResolveExecutionAccount(payload, execplane.ProviderCodex)
	require.NoError(t, err)

	assert.Equal(t, execplane.AccountActive, resolution.Status)
	assert.True(t, resolution.Determined())
	assert.Equal(t, "acct-second", resolution.Account.ID)
	assert.Equal(t, "second@example.test", resolution.Account.Email)
	// The host CLI login travels as the probe account and never as the
	// execution account (acceptance.md S9, final Then).
	assert.Equal(t, "host-cli@example.test", resolution.Probe.Email)
	assert.NotEqual(t, resolution.Account.Email, resolution.Probe.Email)
}

// TestResolveExecutionAccountAdoptsTheSoleRegisteredAccount covers S9 /
// REQ-003: with no active selection and exactly one registered account, that
// account is adopted and the resolution records that it was adopted rather than
// selected. This is the branch the workstation's Claude roster sits in, which
// is why it is reachable — the fixture stays synthetic so the test survives a
// roster change.
func TestResolveExecutionAccountAdoptsTheSoleRegisteredAccount(t *testing.T) {
	t.Parallel()

	payload := fixtureListing(t, map[string]any{
		execplane.ProviderClaude: fixtureProviderBlock(nil, "",
			fixtureAccountEntry("acct-only", "only@example.test", "org-only"),
		),
	})

	resolution, err := execplane.ResolveExecutionAccount(payload, execplane.ProviderClaude)
	require.NoError(t, err)

	assert.Equal(t, execplane.AccountSoleRegistered, resolution.Status)
	assert.True(t, resolution.Determined())
	assert.Equal(t, "only@example.test", resolution.Account.Email)
	assert.NotEmpty(t, resolution.Reason,
		"adopting an account without an active selection is a fact the receipt must carry")
	// Claude exposes no host CLI identity, so there is nothing to compare against.
	assert.Equal(t, execplane.Account{}, resolution.Probe)
}

// TestResolveExecutionAccountRefusesToGuess covers S9 / REQ-003 and REQ-009:
// with no active selection and a roster that is not exactly one account, the
// gate reports indeterminate, carries no account at all, and says why. Choosing
// the first or the most recent registered account — the tempting shortcut that
// produces silent downgrades — fails the zero-account assertion.
func TestResolveExecutionAccountRefusesToGuess(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		"no registered account": fixtureProviderBlock(nil, "host-cli@example.test"),
		"two registered accounts": fixtureProviderBlock(nil, "host-cli@example.test",
			fixtureAccountEntry("acct-first", "first@example.test", "org-first"),
			fixtureAccountEntry("acct-second", "second@example.test", "org-second"),
		),
		"three registered accounts": fixtureProviderBlock(nil, "host-cli@example.test",
			fixtureAccountEntry("acct-first", "first@example.test", "org-first"),
			fixtureAccountEntry("acct-second", "second@example.test", "org-second"),
			fixtureAccountEntry("acct-third", "third@example.test", "org-third"),
		),
	}

	reasons := make(map[string]string, len(cases))
	for name, block := range cases {
		payload := fixtureListing(t, map[string]any{execplane.ProviderCodex: block})

		resolution, err := execplane.ResolveExecutionAccount(payload, execplane.ProviderCodex)
		require.NoError(t, err, name)

		assert.Equal(t, execplane.AccountIndeterminate, resolution.Status, name)
		assert.False(t, resolution.Determined(), name)
		assert.Equal(t, execplane.Account{}, resolution.Account,
			"%s: an ambiguous roster must not resolve to any member", name)
		assert.NotEmpty(t, resolution.Reason, name)
		// The host CLI identity is available and still is not used as a
		// fallback execution account (acceptance.md S9, final Then).
		assert.Equal(t, "host-cli@example.test", resolution.Probe.Email, name)
		assert.NotEqual(t, resolution.Probe.Email, resolution.Account.Email, name)

		reasons[name] = resolution.Reason
	}

	assert.NotEqual(t, reasons["no registered account"], reasons["two registered accounts"],
		"an empty roster and an ambiguous roster are different situations and must read differently")
}

// TestResolveExecutionAccountRejectsUnusableListings covers S9 / REQ-003: a
// listing the gate cannot trust is an error, not an empty roster that silently
// degrades into indeterminate. A provider orca does not report at all is the
// one non-error case, and it still carries a reason.
func TestResolveExecutionAccountRejectsUnusableListings(t *testing.T) {
	t.Parallel()

	t.Run("listing reported failure", func(t *testing.T) {
		t.Parallel()

		_, err := execplane.ResolveExecutionAccount(
			[]byte(`{"ok":false,"error":"account service unavailable"}`), execplane.ProviderCodex)
		require.Error(t, err)
	})

	t.Run("malformed json", func(t *testing.T) {
		t.Parallel()

		_, err := execplane.ResolveExecutionAccount([]byte(`{"ok":true,`), execplane.ProviderCodex)
		require.Error(t, err)
	})

	t.Run("provider absent from roster", func(t *testing.T) {
		t.Parallel()

		payload := fixtureListing(t, map[string]any{
			execplane.ProviderClaude: fixtureProviderBlock(nil, "",
				fixtureAccountEntry("acct-only", "only@example.test", "org-only")),
		})

		resolution, err := execplane.ResolveExecutionAccount(payload, execplane.ProviderCodex)
		require.NoError(t, err)
		assert.Equal(t, execplane.AccountIndeterminate, resolution.Status)
		assert.Equal(t, execplane.Account{}, resolution.Account)
		assert.NotEmpty(t, resolution.Reason)
	})
}

// TestEvaluateLeavesExecutionAccountEmptyWhenIndeterminate covers S9 / REQ-003
// and REQ-009: an unresolved account short-circuits the receipt even when both
// entitlement grades are known and equal. Matching grades are evidence about
// the catalog, not about who runs the workload, so they must not promote an
// indeterminate resolution to verified.
func TestEvaluateLeavesExecutionAccountEmptyWhenIndeterminate(t *testing.T) {
	t.Parallel()

	resolution := execplane.AccountResolution{
		Provider: execplane.ProviderCodex,
		Status:   execplane.AccountIndeterminate,
		Probe:    execplane.Account{Email: "host-cli@example.test", Org: "org-host-cli"},
		Reason:   "no active account selected and 2 accounts registered",
	}
	matched := execplane.Entitlement{Grade: "pro", Source: "host-cli@example.test"}

	receipt := execplane.Evaluate(
		execplane.TierRequest{
			Provider:      execplane.ProviderCodex,
			RequestedTier: "ultra",
			ResolvedModel: "gpt-5.6-sol",
		},
		resolution, matched, matched, time.Now(),
	)

	assert.Empty(t, receipt.ExecutionAccount,
		"an unresolved account must stay unnamed rather than borrow the probe account")
	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
	assert.Equal(t, resolution.Reason, receipt.ResolutionReason)
}
