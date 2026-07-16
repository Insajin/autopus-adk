package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPlaywright_SnapshotProofPolicyIssues_DoNotBecomeDefaultExecutionErrors(t *testing.T) {
	tests := []struct {
		name          string
		proofMode     string
		expectedState string
	}{
		{name: "missing proof", proofMode: "missing", expectedState: "unproven"},
		{name: "disabled comparison", proofMode: "disabled", expectedState: "disabled"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", "", test.proofMode)

			// Then
			require.NoError(t, err)
			assert.NotEmpty(t, run.Output)
			evidence := collectVisualEvidence(run.Output)
			encoded, marshalErr := json.Marshal(evidence)
			require.NoError(t, marshalErr)
			publicEvidence := strings.ToLower(string(encoded))
			assert.Contains(t, publicEvidence, "snapshot")
			assert.Contains(t, publicEvidence, test.expectedState)
		})
	}
}

func TestRunPlaywright_MissingProof_PreservesDiagnosticInsideV2Proof(t *testing.T) {
	// Given / When
	run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", "", "missing")
	require.NoError(t, err)
	evidence := collectVisualEvidence(run.Output)

	// Then
	assert.Equal(t, "unproven", evidence.SnapshotProof.Status)
	assert.Contains(t, evidence.SnapshotProof.Diagnostic, "missing")
}

func TestCombineVerifyErrors_PreservesProcessAndReportFailures(t *testing.T) {
	// Given
	processErr := errors.New("process sentinel")
	reportErr := errors.New("report sentinel")

	// When
	err := combineVerifyErrors(processErr, reportErr)

	// Then
	require.Error(t, err)
	assert.ErrorIs(t, err, processErr)
	assert.ErrorIs(t, err, reportErr)
	assert.Contains(t, err.Error(), "playwright 실행 실패")
}

func TestWriteVerifyVisualGate_FailedSnapshotProof_BlocksOnlyStrictMode(t *testing.T) {
	tests := []struct {
		name    string
		strict  bool
		wantErr bool
	}{
		{name: "default report mode", strict: false, wantErr: false},
		{name: "strict visual gate", strict: true, wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// Given
			assertions := []design.VisualAssertion{{
				Name:       "snapshot-comparison-proof",
				Project:    "chromium",
				Status:     "FAIL",
				Diagnostic: "snapshot comparison state is unproven",
			}}

			// When
			err := writeVerifyVisualGate(
				t.TempDir(),
				[]string{"src/App.tsx"},
				nil,
				nil,
				assertions,
				[]string{"chromium"},
				"desktop",
				design.Context{Found: true, SourcePath: "DESIGN.md"},
				2,
				nil,
				test.strict,
				"",
			)

			// Then
			if test.wantErr {
				assert.ErrorContains(t, err, "strict visual gate failed")
				return
			}
			assert.NoError(t, err)
		})
	}
}
