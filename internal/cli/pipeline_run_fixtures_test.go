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

	"github.com/insajin/autopus-adk/pkg/execplane"
	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// execplaneTierProbeAccountID is a managed account id the fixtures carry so the
// receipts can be checked for leaking it. Only the entitlement grade and the
// recognizable email may leave the process plane.
const execplaneTierProbeAccountID = "acct-3f9c1d20-managed"

func writePipelineOwnerSpec(t *testing.T, root, specID string) {
	t.Helper()
	dir := filepath.Join(root, ".autopus", "specs", specID)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	for name, body := range map[string]string{
		"spec.md":       "# " + specID + ": execution owner contract\n",
		"plan.md":       "# Plan\nPreserve one execution owner.\n",
		"acceptance.md": "# Acceptance\nExactly one owner is active.\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
}

func installPipelineOwnerProcessTrap(t *testing.T, root, name string) string {
	t.Helper()
	marker := filepath.Join(root, name+"-started")
	if runtime.GOOS == "windows" {
		return marker
	}
	binDir := filepath.Join(root, "bin-"+name)
	require.NoError(t, os.MkdirAll(binDir, 0o700))
	script := "#!/bin/sh\nprintf started > \"" + marker + "\"\nexit 97\n"
	require.NoError(t, os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

// isolatePipelineOwnerPath narrows PATH to a single directory holding one trap
// script per named binary. Prepending, which installPipelineOwnerProcessTrap
// does, can only prove what a run must not launch; narrowing also proves what it
// must not find, and the orca path needs that — a workstation running this suite
// very likely has a real orca on PATH, and a real orca must never be touched.
func isolatePipelineOwnerPath(t *testing.T, root string, names ...string) map[string]string {
	t.Helper()
	binDir := filepath.Join(root, "bin-isolated")
	require.NoError(t, os.MkdirAll(binDir, 0o700))
	markers := make(map[string]string, len(names))
	for _, name := range names {
		marker := filepath.Join(root, name+"-started")
		markers[name] = marker
		if runtime.GOOS == "windows" {
			continue
		}
		script := "#!/bin/sh\nprintf started > \"" + marker + "\"\nexit 97\n"
		require.NoError(t, os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o700))
	}
	t.Setenv("PATH", binDir)
	return markers
}

// assertPipelineRunBlockedBeforeAnyPhase reads the blocked receipt a preflight
// failure leaves behind and proves nothing ran: every canonical phase is still
// pending, so no worker, Dispatch, or provider session can have existed. It
// returns the receipt so a caller can also pin the blocker it names.
func assertPipelineRunBlockedBeforeAnyPhase(t *testing.T, specID string) pipeline.OrchestrationRunReceipt {
	t.Helper()
	cp, err := pipeline.LoadFile(specCheckpointPath(specID))
	require.NoError(t, err)
	require.NotNil(t, cp, "a blocked run must leave the receipt that says why")
	require.NotNil(t, cp.Receipt)
	assert.Equal(t, specID, cp.SpecID)
	assert.Equal(t, pipeline.TerminalBlocked, cp.Receipt.Terminal)
	require.NotEmpty(t, cp.TaskStatus)
	for phase, status := range cp.TaskStatus {
		assert.Equal(t, pipeline.CheckpointStatusPending, status, phase)
	}
	return *cp.Receipt
}

func assertPipelineExecutionOwnerReceiptIsBodyFree(t *testing.T, body []byte) {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	assert.ElementsMatch(t, []string{
		"schema", "owner", "source", "reason", "spec_id", "run_id", "checked_at", "verification_status",
	}, mapKeys(fields))
	for _, forbidden := range []string{"body", "prompt", "output", "task_body", "spec_body"} {
		assert.NotContains(t, fields, forbidden)
	}
}

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
// Both providers answer with a grade, which the prober now recovers for claude
// as well; what differs between them is not the grade but what the grade
// licenses, and the evidence kind on each receipt is where that shows.
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

// pipelineOrcaGateOutcome is what an orca-owned run leaves a reader: the gate's
// verdict, recovered from the persisted integrity receipt rather than from a
// handoff payload the run no longer emits, plus the stream the verdict was
// reported on and the error that stopped the run.
type pipelineOrcaGateOutcome struct {
	SpecID               string
	VerificationStatus   string
	VerificationReason   string
	IntegrityReceiptPath string
	OwnerReceiptPath     string
	Stderr               string
	Err                  error
}

// runPipelineOrcaGateWithProbe drives an orca-owned run under one probe stub and
// asserts what every gate path shares: the gate completes and records its verdict
// while nothing exists yet, and the run then stops at the absent orca CLI instead
// of diverting to the OMP backend the operator declined.
//
// PATH holds an omp trap and no orca, which is what keeps this test off a real
// orca: the gate's verdict is decided entirely by the injected probe, and the
// only thing standing between the verdict and a supervised worker is a process
// plane the fixture removed.
func runPipelineOrcaGateWithProbe(t *testing.T, probe execplaneTierProbeFunc) pipelineOrcaGateOutcome {
	t.Helper()
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-INTEGRITY-GATE-001"
	writePipelineOwnerSpec(t, root, specID)
	markers := isolatePipelineOwnerPath(t, root, "omp")
	stubExecplaneTierProbe(t, probe)

	var stdout, stderr bytes.Buffer
	cmd := newPipelineRunCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{specID, "--platform", "omp", "--execution-owner", "orca"})

	err := cmd.Execute()

	assert.NoFileExists(t, markers["omp"], "an orca-owned run must never divert to OMP")
	assert.NotContains(t, stdout.String(), "handoff_required",
		"REQ-108: handoff_required is no longer a terminal state")
	// REQ-106/INV-105: the gate ran, yet every phase is still pending — so no
	// worker could have started under whatever verdict the gate reached.
	assertPipelineRunBlockedBeforeAnyPhase(t, specID)

	outcome := pipelineOrcaGateOutcome{
		SpecID: specID, Stderr: stderr.String(), Err: err,
		IntegrityReceiptPath: filepath.ToSlash(
			filepath.Join(pipelineStateDir, specID+".tier-integrity.json")),
		OwnerReceiptPath: filepath.ToSlash(
			filepath.Join(pipelineStateDir, specID+".execution-owner.json")),
	}
	receipt := readPipelineTierIntegrityReceipt(t, outcome.IntegrityReceiptPath)
	outcome.VerificationStatus = receipt.VerificationStatus
	outcome.VerificationReason = receipt.Reason
	// A verdict nobody sees is the silent downgrade this gate exists to prevent,
	// and the handoff payload that used to carry it is gone.
	assert.Contains(t, outcome.Stderr, "Tier integrity "+outcome.VerificationStatus)
	assert.Contains(t, outcome.Stderr, outcome.VerificationReason)
	assert.Contains(t, outcome.Stderr, outcome.IntegrityReceiptPath,
		"the verdict must point at the record that reconstructs it")
	return outcome
}

func readPipelineTierIntegrityReceipt(t *testing.T, path string) pipelineTierIntegrityReceipt {
	t.Helper()
	require.NotEmpty(t, path, "the verdict must reference a persisted integrity receipt")
	body, err := os.ReadFile(filepath.FromSlash(path))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(filepath.FromSlash(path))
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	for _, accountID := range []string{execplaneTierProbeAccountID, execplaneReprobeAccountID} {
		assert.NotContains(t, string(body), accountID,
			"a managed account id must never reach a receipt")
	}
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

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
