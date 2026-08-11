package execplane_test

import (
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureTierRequest is the policy plane's ask, carried into the gate unchanged.
func fixtureTierRequest(provider, tier, model string) execplane.TierRequest {
	return execplane.TierRequest{Provider: provider, RequestedTier: tier, ResolvedModel: model}
}

// fixtureDeterminedAccount is a resolution that names exactly one execution account.
func fixtureDeterminedAccount(provider, email string) execplane.AccountResolution {
	return execplane.AccountResolution{
		Provider: provider,
		Status:   execplane.AccountActive,
		Account:  execplane.Account{ID: "acct-" + email, Email: email},
		Probe:    execplane.Account{Email: "host-cli@example.test"},
	}
}

// fixtureVerifiedReceipt is the only receipt shape that may carry
// StatusVerified: a determined account whose grade equals the grade the held
// catalog was probed under.
func fixtureVerifiedReceipt(t *testing.T) execplane.IntegrityReceipt {
	t.Helper()

	receipt := execplane.Evaluate(
		fixtureTierRequest(execplane.ProviderCodex, "ultra", "gpt-5.6-sol"),
		fixtureDeterminedAccount(execplane.ProviderCodex, "execution@example.test"),
		execplane.Entitlement{Grade: "pro", Source: "execution@example.test"},
		execplane.Entitlement{Grade: "pro", Source: "host-cli@example.test"},
		time.Now(),
	)
	require.Equal(t, execplane.StatusVerified, receipt.VerificationStatus)
	return receipt
}

// TestVerifiedReceiptCarriesAllSixFields covers S4 / REQ-005: requested tier,
// resolved provider model, execution account, catalog source, resolution
// reason, and verification status are all present, and the receipt is stamped
// with a schema version the way the pipeline's other receipts are.
func TestVerifiedReceiptCarriesAllSixFields(t *testing.T) {
	t.Parallel()

	receipt := fixtureVerifiedReceipt(t)

	// Spelled out rather than compared to the constant alone: a schema bump has
	// to be a deliberate edit, not something a rename carries along silently.
	assert.Equal(t, "execplane_tier_integrity_receipt.v1", receipt.Schema)
	assert.Equal(t, execplane.IntegrityReceiptSchema, receipt.Schema)

	assert.Equal(t, "ultra", receipt.RequestedTier)
	assert.Equal(t, "gpt-5.6-sol", receipt.ResolvedModel)
	assert.Equal(t, "execution@example.test", receipt.ExecutionAccount)
	assert.Equal(t, "host-cli@example.test", receipt.CatalogSource.Account)
	assert.Equal(t, "pro", receipt.CatalogSource.Entitlement)
	assert.NotEmpty(t, receipt.ResolutionReason)
	assert.Equal(t, time.UTC, receipt.CheckedAt.Location())
	assert.True(t, receipt.Complete())
}

// TestIncompleteReceiptIsRejectedFieldByField covers S4 / REQ-005: each field
// is load-bearing, so blanking any one of them makes the receipt unusable
// rather than merely weaker. The catalog-source cases are the ones the spec
// calls out — an account with no entitlement grade leaves a reader unable to
// tell whether that catalog covers the execution account, so five present
// fields are not enough.
func TestIncompleteReceiptIsRejectedFieldByField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		blank func(*execplane.IntegrityReceipt)
	}{
		{"schema tag missing", func(r *execplane.IntegrityReceipt) { r.Schema = "" }},
		{"requested tier missing", func(r *execplane.IntegrityReceipt) { r.RequestedTier = "" }},
		{"resolved model missing", func(r *execplane.IntegrityReceipt) { r.ResolvedModel = "" }},
		{"execution account missing", func(r *execplane.IntegrityReceipt) { r.ExecutionAccount = "" }},
		{"catalog account missing", func(r *execplane.IntegrityReceipt) { r.CatalogSource.Account = "" }},
		{"catalog grade missing", func(r *execplane.IntegrityReceipt) { r.CatalogSource.Entitlement = "" }},
		{"resolution reason missing", func(r *execplane.IntegrityReceipt) { r.ResolutionReason = "" }},
		{"status missing", func(r *execplane.IntegrityReceipt) { r.VerificationStatus = "" }},
	}

	for _, tc := range cases {
		receipt := fixtureVerifiedReceipt(t)
		tc.blank(&receipt)

		assert.False(t, receipt.Complete(), "%s: the receipt must not pass as complete", tc.name)
	}
}

// TestResolutionReasonIsNeverEmpty covers S5 / REQ-006: silence is the failure
// mode the requirement exists to prevent, so the sweep is exhaustive over
// CompareEntitlement's four branches crossed with Evaluate's determined and
// indeterminate branches — including an indeterminate resolution that arrives
// carrying no reason of its own.
func TestResolutionReasonIsNeverEmpty(t *testing.T) {
	t.Parallel()

	entitlements := []execplane.Entitlement{
		{},
		{Grade: "pro", Source: "host-cli@example.test"},
		{Grade: "plus", Source: "host-cli@example.test"},
	}
	resolutions := map[string]execplane.AccountResolution{
		"determined": fixtureDeterminedAccount(execplane.ProviderCodex, "execution@example.test"),
		"indeterminate with reason": {
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountIndeterminate,
			Reason:   "no active account selected and 2 accounts registered",
		},
		"indeterminate with no reason": {
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountIndeterminate,
		},
	}

	for _, execution := range entitlements {
		for _, probe := range entitlements {
			_, reason := execplane.CompareEntitlement(execution, probe)
			assert.NotEmpty(t, reason, "exec %q vs probe %q", execution.Grade, probe.Grade)

			for name, resolution := range resolutions {
				receipt := execplane.Evaluate(
					fixtureTierRequest(execplane.ProviderCodex, "ultra", "gpt-5.6-sol"),
					resolution, execution, probe, time.Now(),
				)
				assert.NotEmpty(t, receipt.ResolutionReason,
					"%s: exec %q vs probe %q", name, execution.Grade, probe.Grade)
			}
		}
	}
}

