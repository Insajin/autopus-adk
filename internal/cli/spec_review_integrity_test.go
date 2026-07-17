package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// integStatus builds a ProviderStatus with the given machine status string.
func integStatus(name, status string) spec.ProviderStatus {
	return spec.ProviderStatus{Provider: name, Status: status}
}

// TestObservationDegradedReasons_Matrix pins the machine-readable degraded
// reasons for the coverage x quorum matrix. Order is stable: partial_doc_context
// precedes provider_quorum (REQ-RINT-TRUNC-04, REQ-RINT-QUORUM-05).
func TestObservationDegradedReasons_Matrix(t *testing.T) {
	fullCov := []spec.DocCoverage{
		spec.ComputeCoverage("plan.md", 150, 150),     // 100%, complete
		spec.ComputeCoverage("research.md", 250, 250), // 100%, complete
	}
	// research.md injected 200 of 250 -> percent 80, incomplete (AC-RINT-COV-1).
	partialCov := []spec.DocCoverage{
		spec.ComputeCoverage("plan.md", 150, 150),
		spec.ComputeCoverage("research.md", 200, 250),
	}
	quorumMet := []spec.ProviderStatus{
		integStatus("claude", "success"),
		integStatus("gemini", "success"),
		integStatus("codex", "timeout"),
	}
	subQuorum := []spec.ProviderStatus{
		integStatus("claude", "success"),
		integStatus("gemini", "timeout"),
		integStatus("codex", "timeout"),
	}

	tests := []struct {
		name       string
		coverages  []spec.DocCoverage
		statuses   []spec.ProviderStatus
		configured int
		minProv    int
		want       []string
	}{
		{"full coverage + quorum met", fullCov, quorumMet, 3, 0, nil},
		{"partial coverage + quorum met", partialCov, quorumMet, 3, 0, []string{spec.DegradedReasonPartialDocContext}},
		{"full coverage + sub-quorum", fullCov, subQuorum, 3, 0, []string{spec.DegradedReasonProviderQuorum}},
		{
			"partial coverage + sub-quorum",
			partialCov, subQuorum, 3, 0,
			[]string{spec.DegradedReasonPartialDocContext, spec.DegradedReasonProviderQuorum},
		},
		// Single-provider local review (n=1, quorum 1) must not degrade.
		{"single provider met", fullCov, []spec.ProviderStatus{integStatus("claude", "success")}, 1, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := observationDegradedReasons(tt.coverages, tt.statuses, tt.configured, tt.minProv)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestComputeCoverageOracle pins the concrete coverage values AC-RINT-COV-1
// depends on: 250/200 -> 80 incomplete, 150/150 -> 100 complete.
func TestComputeCoverageOracle(t *testing.T) {
	research := spec.ComputeCoverage("research.md", 200, 250)
	assert.Equal(t, 80, research.Percent)
	assert.False(t, research.Complete)

	plan := spec.ComputeCoverage("plan.md", 150, 150)
	assert.Equal(t, 100, plan.Percent)
	assert.True(t, plan.Complete)
}

// TestEvaluateIntegrityGate covers the promotion decision truth table
// (REQ-RINT-PROMO-06, REQ-RINT-OVERRIDE-07).
func TestEvaluateIntegrityGate(t *testing.T) {
	tests := []struct {
		name          string
		reasons       []string
		allowDegraded bool
		wantPromote   bool
		wantOverride  bool
	}{
		{"clean pass promotes", nil, false, true, false},
		{"degraded without override is blocked", []string{spec.DegradedReasonProviderQuorum}, false, false, false},
		{"degraded with override promotes via override", []string{spec.DegradedReasonProviderQuorum}, true, true, true},
		{"partial doc with override promotes via override", []string{spec.DegradedReasonPartialDocContext}, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateIntegrityGate(tt.reasons, tt.allowDegraded)
			assert.Equal(t, tt.wantPromote, got.promote)
			assert.Equal(t, tt.wantOverride, got.viaOverride)
		})
	}
}

// TestIntegrityMessages pins the operator-facing stdout formats: the block
// message names the reasons and the exact remedy hint, and the override audit
// line names --allow-degraded and the reasons.
func TestIntegrityMessages(t *testing.T) {
	block := formatIntegrityBlockMessage([]string{spec.DegradedReasonPartialDocContext, spec.DegradedReasonProviderQuorum})
	assert.Contains(t, block, "partial_doc_context")
	assert.Contains(t, block, "provider_quorum")
	assert.Contains(t, block, "--allow-degraded")
	assert.Contains(t, block, integrityRemedyHint)

	audit := formatIntegrityOverrideAudit([]string{spec.DegradedReasonProviderQuorum})
	assert.Contains(t, audit, "--allow-degraded")
	assert.Contains(t, audit, "provider_quorum")
}

// TestSyncReviewedSpecStatus_IntegrityGate exercises the promotion gate directly
// against a scaffolded SPEC for the block / override / clean-pass paths.
func TestSyncReviewedSpecStatus_IntegrityGate(t *testing.T) {
	newPassResult := func(reasons []string) *spec.ReviewResult {
		return &spec.ReviewResult{
			SpecID:          "SPEC-GATE-UNIT-001",
			Verdict:         spec.VerdictPass,
			DegradedReasons: reasons,
			ProviderStatuses: []spec.ProviderStatus{
				integStatus("claude", "success"),
			},
		}
	}

	t.Run("clean pass promotes to approved", func(t *testing.T) {
		dir := t.TempDir()
		specDir := scaffoldReviewSpec(t, dir, "SPEC-GATE-CLEAN-001")
		require.NoError(t, syncReviewedSpecStatus(specDir, newPassResult(nil), false))
		doc, err := spec.Load(specDir)
		require.NoError(t, err)
		assert.Equal(t, "approved", doc.Status)
	})

	t.Run("degraded PASS without override leaves prior status", func(t *testing.T) {
		dir := t.TempDir()
		specDir := scaffoldReviewSpec(t, dir, "SPEC-GATE-BLOCK-001")
		before, err := spec.Load(specDir)
		require.NoError(t, err)

		result := newPassResult([]string{spec.DegradedReasonProviderQuorum})
		require.NoError(t, syncReviewedSpecStatus(specDir, result, false))

		after, err := spec.Load(specDir)
		require.NoError(t, err)
		assert.Equal(t, before.Status, after.Status)
		assert.NotEqual(t, "approved", after.Status)
		assert.False(t, result.OverridePromotion)
	})

	t.Run("PASS with active finding never reaches integrity gate", func(t *testing.T) {
		// Pre-integrity behavior: an active blocking finding holds promotion even
		// with full observation and no degraded reasons. The gate must not change it.
		dir := t.TempDir()
		specDir := scaffoldReviewSpec(t, dir, "SPEC-GATE-FINDING-001")
		before, err := spec.Load(specDir)
		require.NoError(t, err)

		result := newPassResult(nil)
		result.Findings = []spec.ReviewFinding{{
			ID:       "F-001",
			Status:   spec.FindingStatusOpen,
			Severity: "major",
			Category: spec.FindingCategoryCorrectness,
		}}
		require.NoError(t, syncReviewedSpecStatus(specDir, result, false))

		after, err := spec.Load(specDir)
		require.NoError(t, err)
		assert.Equal(t, before.Status, after.Status)
		assert.NotEqual(t, "approved", after.Status)
	})

	t.Run("degraded PASS with override promotes and audits", func(t *testing.T) {
		dir := t.TempDir()
		specDir := scaffoldReviewSpec(t, dir, "SPEC-GATE-OVERRIDE-001")

		result := newPassResult([]string{spec.DegradedReasonProviderQuorum})
		require.NoError(t, syncReviewedSpecStatus(specDir, result, true))

		doc, err := spec.Load(specDir)
		require.NoError(t, err)
		assert.Equal(t, "approved", doc.Status)
		assert.True(t, result.OverridePromotion)

		reviewBytes, err := os.ReadFile(filepath.Join(specDir, "review.md"))
		require.NoError(t, err)
		reviewText := string(reviewBytes)
		assert.Contains(t, reviewText, "--allow-degraded",
			"override promotion must record the allow-degraded audit line in review.md")
		assert.Contains(t, reviewText, "provider_quorum")
	})
}
