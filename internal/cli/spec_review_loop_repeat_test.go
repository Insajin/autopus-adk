package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

// repeatReviewOutput restates two findings the review already knows: one by
// title (REQ-001, only punctuation differs) and one by location (the resolved
// pkg/auth/login.go:42 finding, worded differently).
const repeatReviewOutput = `{"verdict":"REVISE","summary":"still broken",` +
	`"finding_statuses":[{"id":"F-001","status":"open","reason":"unchanged"},` +
	`{"id":"F-002","status":"resolved","reason":"fixed"}],` +
	`"findings":[` +
	`{"severity":"major","category":"correctness","scope_ref":"REQ-001",` +
	`"description":"Missing REQ-001 coverage!","suggestion":"Add coverage."},` +
	`{"severity":"major","category":"correctness","scope_ref":"pkg/auth/login.go:42",` +
	`"description":"Session lifetime is unbounded","suggestion":"Bound the session."}]}`

func repeatReviewPriorFindings() []spec.ReviewFinding {
	return []spec.ReviewFinding{
		{
			ID: "F-001", Severity: "major", Category: spec.FindingCategoryCorrectness,
			ScopeRef: "REQ-001", Description: "Missing REQ-001 coverage",
			Status: spec.FindingStatusOpen,
		},
		{
			ID: "F-002", Severity: "major", Category: spec.FindingCategoryCorrectness,
			ScopeRef: "pkg/auth/login.go:42", Description: "Token never expires",
			Status: spec.FindingStatusResolved,
		},
	}
}

func TestRunSpecReviewLoop_RecordsRepeatDiscoveryAndSameInputReReview(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-REPEAT-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{
			{Provider: "claude", Output: repeatReviewOutput},
			{Provider: "codex", Output: repeatReviewOutput},
		}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, repeatReviewPriorFindings())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.SameInputReReview,
		"an unchanged SPEC re-reviewed in the next revision is a same-input re-review")
	require.Len(t, result.RepeatDiscoveries, 2)
	assert.Equal(t, 1, result.RepeatDiscoveries[0].Revision)
	matched := map[string]string{}
	for _, r := range result.RepeatDiscoveries {
		matched[r.FindingID] = r.MatchedPriorID
	}
	assert.Equal(t, map[string]string{"F-005": "F-003", "F-006": "F-004"}, matched)

	persisted, err := spec.LoadFindings(specDir)
	require.NoError(t, err)
	byID := map[string]spec.ReviewFinding{}
	for _, f := range persisted {
		byID[f.ID] = f
	}
	require.Contains(t, byID, "F-003")
	assert.Equal(t, "F-001", byID["F-003"].RepeatOf, "title restatement is tagged as a repeat")
	assert.Equal(t, spec.FindingStatusOutOfScope, byID["F-003"].Status)
	require.Contains(t, byID, "F-004")
	assert.Equal(t, "F-002", byID["F-004"].RepeatOf, "file:line restatement is tagged as a repeat")
	assert.Equal(t, spec.FindingStatusOutOfScope, byID["F-004"].Status,
		"a repeat must not be re-opened as fresh work")
}

func TestRunSpecReviewLoop_RepeatDiscoveryReachesReviewReceipt(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-REPEAT-002"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{
			{Provider: "claude", Output: repeatReviewOutput},
			{Provider: "codex", Output: repeatReviewOutput},
		}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, repeatReviewPriorFindings())
	require.NoError(t, err)

	receipt, err := syncReviewedSpecStatusWithReceipt(specDir, result, false)
	require.NoError(t, err)
	path, err := persistSpecReviewPromotionReceipt(specDir, receipt)
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded struct {
		RepeatDiscoveries       []spec.RepeatDiscovery `json:"repeat_discoveries"`
		RepeatDiscoveryCount    int                    `json:"repeat_discovery_count"`
		SameInputReReview       bool                   `json:"same_input_rereview"`
		DiscoveryRepeatDetected bool                   `json:"discovery_repeat_detected"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, 2, decoded.RepeatDiscoveryCount)
	assert.True(t, decoded.SameInputReReview)
	assert.True(t, decoded.DiscoveryRepeatDetected)
	require.Len(t, decoded.RepeatDiscoveries, 2)
	assert.Equal(t, "F-003", decoded.RepeatDiscoveries[0].MatchedPriorID)
}

func TestApplySpecReviewRepeatDiscovery_LeavesConvergedReceiptBytesUnchanged(t *testing.T) {
	specDir := t.TempDir()
	receipt := specReviewPromotionReceipt{
		Schema: specReviewPromotionReceiptSchema, DegradedReasons: []string{},
	}

	applySpecReviewRepeatDiscovery(&receipt, &spec.ReviewResult{Verdict: spec.VerdictPass})
	path, err := persistSpecReviewPromotionReceipt(specDir, receipt)

	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "repeat_discover")
	assert.NotContains(t, string(body), "same_input_rereview")
	assert.False(t, receipt.DiscoveryRepeatDetected)
}

func TestApplySpecReviewRepeatDiscovery_SameInputAloneCountsAsDetection(t *testing.T) {
	receipt := specReviewPromotionReceipt{Schema: specReviewPromotionReceiptSchema}

	applySpecReviewRepeatDiscovery(&receipt, &spec.ReviewResult{SameInputReReview: true})

	assert.Equal(t, 0, receipt.RepeatDiscoveryCount)
	assert.True(t, receipt.DiscoveryRepeatDetected,
		"re-reviewing identical input is a convergence failure even with zero repeats")
}

func TestSpecReviewInputSnapshot_TracksSpecEditsAndIgnoresReviewOutput(t *testing.T) {
	specDir := scaffoldReviewSpec(t, t.TempDir(), "SPEC-REVIEW-SNAPSHOT-001")

	base := specReviewInputSnapshot(specDir)
	require.NotEmpty(t, base)

	require.NoError(t, os.WriteFile(filepath.Join(specDir, "review.md"), []byte("# rewritten\n"), 0o600))
	assert.Equal(t, base, specReviewInputSnapshot(specDir),
		"the loop rewrites review.md every revision; it cannot count as changed input")

	specPath := filepath.Join(specDir, "spec.md")
	body, err := os.ReadFile(specPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(specPath, append(body, []byte("\nnew requirement\n")...), 0o600))
	assert.NotEqual(t, base, specReviewInputSnapshot(specDir))

	assert.Empty(t, specReviewInputSnapshot(filepath.Join(specDir, "missing")))
}

func TestPrintSpecReviewRepeatSummary_SilentUnlessDetected(t *testing.T) {
	var quiet bytes.Buffer
	printSpecReviewRepeatSummary(&quiet, &spec.ReviewResult{Verdict: spec.VerdictPass})
	assert.Empty(t, quiet.String())

	var noisy bytes.Buffer
	printSpecReviewRepeatSummary(&noisy, &spec.ReviewResult{
		SameInputReReview: true,
		RepeatDiscoveries: []spec.RepeatDiscovery{{FindingID: "F-003", MatchedPriorID: "F-001"}},
	})
	assert.Equal(t, "repeat discoveries: 1 (same input: yes)\n", noisy.String())

	var nilResult bytes.Buffer
	printSpecReviewRepeatSummary(&nilResult, nil)
	assert.Empty(t, nilResult.String())
}
