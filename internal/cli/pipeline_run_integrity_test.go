package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/insajin/autopus-adk/pkg/orcarun"
)

func TestPipelineTierIntegrityGate_VerifiedVerdictIsReportedAndReferencesTheReceipt(t *testing.T) {
	reprobes := noExecplaneCatalogReprobe(t)
	outcome := runPipelineOrcaGateWithProbe(t, verifiedExecplaneTierEvidence)

	// REQ-004's point: matching grades reuse the held catalog for free.
	assert.Zero(t, *reprobes)

	// The gate itself never stops a run; the absent process plane does.
	assert.ErrorIs(t, outcome.Err, orcarun.ErrOrcaUnavailable)
	assert.Equal(t, execplane.StatusVerified, outcome.VerificationStatus)
	assert.NotEmpty(t, outcome.VerificationReason, "a verdict without a reason cannot be acted on")
	assert.Equal(t,
		filepath.ToSlash(filepath.Join(pipelineStateDir, outcome.SpecID+".tier-integrity.json")),
		outcome.IntegrityReceiptPath)
	assert.Equal(t, execplane.StatusVerified,
		readPipelineExecutionOwnerReceipt(t, outcome.OwnerReceiptPath).VerificationStatus)

	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	assert.Equal(t, pipelineTierIntegrityReceiptSchema, receipt.Schema)
	assert.Equal(t, outcome.SpecID, receipt.SpecID)
	assert.Equal(t, execplane.StatusVerified, receipt.VerificationStatus)
	assert.False(t, receipt.CheckedAt.IsZero())
	require.Len(t, receipt.Providers, len(pipelineTierIntegrityProviders))
	// Both verify, but not on the same footing: only codex holds a served catalog.
	wantEvidence := map[string]execplane.EvidenceKind{
		execplane.ProviderCodex:  execplane.EvidenceProbedCatalog,
		execplane.ProviderClaude: execplane.EvidenceEntitlementParity,
	}
	for _, provider := range receipt.Providers {
		where := []any{"provider=%s", provider.Provider}
		assert.Equal(t, execplane.IntegrityReceiptSchema, provider.Schema, where...)
		assert.Equal(t, execplane.StatusVerified, provider.VerificationStatus, where...)
		assert.True(t, provider.Complete(), where...)
		assert.Equal(t, "exec@example.test", provider.ExecutionAccount, where...)
		assert.Equal(t, "probe@example.test", provider.CatalogSource.Account, where...)
		assert.Equal(t, "pro", provider.CatalogSource.Entitlement, where...)
		assert.Contains(t, outcome.VerificationReason, provider.Provider+": ", where...)
		assert.Equal(t, wantEvidence[provider.Provider], provider.EvidenceKind, where...)
	}
}

func TestPipelineTierIntegrityGate_UnverifiedSurfacesStatusAndReason(t *testing.T) {
	const reason = "two registered accounts and no active selection"
	noExecplaneCatalogReprobe(t)
	outcome := runPipelineOrcaGateWithProbe(t,
		func(_ context.Context, provider string) (execplane.Evidence, error) {
			return execplane.Evidence{Resolution: execplane.AccountResolution{
				Provider: provider, Status: execplane.AccountIndeterminate, Reason: reason,
			}}, nil
		})

	// An unverified verdict is recorded, not fatal: the run proceeds to the
	// execution boundary exactly as a verified one does, and stops there for the
	// unrelated reason that this workstation has no orca CLI.
	assert.ErrorIs(t, outcome.Err, orcarun.ErrOrcaUnavailable)
	assert.Equal(t, execplane.StatusUnverified, outcome.VerificationStatus)
	for _, provider := range pipelineTierIntegrityProviders {
		assert.Contains(t, outcome.VerificationReason, provider+": "+reason,
			"the output must name which provider degraded and why")
	}
	assert.Equal(t, execplane.StatusUnverified,
		readPipelineExecutionOwnerReceipt(t, outcome.OwnerReceiptPath).VerificationStatus,
		"the owner receipt must not claim a verification the gate refused")

	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
	require.Len(t, receipt.Providers, len(pipelineTierIntegrityProviders))
	for _, provider := range receipt.Providers {
		assert.Equal(t, execplane.StatusUnverified, provider.VerificationStatus, provider.Provider)
		assert.Equal(t, reason, provider.ResolutionReason, provider.Provider)
		assert.Empty(t, provider.ExecutionAccount,
			"an indeterminate account must stay unnamed instead of borrowing the probe account")
	}
}

