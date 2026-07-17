package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// Unit oracles for syncReviewedSpecStatus (SPEC-ADK-REVIEW-INTEGRITY-001).
//
// These exercise the status-transition function in isolation — no orchestra
// loop, no chdir — so each branch of the promotion gate is proven directly:
// the clean-PASS promote, the degraded block, the --allow-degraded override,
// the non-PASS early returns, the shipped-status regression guard (issue #38),
// and the nil no-op. The end-to-end coverage through the review loop lives in
// spec_review_integrity_gate_test.go; the gate/format tables live in
// spec_review_integrity_test.go. This file closes the sync-layer half.

// newSyncPassResult builds a minimal PASS ReviewResult carrying the given
// machine-readable degraded reasons and one successful provider. It has no
// findings, so it clears the pre-integrity verdict/finding guard and reaches
// the observation-integrity promotion gate.
func newSyncPassResult(reasons []string) *spec.ReviewResult {
	return &spec.ReviewResult{
		SpecID:           "SPEC-SYNCSTATUS-UNIT-001",
		Verdict:          spec.VerdictPass,
		DegradedReasons:  reasons,
		ProviderStatuses: []spec.ProviderStatus{{Provider: "claude", Status: "success"}},
	}
}

// statusOf loads spec.md from a scaffolded SPEC dir and returns its parsed
// status, failing the test on a load error.
func statusOf(t *testing.T, specDir string) string {
	t.Helper()
	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	return doc.Status
}

// TestSyncStatus_CleanPassPromotesToApproved locks the happy path: a PASS with
// no degraded reasons advances a draft SPEC to approved (REQ-RINT-PROMO-06).
func TestSyncStatus_CleanPassPromotesToApproved(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-CLEAN-001")
	require.Equal(t, "draft", statusOf(t, specDir), "fixture must start at draft")

	require.NoError(t, syncReviewedSpecStatus(specDir, newSyncPassResult(nil), false))

	assert.Equal(t, "approved", statusOf(t, specDir),
		"a clean PASS with no degraded reasons must auto-promote to approved")
}

// TestSyncStatus_DegradedWithoutOverrideStaysDraftAndWarns proves the block
// path: a partial_doc_context PASS without --allow-degraded holds the status at
// draft, records no override, prints the remedy message, and does not persist
// review.md (that artifact belongs to the loop, not the block path).
func TestSyncStatus_DegradedWithoutOverrideStaysDraftAndWarns(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-BLOCK-001")

	result := newSyncPassResult([]string{spec.DegradedReasonPartialDocContext})
	out := captureStdout(t, func() {
		require.NoError(t, syncReviewedSpecStatus(specDir, result, false))
	})

	assert.Equal(t, "draft", statusOf(t, specDir),
		"a degraded PASS without --allow-degraded must not leave draft")
	assert.False(t, result.OverridePromotion, "no override may be recorded on the block path")
	assert.Contains(t, out, spec.DegradedReasonPartialDocContext,
		"the block message must name the blocking reason")
	assert.Contains(t, out, "--allow-degraded",
		"the block message must name the override escape hatch")

	_, statErr := os.Stat(filepath.Join(specDir, "review.md"))
	assert.True(t, os.IsNotExist(statErr), "block path must not persist review.md")
}

// TestSyncStatus_DegradedWithOverridePromotesAndAudits proves the override
// path: --allow-degraded promotes a partial_doc_context PASS to approved, flags
// OverridePromotion, prints the audit line, and persists review.md carrying the
// **Promotion Override**: marker naming the overridden reason (REQ-RINT-OVERRIDE-07).
func TestSyncStatus_DegradedWithOverridePromotesAndAudits(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-OVERRIDE-001")

	result := newSyncPassResult([]string{spec.DegradedReasonPartialDocContext})
	out := captureStdout(t, func() {
		require.NoError(t, syncReviewedSpecStatus(specDir, result, true))
	})

	assert.Equal(t, "approved", statusOf(t, specDir),
		"--allow-degraded must promote a degraded PASS to approved")
	assert.True(t, result.OverridePromotion,
		"the override path must flag OverridePromotion so review.md renders the audit line")
	assert.Contains(t, out, "--allow-degraded", "override audit must print to stdout")
	assert.Contains(t, out, spec.DegradedReasonPartialDocContext)

	reviewBytes, err := os.ReadFile(filepath.Join(specDir, "review.md"))
	require.NoError(t, err, "override path must persist review.md")
	reviewText := string(reviewBytes)
	assert.Contains(t, reviewText, "**Promotion Override**:",
		"review.md must carry the promotion-override audit marker")
	assert.Contains(t, reviewText, spec.DegradedReasonPartialDocContext,
		"the audit line must name the overridden reason")
}

