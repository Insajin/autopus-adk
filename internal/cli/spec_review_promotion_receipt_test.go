package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestSyncReviewedSpecStatusWithReceipt_CleanPassRecordsStatusChange(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-PROMOTION-RECEIPT-001")

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, newSyncPassResult(nil), false)

	require.NoError(t, err)
	assert.Equal(t, specReviewPromotionReceiptSchema, receipt.Schema)
	assert.True(t, receipt.StatusChanged)
	assert.Equal(t, "draft", receipt.PreviousStatus)
	assert.Equal(t, "approved", receipt.CurrentStatus)
	assert.Empty(t, receipt.DegradedReasons)
	assert.False(t, receipt.OverrideApplied)
	assert.Equal(t, "PASS", receipt.AnalysisVerdict)
	assert.Equal(t, "passed", receipt.GateStatus)
	assert.False(t, receipt.CriticalVeto)
	assert.NotEmpty(t, receipt.RunID)
	assert.NotEmpty(t, receipt.FinishedAt)
}

func TestSyncReviewedSpecStatusWithReceipt_DegradedPassRecordsBlockedPromotion(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-PROMOTION-RECEIPT-002")
	reasons := []string{spec.DegradedReasonProviderQuorum}

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, newSyncPassResult(reasons), false)

	require.NoError(t, err)
	assert.False(t, receipt.StatusChanged)
	assert.Equal(t, reasons, receipt.DegradedReasons)
	assert.False(t, receipt.OverrideApplied)
	assert.Equal(t, "draft", receipt.CurrentStatus)
	assert.Equal(t, "degraded", receipt.GateStatus)
}

func TestPersistSpecReviewPromotionReceipt_WritesExactMachineFields(t *testing.T) {
	specDir := t.TempDir()
	receipt := specReviewPromotionReceipt{
		Schema:          specReviewPromotionReceiptSchema,
		StatusChanged:   true,
		DegradedReasons: []string{"provider_quorum"},
		OverrideApplied: true,
	}

	path, err := persistSpecReviewPromotionReceipt(specDir, receipt)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(specDir, "review-receipt.json"), path)
	body, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, true, decoded["status_changed"])
	assert.Equal(t, true, decoded["override_applied"])
	assert.Contains(t, decoded, "degraded_reasons")
	assert.Contains(t, decoded, "analysis_verdict")
	assert.Contains(t, decoded, "gate_status")
	assert.Contains(t, decoded, "critical_veto")
	assert.Contains(t, decoded, "run_id")
	assert.Contains(t, decoded, "finished_at")
}

func TestSyncReviewedSpecStatusWithReceipt_CurrentRunEvidenceAndApprovedRereview(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-PROMOTION-RECEIPT-003")
	require.NoError(t, spec.UpdateStatus(specDir, "approved"))
	finishedAt := time.Date(2026, time.July, 20, 2, 3, 4, 5, time.UTC)

	receipt, err := syncReviewedSpecStatusWithReceipt(
		specDir, newSyncPassResult(nil), false,
		specReviewRuntimeEvidence{RunID: "spec-review-current-run", FinishedAt: finishedAt},
	)

	require.NoError(t, err)
	assert.False(t, receipt.StatusChanged, "clean re-review of approved SPEC is not a false blocker")
	assert.Equal(t, "approved", receipt.CurrentStatus)
	assert.Equal(t, "passed", receipt.GateStatus)
	assert.Equal(t, "spec-review-current-run", receipt.RunID)
	assert.Equal(t, finishedAt.Format(time.RFC3339Nano), receipt.FinishedAt)
}

func TestSyncReviewedSpecStatusWithReceipt_CriticalFindingBlocksAndVetoes(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-PROMOTION-RECEIPT-004")
	result := newSyncPassResult(nil)
	result.Findings = []spec.ReviewFinding{{
		ID: "F-CRITICAL", Severity: "critical", Category: spec.FindingCategorySecurity,
		Status: spec.FindingStatusOpen,
	}}

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, result, false)

	require.NoError(t, err)
	assert.False(t, receipt.StatusChanged)
	assert.Equal(t, "blocked", receipt.GateStatus)
	assert.True(t, receipt.CriticalVeto)
	assert.Equal(t, "draft", receipt.CurrentStatus)
}

func TestHasCriticalSpecReviewVeto_ExactTruthTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		severity string
		category spec.FindingCategory
		status   spec.FindingStatus
		want     bool
	}{
		{name: "critical security", severity: "critical", category: spec.FindingCategorySecurity, status: spec.FindingStatusOpen, want: true},
		{name: "critical correctness", severity: "critical", category: spec.FindingCategoryCorrectness, status: spec.FindingStatusRegressed, want: true},
		{name: "major security", severity: "major", category: spec.FindingCategorySecurity, status: spec.FindingStatusOpen, want: false},
		{name: "critical completeness", severity: "critical", category: spec.FindingCategoryCompleteness, status: spec.FindingStatusOpen, want: false},
		{name: "resolved critical security", severity: "critical", category: spec.FindingCategorySecurity, status: spec.FindingStatusResolved, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			finding := spec.ReviewFinding{Severity: tc.severity, Category: tc.category, Status: tc.status}
			assert.Equal(t, tc.want, hasCriticalSpecReviewVeto([]spec.ReviewFinding{finding}))
		})
	}
}

func TestSpecReviewCommand_RegistersProviderForwardingFlag(t *testing.T) {
	t.Parallel()

	cmd := newSpecReviewCmd()

	assert.NotNil(t, cmd.Flags().Lookup("providers"))
}