func TestPipelineTierIntegrityGate_OrcaProbeFailureDegradesWithoutBlockingTheGate(t *testing.T) {
	const reason = "process plane account listing is unavailable"
	noExecplaneCatalogReprobe(t)
	outcome := runPipelineOrcaGateWithProbe(t,
		func(_ context.Context, provider string) (execplane.Evidence, error) {
			// The prober's documented failure shape: an indeterminate
			// resolution with a non-empty reason, returned with the error.
			return execplane.Evidence{Resolution: execplane.AccountResolution{
				Provider: provider, Status: execplane.AccountIndeterminate, Reason: reason,
			}}, execplane.ErrOrcaUnavailable
		})

	// A missing orca degrades the verdict without the gate itself blocking. What
	// stops this run is the executor: it needs the orca CLI to supervise a phase
	// and says so with its own sentinel. The gate's probe error, a distinct
	// sentinel, must not travel with it — the gate swallowed it into a reason.
	assert.ErrorIs(t, outcome.Err, orcarun.ErrOrcaUnavailable)
	assert.NotErrorIs(t, outcome.Err, execplane.ErrOrcaUnavailable)
	assert.Equal(t, execplane.StatusUnverified, outcome.VerificationStatus)
	assert.Contains(t, outcome.VerificationReason, reason)

	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	require.Len(t, receipt.Providers, len(pipelineTierIntegrityProviders))
	for _, provider := range receipt.Providers {
		assert.Equal(t, execplane.StatusUnverified, provider.VerificationStatus, provider.Provider)
		assert.NotEmpty(t, provider.ResolutionReason, provider.Provider)
		assert.True(t, provider.Complete(),
			"an unverified receipt is still a complete record of why")
	}
}

func TestTierRequestForProvider_NamesTierAndModelWithoutInferringOne(t *testing.T) {
	t.Parallel()

	quality := config.DefaultFullConfig("tier-request").Quality
	// Each owned provider names its evidence kind, so a bare "verified" can
	// never hide which of two very different verifications produced it.
	wantEvidence := map[string]execplane.EvidenceKind{
		execplane.ProviderCodex:  execplane.EvidenceProbedCatalog,
		execplane.ProviderClaude: execplane.EvidenceEntitlementParity,
	}
	for _, provider := range pipelineTierIntegrityProviders {
		request := tierRequestForProvider(quality, provider)
		assert.Equal(t, provider, request.Provider)
		assert.NotEmpty(t, request.RequestedTier, provider)
		assert.NotEmpty(t, request.ResolvedModel, provider)
		assert.Equal(t, wantEvidence[provider], request.Evidence, provider)
	}
	assert.Equal(t, config.QualityProviderCodex, execplane.ProviderCodex,
		"the gate and the quality plane must name providers identically")

	// An unowned provider is not guessed at: no model and no evidence.
	unowned := tierRequestForProvider(quality, "gemini")
	assert.Empty(t, unowned.ResolvedModel)
	assert.Equal(t, execplane.EvidenceNone, unowned.Evidence)

	// Even a spotless entitlement match cannot verify it: parity transfers
	// evidence across accounts, it never manufactures any.
	unownedReceipt := execplane.Evaluate(unowned,
		execplane.AccountResolution{Provider: "gemini", Status: execplane.AccountActive,
			Account: execplane.Account{Email: "exec@example.test"}},
		execplane.Entitlement{Grade: "pro", Source: "exec@example.test"},
		execplane.Entitlement{Grade: "pro", Source: "probe@example.test"},
		time.Now().UTC())
	assert.Equal(t, execplane.EvidenceNone, unownedReceipt.EvidenceKind)
	assert.Equal(t, execplane.StatusUnverified, unownedReceipt.VerificationStatus)
}

func TestFoldPipelineTierIntegrity_DemandsEveryProviderAndAlwaysGivesAReason(t *testing.T) {
	t.Parallel()

	verified := execplane.IntegrityReceipt{
		Schema: execplane.IntegrityReceiptSchema, Provider: execplane.ProviderCodex,
		RequestedTier: "ultra", ResolvedModel: "gpt-5.6-sol/max",
		ExecutionAccount: "exec@example.test",
		CatalogSource: execplane.CatalogSource{
			Account: "probe@example.test", Entitlement: "pro",
		},
		ResolutionReason:   "execution and probe accounts share entitlement pro",
		VerificationStatus: execplane.StatusVerified,
		// Expectation change: Complete() now demands an evidence kind, so a
		// fixture that omits it is no longer a verified receipt.
		EvidenceKind: execplane.EvidenceProbedCatalog,
	}
	status, reason := foldPipelineTierIntegrity([]execplane.IntegrityReceipt{verified})
	assert.Equal(t, execplane.StatusVerified, status)
	assert.Equal(t, "codex: "+verified.ResolutionReason, reason)

	unverified := verified
	unverified.Provider = execplane.ProviderClaude
	unverified.VerificationStatus = execplane.StatusUnverified
	unverified.ResolutionReason = "neither execution nor probe entitlement is known"
	status, reason = foldPipelineTierIntegrity([]execplane.IntegrityReceipt{verified, unverified})
	assert.Equal(t, execplane.StatusUnverified, status, "one unverified provider degrades the run")
	assert.Contains(t, reason, "codex: ")
	assert.Contains(t, reason, "claude: "+unverified.ResolutionReason)

	incomplete := verified
	incomplete.ResolvedModel = ""
	status, reason = foldPipelineTierIntegrity([]execplane.IntegrityReceipt{incomplete})
	assert.Equal(t, execplane.StatusUnverified, status, "an incomplete receipt is not evidence")
	assert.Contains(t, reason, "incomplete")

	status, reason = foldPipelineTierIntegrity(nil)
	assert.Equal(t, execplane.StatusUnverified, status)
	assert.NotEmpty(t, reason)

	// A gate that never ran is unverified too, with its own reason.
	skipped := pipelineTierIntegritySkipped()
	assert.Equal(t, execplane.StatusUnverified, skipped.Status)
	assert.NotEmpty(t, skipped.Reason)
	assert.Empty(t, skipped.ReceiptPath)
}
