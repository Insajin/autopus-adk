package execplane_test

import (
	"errors"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompareEntitlementTrustsEqualGradesAcrossDifferentAccounts covers S3 /
// REQ-004, and it is the point of the whole comparison: the catalog is evidence
// when the grades match, no matter whose account produced it. acceptance.md S3
// calls an oracle that fails on account difference alone unfit, because the
// measured pair (two different accounts in two different orgs, both on the same
// `chatgpt_plan_type`) returned identical decision fields. So this test asserts
// the opposite of "different account means unverified", and holds the two
// account identifiers distinct so it keeps proving that.
func TestCompareEntitlementTrustsEqualGradesAcrossDifferentAccounts(t *testing.T) {
	t.Parallel()

	execution := execplane.Entitlement{Grade: "pro", Source: "execution@example.test"}
	probe := execplane.Entitlement{Grade: "pro", Source: "host-cli@example.test"}

	verdict, reason := execplane.CompareEntitlement(execution, probe)
	require.Equal(t, execplane.VerdictTrusted, verdict)
	assert.True(t, mentionsGrade(reason, "pro"), "reason must name the shared grade: %q", reason)

	receipt := execplane.Evaluate(
		execplane.TierRequest{
			Provider:      execplane.ProviderCodex,
			RequestedTier: "ultra",
			ResolvedModel: "gpt-5.6-sol",
			Evidence:      execplane.EvidenceProbedCatalog,
		},
		execplane.AccountResolution{
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountActive,
			Account:  execplane.Account{ID: "acct-exec", Email: "execution@example.test", Org: "org-exec"},
			Probe:    execplane.Account{Email: "host-cli@example.test", Org: "org-host-cli"},
		},
		execution, probe, time.Now(),
	)

	assert.Equal(t, execplane.StatusVerified, receipt.VerificationStatus)
	assert.NotEqual(t, receipt.ExecutionAccount, receipt.CatalogSource.Account,
		"the fixture must keep execution and probe accounts distinct or it proves nothing")
	assert.True(t, receipt.Complete())
}

// TestCompareEntitlementDoesNotPrivilegeParticularGrades covers S3 / REQ-004
// and the premise S10 fixes: the comparison is string equality, not a
// plan-to-capability table. Any grade equal to itself is trusted, and no grade
// is trusted against a different one — an implementation that ranks grades, or
// that treats one grade as a permissive default, fails this sweep.
func TestCompareEntitlementDoesNotPrivilegeParticularGrades(t *testing.T) {
	t.Parallel()

	grades := []string{"pro", "plus", "free", "enterprise", "team"}
	for _, left := range grades {
		for _, right := range grades {
			verdict, reason := execplane.CompareEntitlement(
				execplane.Entitlement{Grade: left, Source: "execution@example.test"},
				execplane.Entitlement{Grade: right, Source: "host-cli@example.test"},
			)

			want := execplane.VerdictReprobe
			if left == right {
				want = execplane.VerdictTrusted
			}
			assert.Equal(t, want, verdict, "exec %q vs probe %q", left, right)
			assert.NotEmpty(t, reason, "exec %q vs probe %q", left, right)
		}
	}
}

// TestCompareEntitlementDemandsReprobeWhenGradesDiffer covers S3 / REQ-004:
// differing grades void the held catalog, the receipt drops to unverified even
// though the execution account is fully determined, and the reason names both
// grades so a reader can tell which side moved.
func TestCompareEntitlementDemandsReprobeWhenGradesDiffer(t *testing.T) {
	t.Parallel()

	execution := execplane.Entitlement{Grade: "pro", Source: "execution@example.test"}
	probe := execplane.Entitlement{Grade: "plus", Source: "host-cli@example.test"}

	verdict, reason := execplane.CompareEntitlement(execution, probe)
	require.Equal(t, execplane.VerdictReprobe, verdict)
	assert.True(t, mentionsGrade(reason, "pro"), "reason must name the execution grade: %q", reason)
	assert.True(t, mentionsGrade(reason, "plus"), "reason must name the probe grade: %q", reason)

	// The catalog is held, so only the grade mismatch can sink this receipt and
	// the assertions below cannot pass for the wrong reason.
	receipt := execplane.Evaluate(
		execplane.TierRequest{
			Provider:      execplane.ProviderCodex,
			RequestedTier: "ultra",
			ResolvedModel: "gpt-5.6-sol",
			Evidence:      execplane.EvidenceProbedCatalog,
		},
		execplane.AccountResolution{
			Provider: execplane.ProviderCodex,
			Status:   execplane.AccountActive,
			Account:  execplane.Account{ID: "acct-exec", Email: "execution@example.test"},
		},
		execution, probe, time.Now(),
	)

	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus,
		"a catalog probed under a different grade must not pass without a re-probe")
	assert.Equal(t, "plus", receipt.CatalogSource.Entitlement,
		"the receipt records the grade the held catalog was actually probed under")
}

