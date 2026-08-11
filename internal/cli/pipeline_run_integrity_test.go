package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/execplane"
)

// execplaneTierProbeAccountID is a managed account id the fixtures carry so the
// receipts can be checked for leaking it. Only the entitlement grade and the
// recognizable email may leave the process plane.
const execplaneTierProbeAccountID = "acct-3f9c1d20-managed"

// stubExecplaneTierProbe pins the gate's only outside-world seam, so the wiring
// is exercised without an orca binary or this workstation's account roster.
func stubExecplaneTierProbe(t *testing.T, probe execplaneTierProbeFunc) {
	t.Helper()
	original := runtimeExecplaneTierProbe
	t.Cleanup(func() { runtimeExecplaneTierProbe = original })
	runtimeExecplaneTierProbe = probe
}

// verifiedExecplaneTierEvidence resolves both providers to an execution account
// that differs from the probe account but shares its entitlement grade — the
// S3 shape in which the held catalog stands as evidence with no re-probe.
//
// It answers for claude too, which production never does (claude exposes no
// per-account grade source). That is deliberate: this fixture exercises the
// handoff wiring for a verified verdict, not a claim about claude.
func verifiedExecplaneTierEvidence(_ context.Context, provider string) (execplane.Evidence, error) {
	return execplane.Evidence{
		Resolution: execplane.AccountResolution{
			Provider: provider, Status: execplane.AccountActive,
			Account: execplane.Account{ID: execplaneTierProbeAccountID, Email: "exec@example.test"},
			Probe:   execplane.Account{ID: execplaneTierProbeAccountID, Email: "probe@example.test"},
		},
		ExecEntitlement:  execplane.Entitlement{Grade: "pro", Source: "exec@example.test"},
		ProbeEntitlement: execplane.Entitlement{Grade: "pro", Source: "probe@example.test"},
	}, nil
}

// runPipelineOrcaHandoffWithProbe drives the orca handoff under one probe stub
// and asserts REQ-007 on every path: the gate completes before anything can be
// created, so no OMP process, no Orca Run, and no checkpoint exist afterwards.
func runPipelineOrcaHandoffWithProbe(
	t *testing.T,
	probe execplaneTierProbeFunc,
) (string, pipelineExecutionOwnerResult, error) {
	t.Helper()
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-INTEGRITY-GATE-001"
	writePipelineOwnerSpec(t, root, specID)
	ompMarker := installPipelineOwnerProcessTrap(t, root, "omp")
	orcaMarker := installPipelineOwnerProcessTrap(t, root, "orca")
	stubExecplaneTierProbe(t, probe)

	var stdout bytes.Buffer
	cmd := newPipelineRunCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specID, "--platform", "omp", "--execution-owner", "orca"})

	err := cmd.Execute()

	assert.NoFileExists(t, ompMarker, "the gate must finish before any OMP process starts")
	assert.NoFileExists(t, orcaMarker, "the gate must not create an Orca Run, worker, or session")
	assert.NoFileExists(t, specCheckpointPath(specID), "the gate must not write a checkpoint")

	var result pipelineExecutionOwnerResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result), "stdout=%q err=%v", stdout.String(), err)
	return specID, result, err
}

func readPipelineTierIntegrityReceipt(t *testing.T, path string) pipelineTierIntegrityReceipt {
	t.Helper()
	require.NotEmpty(t, path, "the handoff must reference a persisted integrity receipt")
	body, err := os.ReadFile(filepath.FromSlash(path))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(filepath.FromSlash(path))
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	assert.NotContains(t, string(body), execplaneTierProbeAccountID,
		"a managed account id must never reach a receipt")
	var receipt pipelineTierIntegrityReceipt
	require.NoError(t, json.Unmarshal(body, &receipt))
	return receipt
}

func readPipelineExecutionOwnerReceipt(t *testing.T, path string) pipelineExecutionOwnerReceipt {
	t.Helper()
	body, err := os.ReadFile(filepath.FromSlash(path))
	require.NoError(t, err)
	var receipt pipelineExecutionOwnerReceipt
	require.NoError(t, json.Unmarshal(body, &receipt))
	return receipt
}

