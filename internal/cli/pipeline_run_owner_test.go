package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/execplane"
)

func TestPipelineRunCmd_ExecutionOwnerAcceptsOnlyExactSingleValues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "default", want: pipelineExecutionOwnerOMP},
		{name: "explicit omp", values: []string{"omp"}, want: pipelineExecutionOwnerOMP},
		{name: "explicit orca", values: []string{"orca"}, want: pipelineExecutionOwnerOrca},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &pipelineRunConfig{}
			cmd := newPipelineRunCmdWithConfig(cfg)
			args := []string{"SPEC-OWNER-001", "--platform", "omp"}
			for _, value := range test.values {
				args = append(args, "--execution-owner", value)
			}

			require.NoError(t, cmd.ParseFlags(args))
			assert.Equal(t, test.want, cfg.ExecutionOwner)
			assert.Equal(t, len(test.values) == 1, cfg.executionOwnerExplicit)
		})
	}

	for _, value := range []string{"OMP", "Orca", "omp,orca", "omp ", "local", "supervised"} {
		t.Run("reject "+value, func(t *testing.T) {
			cmd := newPipelineRunCmdWithConfig(&pipelineRunConfig{})
			err := cmd.ParseFlags([]string{
				"SPEC-OWNER-001", "--platform", "omp", "--execution-owner", value,
			})
			assert.ErrorContains(t, err, "must be exactly omp or orca")
		})
	}

	cmd := newPipelineRunCmdWithConfig(&pipelineRunConfig{})
	err := cmd.ParseFlags([]string{
		"SPEC-OWNER-001", "--platform", "omp",
		"--execution-owner", "omp", "--execution-owner", "orca",
	})
	assert.ErrorContains(t, err, "specified exactly once")
}

