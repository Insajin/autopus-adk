package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncReviewedSpecStatusWithReceipt_ProjectsJudgeSummary(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-JUDGE-RECEIPT-001")
	result := newSyncPassResult(nil)
	result.Judge = &spec.JudgeSummary{
		Provider: "claude", Family: "anthropic", Status: "invalid", Verdict: "",
		Accepted: 0, Rejected: 1, Merged: 1, Reason: "invalid review judge JSON",
	}

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, result, false)

	require.NoError(t, err)
	require.NotNil(t, receipt.Judge)
	assert.Equal(t, "claude", receipt.Judge.Provider)
	assert.Equal(t, "anthropic", receipt.Judge.Family)
	assert.Equal(t, "invalid", receipt.Judge.Status)
	assert.Equal(t, 0, receipt.Judge.Accepted)
	assert.Equal(t, 1, receipt.Judge.Rejected)
	assert.Equal(t, 1, receipt.Judge.Merged)
	assert.Equal(t, "invalid review judge JSON", receipt.Judge.Reason)
	assert.Empty(t, receipt.Judge.AcceptedIDs)
	assert.Empty(t, receipt.Judge.Rationale)

	path, err := persistSpecReviewPromotionReceipt(specDir, receipt)
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	var decoded struct {
		Judge *specReviewJudgeReceipt `json:"judge"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.NotNil(t, decoded.Judge)
	assert.Equal(t, *receipt.Judge, *decoded.Judge)
}

func TestSyncReviewedSpecStatusWithReceipt_PersistsJudgeEvidenceAndProviderBackend(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-JUDGE-RECEIPT-002")
	result := newSyncPassResult(nil)
	result.ProviderStatuses[0].Backend = "omp"
	result.Judge = &spec.JudgeSummary{
		Provider: "claude", Family: "anthropic", Status: "ok", Verdict: "PASS",
		Accepted: 1, AcceptedIDs: []string{"F-001"}, Rationale: strings.Repeat("x", 600),
	}

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, result, false)
	require.NoError(t, err)
	path, err := persistSpecReviewPromotionReceipt(specDir, receipt)
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded struct {
		Judge     *specReviewJudgeReceipt `json:"judge"`
		Providers []spec.ProviderStatus   `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.NotNil(t, decoded.Judge)
	assert.Equal(t, []string{"F-001"}, decoded.Judge.AcceptedIDs)
	assert.Equal(t, strings.Repeat("x", 500), decoded.Judge.Rationale)
	require.Len(t, decoded.Providers, 1)
	assert.Equal(t, "omp", decoded.Providers[0].Backend)
	assert.Contains(t, string(body), `"executed_backend": "omp"`)
	assert.Contains(t, string(body), `"accepted_ids": [`)
	assert.Contains(t, string(body), `"rationale":`)
}

func TestPersistSpecReviewPromotionReceipt_OmitsAbsentJudge(t *testing.T) {
	specDir := t.TempDir()

	path, err := persistSpecReviewPromotionReceipt(specDir, specReviewPromotionReceipt{
		Schema: specReviewPromotionReceiptSchema, DegradedReasons: []string{},
	})

	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(body), `"judge"`)
}