func TestPipelineTierIntegrityGate_VerifiedHandoffReferencesTheReceipt(t *testing.T) {
	specID, result, err := runPipelineOrcaHandoffWithProbe(t, verifiedExecplaneTierEvidence)

	assert.ErrorIs(t, err, errPipelineExecutionOwnerHandoffRequired)
	assert.Equal(t, "handoff_required", result.Status)
	assert.Equal(t, execplane.StatusVerified, result.VerificationStatus)
	assert.NotEmpty(t, result.VerificationReason, "a verdict without a reason cannot be acted on")
	assert.Equal(t,
		filepath.ToSlash(filepath.Join(pipelineStateDir, specID+".tier-integrity.json")),
		result.IntegrityReceiptPath)
	assert.Equal(t, execplane.StatusVerified,
		readPipelineExecutionOwnerReceipt(t, result.ReceiptPath).VerificationStatus)

	receipt := readPipelineTierIntegrityReceipt(t, result.IntegrityReceiptPath)
	assert.Equal(t, pipelineTierIntegrityReceiptSchema, receipt.Schema)
	assert.Equal(t, specID, receipt.SpecID)
	assert.Equal(t, execplane.StatusVerified, receipt.VerificationStatus)
	assert.Equal(t, result.VerificationReason, receipt.Reason)
	assert.False(t, receipt.CheckedAt.IsZero())
	require.Len(t, receipt.Providers, len(pipelineTierIntegrityProviders))
	for _, provider := range receipt.Providers {
		where := []any{"provider=%s", provider.Provider}
		assert.Equal(t, execplane.IntegrityReceiptSchema, provider.Schema, where...)
		assert.Equal(t, execplane.StatusVerified, provider.VerificationStatus, where...)
		assert.True(t, provider.Complete(), where...)
		assert.Equal(t, "exec@example.test", provider.ExecutionAccount, where...)
		assert.Equal(t, "probe@example.test", provider.CatalogSource.Account, where...)
		assert.Equal(t, "pro", provider.CatalogSource.Entitlement, where...)
		assert.Contains(t, result.VerificationReason, provider.Provider+": ", where...)
	}
}

func TestPipelineTierIntegrityGate_UnverifiedSurfacesStatusAndReason(t *testing.T) {
	const reason = "two registered accounts and no active selection"
	_, result, err := runPipelineOrcaHandoffWithProbe(t,
		func(_ context.Context, provider string) (execplane.Evidence, error) {
			return execplane.Evidence{Resolution: execplane.AccountResolution{
				Provider: provider, Status: execplane.AccountIndeterminate, Reason: reason,
			}}, nil
		})

	// An unverified verdict is recorded, not fatal: the handoff still happens
	// and the run still stops at the same handoff boundary as before the gate.
	assert.ErrorIs(t, err, errPipelineExecutionOwnerHandoffRequired)
	assert.Equal(t, "handoff_required", result.Status)
	assert.Equal(t, execplane.StatusUnverified, result.VerificationStatus)
	for _, provider := range pipelineTierIntegrityProviders {
		assert.Contains(t, result.VerificationReason, provider+": "+reason,
			"the output must name which provider degraded and why")
	}
	assert.Equal(t, execplane.StatusUnverified,
		readPipelineExecutionOwnerReceipt(t, result.ReceiptPath).VerificationStatus,
		"the owner receipt must not claim a verification the gate refused")

	receipt := readPipelineTierIntegrityReceipt(t, result.IntegrityReceiptPath)
	assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
	require.Len(t, receipt.Providers, len(pipelineTierIntegrityProviders))
	for _, provider := range receipt.Providers {
		assert.Equal(t, execplane.StatusUnverified, provider.VerificationStatus, provider.Provider)
		assert.Equal(t, reason, provider.ResolutionReason, provider.Provider)
		assert.Empty(t, provider.ExecutionAccount,
			"an indeterminate account must stay unnamed instead of borrowing the probe account")
	}
}

func TestPipelineTierIntegrityGate_OrcaProbeFailureStillHandsOffUnverified(t *testing.T) {
	const reason = "process plane account listing is unavailable"
	_, result, err := runPipelineOrcaHandoffWithProbe(t,
		func(_ context.Context, provider string) (execplane.Evidence, error) {
			// The prober's documented failure shape: an indeterminate
			// resolution with a non-empty reason, returned with the error.
			return execplane.Evidence{Resolution: execplane.AccountResolution{
				Provider: provider, Status: execplane.AccountIndeterminate, Reason: reason,
			}}, execplane.ErrOrcaUnavailable
		})

	assert.ErrorIs(t, err, errPipelineExecutionOwnerHandoffRequired,
		"a missing orca must degrade the verdict, not block the handoff")
	assert.NotErrorIs(t, err, execplane.ErrOrcaUnavailable)
	assert.Equal(t, "handoff_required", result.Status)
	assert.Equal(t, execplane.StatusUnverified, result.VerificationStatus)
	assert.Contains(t, result.VerificationReason, reason)

	receipt := readPipelineTierIntegrityReceipt(t, result.IntegrityReceiptPath)
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
	for _, provider := range pipelineTierIntegrityProviders {
		request := tierRequestForProvider(quality, provider)
		assert.Equal(t, provider, request.Provider)
		assert.NotEmpty(t, request.RequestedTier, provider)
		assert.NotEmpty(t, request.ResolvedModel, provider)
	}
	assert.Equal(t, config.QualityProviderCodex, execplane.ProviderCodex,
		"the gate and the quality plane must name providers identically")

	// An unowned provider is not guessed at: the empty model fails the
	// receipt's completeness check rather than inventing a tier contract.
	assert.Empty(t, tierRequestForProvider(quality, "gemini").ResolvedModel)
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