func TestPipelineRunCmd_InvalidOrMixedOwnerStopsBeforeFilesystemEffects(t *testing.T) {
	for _, args := range [][]string{
		{"SPEC-OWNER-001", "--platform", "omp", "--execution-owner", "omp,orca"},
		{"SPEC-OWNER-001", "--platform", "omp", "--execution-owner", "omp", "--execution-owner", "orca"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			root := t.TempDir()
			chdirForTest(t, root)
			cmd := newPipelineRunCmd()
			cmd.SetArgs(args)

			require.Error(t, cmd.Execute())
			_, err := os.Stat(pipelineStateDir)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestResolvePipelineExecutionOwner_IsMutuallyExclusiveAndRequiresOMPBoundary(t *testing.T) {
	tests := []struct {
		name    string
		cfg     pipelineRunConfig
		owner   string
		source  string
		wantErr string
	}{
		{
			name: "default omp", cfg: pipelineRunConfig{Platform: "omp"},
			owner: pipelineExecutionOwnerOMP, source: pipelineExecutionOwnerSourceDefault,
		},
		{
			name:  "explicit omp",
			cfg:   pipelineRunConfig{Platform: "omp", ExecutionOwner: "omp", executionOwnerExplicit: true},
			owner: pipelineExecutionOwnerOMP, source: pipelineExecutionOwnerSourceExplicit,
		},
		{
			name:  "explicit orca",
			cfg:   pipelineRunConfig{Platform: "omp", ExecutionOwner: "orca", executionOwnerExplicit: true},
			owner: pipelineExecutionOwnerOrca, source: pipelineExecutionOwnerSourceExplicit,
		},
		{
			name:    "implicit orca rejected",
			cfg:     pipelineRunConfig{Platform: "omp", ExecutionOwner: "orca"},
			wantErr: "must be selected explicitly",
		},
		{
			name:    "non OMP boundary rejected",
			cfg:     pipelineRunConfig{Platform: "codex", ExecutionOwner: "omp", executionOwnerExplicit: true},
			wantErr: "requires exact --platform omp",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := resolvePipelineExecutionOwner(&test.cfg)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.owner, decision.Owner)
			assert.Equal(t, test.source, decision.Source)
			assert.NotEmpty(t, decision.Reason)
			assert.NotEqual(t, decision.Owner == pipelineExecutionOwnerOMP,
				decision.Owner == pipelineExecutionOwnerOrca,
				"exactly one execution owner must be selected")
		})
	}
}

func TestPipelineRun_OrcaOwnerEmitsHandoffBeforeOMPProcess(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	specID := "SPEC-OWNER-ORCA-001"
	writePipelineOwnerSpec(t, root, specID)
	marker := installPipelineOwnerProcessTrap(t, root, "omp")
	// verification_status is now derived from the tier integrity gate instead of
	// being hardcoded, so this test pins the gate's only outside-world seam.
	// Without the stub the expectation below would depend on whichever provider
	// accounts the workstation running the suite happens to have registered.
	stubExecplaneTierProbe(t, verifiedExecplaneTierEvidence)

	var stdout bytes.Buffer
	cmd := newPipelineRunCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		specID, "--platform", "omp", "--execution-owner", "orca",
	})

	err := cmd.Execute()

	require.Error(t, err)
	assert.ErrorIs(t, err, errPipelineExecutionOwnerHandoffRequired)
	assert.True(t, isJSONFatalError(err), "structured handoff must not add an unstructured stderr error")
	assert.NoFileExists(t, marker, "Orca ownership must fail closed before starting OMP")
	assert.NoFileExists(t, specCheckpointPath(specID), "handoff must not create an OMP checkpoint")

	var result pipelineExecutionOwnerResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result), "stdout=%q err=%v", stdout.String(), err)
	assert.Equal(t, pipelineExecutionOwnerResultSchema, result.Schema)
	assert.Equal(t, "handoff_required", result.Status)
	assert.Equal(t, pipelineExecutionOwnerOrca, result.Owner)
	assert.Equal(t, pipelineExecutionOwnerSourceExplicit, result.Source)
	assert.Equal(t, specID, result.SpecID)
	assert.NotEmpty(t, result.RunID)
	assert.Equal(t, "orca skills get orchestration --full", result.RequiredAction)

	receiptBytes, readErr := os.ReadFile(filepath.FromSlash(result.ReceiptPath))
	require.NoError(t, readErr)
	var receipt pipelineExecutionOwnerReceipt
	require.NoError(t, json.Unmarshal(receiptBytes, &receipt))
	assert.Equal(t, result.RunID, receipt.RunID)
	assert.Equal(t, execplane.StatusVerified, receipt.VerificationStatus,
		"a verified gate is what puts verified in the receipt")
	assertPipelineExecutionOwnerReceiptIsBodyFree(t, receiptBytes)
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(filepath.FromSlash(result.ReceiptPath))
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestPipelineRun_OMPOwnerPreservesDryRunForDefaultAndExplicitSelection(t *testing.T) {
	for _, test := range []struct {
		name   string
		flag   []string
		source string
	}{
		{name: "default", source: pipelineExecutionOwnerSourceDefault},
		{name: "explicit", flag: []string{"--execution-owner", "omp"}, source: pipelineExecutionOwnerSourceExplicit},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			chdirForTest(t, root)
			specID := "SPEC-OWNER-OMP-001"
			writePipelineOwnerSpec(t, root, specID)
			orcaMarker := installPipelineOwnerProcessTrap(t, root, "orca")
			var stdout bytes.Buffer
			cmd := newPipelineRunCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			args := []string{specID, "--platform", "omp", "--dry-run"}
			args = append(args, test.flag...)
			cmd.SetArgs(args)

			require.NoError(t, cmd.Execute())
			assert.Contains(t, stdout.String(), "Pipeline complete: 5 phases executed")
			assert.NoFileExists(t, orcaMarker, "OMP ownership must not create an Orca Run")
			var receipt pipelineExecutionOwnerReceipt
			body, err := os.ReadFile(filepath.Join(pipelineStateDir, specID+".execution-owner.json"))
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &receipt))
			assert.Equal(t, pipelineExecutionOwnerOMP, receipt.Owner)
			assert.Equal(t, test.source, receipt.Source)
			assert.Equal(t, specID, receipt.SpecID)
			assert.NotEmpty(t, receipt.RunID)
			assert.False(t, receipt.CheckedAt.IsZero())
			// REQ-008 scopes the tier integrity gate to the process-plane
			// handoff, so the OMP-owned path probes nothing — and a receipt
			// that claims "verified" without a check is the silent downgrade
			// SPEC-EXECPLANE-001 exists to prevent.
			assert.Equal(t, execplane.StatusUnverified, receipt.VerificationStatus)
			assert.NoFileExists(t, filepath.Join(pipelineStateDir, specID+".tier-integrity.json"),
				"no gate ran, so there is no integrity receipt to read")
			assertPipelineExecutionOwnerReceiptIsBodyFree(t, body)
		})
	}
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

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