// TestSyncStatus_ReviseVerdictLeavesStatusUntouched proves a non-PASS verdict
// returns before the gate: even with --allow-degraded set, a REVISE result must
// never promote and must record no override.
func TestSyncStatus_ReviseVerdictLeavesStatusUntouched(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-REVISE-001")

	result := &spec.ReviewResult{SpecID: "SPEC-SYNCSTATUS-REVISE-001", Verdict: spec.VerdictRevise}
	require.NoError(t, syncReviewedSpecStatus(specDir, result, true))

	assert.Equal(t, "draft", statusOf(t, specDir),
		"a REVISE verdict must not advance status regardless of allow-degraded")
	assert.False(t, result.OverridePromotion)
}

// TestSyncStatus_RejectVerdictLeavesStatusUntouched proves the REJECT verdict
// also short-circuits before the promotion gate.
func TestSyncStatus_RejectVerdictLeavesStatusUntouched(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-REJECT-001")

	result := &spec.ReviewResult{SpecID: "SPEC-SYNCSTATUS-REJECT-001", Verdict: spec.VerdictReject}
	require.NoError(t, syncReviewedSpecStatus(specDir, result, false))

	assert.Equal(t, "draft", statusOf(t, specDir), "a REJECT verdict must not advance status")
}

// TestSyncStatus_ShippedStatusNeverRegresses is the issue #38 regression guard
// at the sync layer: a clean PASS that would otherwise promote must never
// rewrite an already-shipped status back to approved. Both shipped keys are
// pinned; the loop-level test covers "completed", this pins "implemented" too.
func TestSyncStatus_ShippedStatusNeverRegresses(t *testing.T) {
	for _, shipped := range []string{"implemented", "completed"} {
		shipped := shipped
		t.Run(shipped, func(t *testing.T) {
			dir := t.TempDir()
			specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-SHIPPED-"+shipped+"-001")
			require.NoError(t, spec.UpdateStatus(specDir, shipped))

			require.NoError(t, syncReviewedSpecStatus(specDir, newSyncPassResult(nil), false))

			assert.Equal(t, shipped, statusOf(t, specDir),
				"a PASS review must not silently regress shipped status")
		})
	}
}

// TestSyncStatus_ShippedStatusOverrideStillNeverRegresses proves the shipped
// guard runs before the promotion gate: even a degraded PASS with
// --allow-degraded cannot regress a shipped status, because the guard returns
// first.
func TestSyncStatus_ShippedStatusOverrideStillNeverRegresses(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-SHIPPED-OVR-001")
	require.NoError(t, spec.UpdateStatus(specDir, "completed"))

	result := newSyncPassResult([]string{spec.DegradedReasonProviderQuorum})
	require.NoError(t, syncReviewedSpecStatus(specDir, result, true))

	assert.Equal(t, "completed", statusOf(t, specDir),
		"the shipped-status guard must fire before the override path")
	assert.False(t, result.OverridePromotion,
		"a shipped SPEC must not record an override it never reached")
}

// TestSyncStatus_NilResultIsNoOp proves the defensive nil guard: a nil result
// is a safe no-op that neither errors nor mutates status.
func TestSyncStatus_NilResultIsNoOp(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-SYNCSTATUS-NIL-001")

	require.NoError(t, syncReviewedSpecStatus(specDir, nil, false), "a nil result must be a safe no-op")
	assert.Equal(t, "draft", statusOf(t, specDir), "a nil result must not change status")
}