// TestReceiptNamesRequestedTierAndServedModelSeparately covers S5 / REQ-006: a
// downgrade is only reconstructible when both values survive as separate
// fields, the way the measured pair gpt-5.6-terra -> gpt-5.5 has to be
// nameable on both ends. Flattening them into one field is the silent
// downgrade the requirement forbids.
func TestReceiptNamesRequestedTierAndServedModelSeparately(t *testing.T) {
	t.Parallel()

	receipt := execplane.Evaluate(
		fixtureTierRequest(execplane.ProviderCodex, "ultra", "gpt-5.5"),
		fixtureDeterminedAccount(execplane.ProviderCodex, "execution@example.test"),
		execplane.Entitlement{Grade: "pro", Source: "execution@example.test"},
		execplane.Entitlement{Grade: "plus", Source: "host-cli@example.test"},
		time.Now(),
	)

	assert.Equal(t, "ultra", receipt.RequestedTier)
	assert.Equal(t, "gpt-5.5", receipt.ResolvedModel)
	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
	assert.NotEmpty(t, receipt.ResolutionReason)
	assert.True(t, receipt.Complete(),
		"an unverified receipt is still a complete record of why")
}

// TestDeterminedIdentityWithoutCatalogProbeStaysUnverified covers S8 / REQ-009,
// and it is the combination receipt readers misread most often. Claude has no
// account-scoped catalog probe, so identity can be settled while model
// availability cannot be. A named execution account sitting next to
// `unverified` is the correct state, not a defect — and promoting the tier to
// verified because the identity checked out is the actual defect.
func TestDeterminedIdentityWithoutCatalogProbeStaysUnverified(t *testing.T) {
	t.Parallel()

	resolution := execplane.AccountResolution{
		Provider: execplane.ProviderClaude,
		Status:   execplane.AccountSoleRegistered,
		Account:  execplane.Account{ID: "acct-only", Email: "owner@example.test"},
		Reason:   "no active account selected; adopted the sole registered account",
	}
	require.True(t, resolution.Determined())

	request := fixtureTierRequest(execplane.ProviderClaude, "ultra", "claude-opus-5")
	receipt := execplane.Evaluate(
		request, resolution, execplane.Entitlement{}, execplane.Entitlement{}, time.Now())

	assert.Equal(t, "owner@example.test", receipt.ExecutionAccount,
		"identity is settled even though availability is not")
	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
	assert.NotEmpty(t, receipt.ResolutionReason)
	assert.True(t, receipt.Complete())

	// S8 (no probe available) and S9 (no account resolved) share the status but
	// must stay distinguishable, or a reader cannot tell how strong the
	// evidence behind an unverified tier is.
	unresolved := execplane.Evaluate(request, execplane.AccountResolution{
		Provider: execplane.ProviderClaude,
		Status:   execplane.AccountIndeterminate,
		Reason:   "no active account selected and 2 accounts registered",
	}, execplane.Entitlement{}, execplane.Entitlement{}, time.Now())

	assert.Empty(t, unresolved.ExecutionAccount)
	assert.NotEqual(t, receipt.ResolutionReason, unresolved.ResolutionReason)
}

// TestVerifiedRequiresDeterminedAccountAndEqualKnownGrades covers S8 / REQ-009
// as a property over the whole input space: verified is reachable only from a
// determined account whose known grade equals the known grade the catalog was
// probed under, and it never appears without a reason. This is what blocks the
// optimistic shortcuts — deriving a grade from a subscription type, treating a
// missing probe as harmless, or trusting a catalog whose grade is unknown.
func TestVerifiedRequiresDeterminedAccountAndEqualKnownGrades(t *testing.T) {
	t.Parallel()

	entitlements := []execplane.Entitlement{
		{},
		{Grade: "pro", Source: "host-cli@example.test"},
		{Grade: "plus", Source: "host-cli@example.test"},
	}
	resolutions := []execplane.AccountResolution{
		fixtureDeterminedAccount(execplane.ProviderCodex, "execution@example.test"),
		{
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountSoleRegistered,
			Account:  execplane.Account{ID: "acct-only", Email: "only@example.test"},
			Reason:   "no active account selected; adopted the sole registered account",
		},
		{
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountIndeterminate,
			Reason:   "no active account selected and 2 accounts registered",
		},
	}

	for _, resolution := range resolutions {
		for _, execution := range entitlements {
			for _, probe := range entitlements {
				receipt := execplane.Evaluate(
					fixtureTierRequest(execplane.ProviderCodex, "ultra", "gpt-5.6-sol"),
					resolution, execution, probe, time.Now(),
				)
				where := []any{"%s / exec %q / probe %q",
					resolution.Status, execution.Grade, probe.Grade}

				require.NotEmpty(t, receipt.ResolutionReason, where...)
				wantVerified := resolution.Determined() &&
					execution.Known() && probe.Known() && execution.Grade == probe.Grade
				if !wantVerified {
					assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus, where...)
					continue
				}
				assert.Equal(t, execplane.StatusVerified, receipt.VerificationStatus, where...)
				assert.NotEmpty(t, receipt.ExecutionAccount, where...)
				assert.True(t, receipt.Complete(), where...)
			}
		}
	}
}
