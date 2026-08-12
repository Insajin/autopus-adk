package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/insajin/autopus-adk/pkg/orcarun"
)

// execplaneReprobeAccountID is the managed account the mismatch fixture resolves
// to. It is UUID-shaped because that is the only shape execplane will join into
// a managed home path, and the receipts are checked for leaking it.
const execplaneReprobeAccountID = "3f9c1d20-4a7b-4c31-9f2e-8b6d5a1c7e40"

// stubExecplaneCatalogReprobe pins the re-probe seam and returns its call
// counter. The counter is the point: REQ-004's cheap path is defined by the
// subprocesses it does not spend, and only a counter can hold that.
func stubExecplaneCatalogReprobe(t *testing.T, reprobe execplaneCatalogReprobeFunc) *int {
	t.Helper()
	original := runtimeExecplaneCatalogReprobe
	t.Cleanup(func() { runtimeExecplaneCatalogReprobe = original })
	calls := 0
	runtimeExecplaneCatalogReprobe = func(ctx context.Context, binary, home string) ([]byte, error) {
		calls++
		return reprobe(ctx, binary, home)
	}
	return &calls
}

// noExecplaneCatalogReprobe pins the seam shut. Matching or unrecoverable grades
// must cost zero subprocesses, so a re-probe is a gate bug and fails here rather
// than reaching whatever codex happens to be on PATH.
func noExecplaneCatalogReprobe(t *testing.T) *int {
	t.Helper()
	return stubExecplaneCatalogReprobe(t, func(context.Context, string, string) ([]byte, error) {
		t.Error("the gate re-probed a provider whose entitlements did not differ")
		return nil, errors.New("re-probe is not permitted here")
	})
}

// mismatchedCodexTierEvidence differs from the verified fixture in one place:
// codex's held catalog was probed under a weaker grade than the account that
// will run the workload. Claude's grades match, so exactly one provider can
// spend a subprocess and the counter distinguishes the two paths.
func mismatchedCodexTierEvidence(_ context.Context, provider string) (execplane.Evidence, error) {
	evidence := execplane.Evidence{
		Resolution: execplane.AccountResolution{
			Provider: provider, Status: execplane.AccountActive,
			Account: execplane.Account{ID: execplaneReprobeAccountID, Email: "exec@example.test"},
			Probe:   execplane.Account{Email: "probe@example.test"},
		},
		ExecEntitlement:  execplane.Entitlement{Grade: "pro", Source: "exec@example.test"},
		ProbeEntitlement: execplane.Entitlement{Grade: "pro", Source: "probe@example.test"},
	}
	if provider == execplane.ProviderCodex {
		evidence.ProbeEntitlement.Grade = "plus"
	}
	return evidence, nil
}

// providerTierIntegrityReceipt picks one provider's receipt out of the folded
// record, failing rather than silently asserting nothing when it is absent.
func providerTierIntegrityReceipt(
	t *testing.T, receipt pipelineTierIntegrityReceipt, provider string,
) execplane.IntegrityReceipt {
	t.Helper()
	for _, candidate := range receipt.Providers {
		if candidate.Provider == provider {
			return candidate
		}
	}
	require.FailNow(t, "the gate recorded no receipt for provider "+provider)
	return execplane.IntegrityReceipt{}
}

func TestPipelineTierIntegrityGate_MismatchedEntitlementReprobesUnderTheExecutionAccount(t *testing.T) {
	var homes []string
	calls := stubExecplaneCatalogReprobe(t,
		func(_ context.Context, binary, home string) ([]byte, error) {
			assert.Equal(t, "codex", binary,
				"the re-probe must read the catalog through the CLI this run would launch")
			homes = append(homes, home)
			return []byte(`{"models":[{"slug":"gpt-5.6-sol"}]}`), nil
		})

	outcome := runPipelineOrcaGateWithProbe(t, mismatchedCodexTierEvidence)

	assert.ErrorIs(t, outcome.Err, orcarun.ErrOrcaUnavailable)
	assert.Equal(t, 1, *calls, "only the mismatched provider may spend a subprocess")
	require.Len(t, homes, 1)
	assert.Contains(t, homes[0], execplaneReprobeAccountID,
		"the catalog must be re-read under the execution account's own home")
	assert.Equal(t, execplane.StatusVerified, outcome.VerificationStatus,
		"a successful re-probe restores the evidence the mismatch destroyed")

	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	codex := providerTierIntegrityReceipt(t, receipt, execplane.ProviderCodex)
	assert.Equal(t, execplane.StatusVerified, codex.VerificationStatus)
	assert.Equal(t, "exec@example.test", codex.CatalogSource.Account,
		"a re-probed catalog is the execution account's own catalog")
	assert.Equal(t, "pro", codex.CatalogSource.Entitlement)
	assert.Equal(t, execplane.EvidenceProbedCatalog, codex.EvidenceKind)
	assert.Contains(t, codex.ResolutionReason, "re-probed under the execution account",
		"the receipt must say the catalog was replaced, not that it always matched")
	assert.True(t, codex.Complete())

	claude := providerTierIntegrityReceipt(t, receipt, execplane.ProviderClaude)
	assert.Equal(t, execplane.EvidenceEntitlementParity, claude.EvidenceKind,
		"claude has no account-scoped catalog probe, so parity is all it can claim")
	assert.NotContains(t, claude.ResolutionReason, "re-probe",
		"matching grades must not be described as a re-probe")
}

func TestPipelineTierIntegrityGate_FailedReprobeIsUnverifiedAndNamesTheFailure(t *testing.T) {
	const probeFailure = "codex debug models under /Users/somebody/managed-home: exit status 1"
	calls := stubExecplaneCatalogReprobe(t,
		func(context.Context, string, string) ([]byte, error) {
			return nil, errors.New(probeFailure)
		})

	outcome := runPipelineOrcaGateWithProbe(t, mismatchedCodexTierEvidence)

	// REQ-004's last branch: neither the held catalog nor a fresh one is
	// evidence, so the verdict degrades to REQ-009 instead of stopping the run.
	// What stops this run is the missing orca CLI the executor needs.
	assert.ErrorIs(t, outcome.Err, orcarun.ErrOrcaUnavailable)
	assert.Equal(t, 1, *calls)
	assert.Equal(t, execplane.StatusUnverified, outcome.VerificationStatus)
	assert.Contains(t, outcome.VerificationReason,
		"catalog re-probe under the execution account failed")
	assert.NotContains(t, outcome.VerificationReason, probeFailure,
		"the probe error can name a managed home, so only the failed step is reported")

	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	codex := providerTierIntegrityReceipt(t, receipt, execplane.ProviderCodex)
	assert.Equal(t, execplane.StatusUnverified, codex.VerificationStatus)
	assert.Contains(t, codex.ResolutionReason, "entitlement differs",
		"the reason must still name the mismatch that forced the re-probe")
	assert.Contains(t, codex.ResolutionReason,
		"catalog re-probe under the execution account failed")
	assert.Equal(t, "probe@example.test", codex.CatalogSource.Account,
		"a failed re-probe must not relabel the mismatched catalog as the executor's")
	assert.True(t, codex.Complete(),
		"an unverified receipt is still a complete record of why")
}