// TestCompareEntitlementRefusesToCompareUnknownGrades covers S3 / REQ-004 and
// S8 / REQ-009: a grade that could not be recovered degrades to unverified
// instead of to a permissive default, and each missing side reads differently
// so the receipt says which one is missing.
func TestCompareEntitlementRefusesToCompareUnknownGrades(t *testing.T) {
	t.Parallel()

	known := execplane.Entitlement{Grade: "pro", Source: "known@example.test"}
	cases := map[string]struct{ execution, probe execplane.Entitlement }{
		"execution grade unknown": {execution: execplane.Entitlement{}, probe: known},
		"probe grade unknown":     {execution: known, probe: execplane.Entitlement{}},
		"neither grade known":     {execution: execplane.Entitlement{}, probe: execplane.Entitlement{}},
	}

	reasons := make(map[string]string, len(cases))
	for name, tc := range cases {
		verdict, reason := execplane.CompareEntitlement(tc.execution, tc.probe)

		assert.Equal(t, execplane.VerdictUnverified, verdict, name)
		assert.NotEmpty(t, reason, name)
		reasons[name] = reason
	}

	assert.NotEqual(t, reasons["execution grade unknown"], reasons["probe grade unknown"],
		"which side is missing decides whether a re-probe could help, so the reasons must differ")
	assert.NotEqual(t, reasons["neither grade known"], reasons["probe grade unknown"])
}

// TestParseCodexEntitlementReadsTheGradeClaim covers S3 / REQ-004: the grade
// comes out of the id_token claim with no network call, and only the grade and
// its source label leave the parser — the account identifier that shares the
// claim stays inside.
func TestParseCodexEntitlementReadsTheGradeClaim(t *testing.T) {
	t.Parallel()

	entitlement, err := execplane.ParseCodexEntitlement(
		fixtureGradedAuth(t, "pro"), "host-cli@example.test")
	require.NoError(t, err)

	assert.Equal(t, "pro", entitlement.Grade)
	assert.Equal(t, "host-cli@example.test", entitlement.Source)
	assert.True(t, entitlement.Known())
	assert.NotContains(t, entitlement.Grade+entitlement.Source, fixtureAccountIDClaim,
		"nothing from the credential but the grade may reach the receipt")
}

// TestParseCodexEntitlementRejectsUnusableCredentials covers S8 / REQ-009: a
// credential that yields no grade is an error and an unknown entitlement, never
// an assumed one. Assuming a grade here would fabricate the equality that
// REQ-004 checks.
func TestParseCodexEntitlementRejectsUnusableCredentials(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		auth         []byte
		wantSentinel bool
	}{
		"credential carries no grade claim": {
			auth:         fixtureCodexAuth(t, map[string]any{"sub": "fixture-subject"}),
			wantSentinel: true,
		},
		"grade claim is empty": {
			auth: fixtureCodexAuth(t, map[string]any{
				openAIAuthClaimKey: map[string]any{"chatgpt_plan_type": ""},
			}),
			wantSentinel: true,
		},
		"grade claim is not a string": {
			auth: fixtureCodexAuth(t, map[string]any{
				openAIAuthClaimKey: map[string]any{"chatgpt_plan_type": 3},
			}),
			wantSentinel: true,
		},
		"credential carries no token": {
			auth:         []byte(`{"tokens":{}}`),
			wantSentinel: true,
		},
		"token is not a jwt": {
			auth: []byte(`{"id_token":"header.payload"}`),
		},
		"credential is not json": {
			auth: []byte(`{"tokens":`),
		},
	}

	for name, tc := range cases {
		entitlement, err := execplane.ParseCodexEntitlement(tc.auth, "host-cli@example.test")

		require.Error(t, err, name)
		if tc.wantSentinel {
			assert.True(t, errors.Is(err, execplane.ErrNoEntitlementClaim), name)
		}
		assert.False(t, entitlement.Known(), "%s: a failed parse must not report a grade", name)
	}
}
