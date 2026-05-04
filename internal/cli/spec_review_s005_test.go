package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// TestRunSpecReview_FailedProviderRejectIgnored covers SPEC-SPECREV-001 S-005
// hardening: a failed provider (TimedOut or ExitCode != 0) emitting a partial
// "VERDICT: REJECT" in stdout MUST NOT trigger the REJECT short-circuit. The
// surviving providers' verdicts decide the merged outcome.
func TestRunSpecReview_FailedProviderRejectIgnored(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-S005-GUARD-001")
	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		// Default config has 2 providers (claude, gemini).
		// claude succeeds with PASS; gemini failed (ExitCode=1) but its partial
		// stdout contains a spurious "VERDICT: REJECT" — must be ignored.
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{
			{Provider: "claude", ExitCode: 0, Output: "VERDICT: PASS"},
			{Provider: "gemini", ExitCode: 1, Error: "subprocess crash", Output: "partial output\nVERDICT: REJECT"},
		}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-S005-GUARD-001", "consensus", 10))

	// Read review.md and verify the verdict was NOT short-circuited to REJECT.
	reviewBytes, err := os.ReadFile(filepath.Join(specDir, "review.md"))
	require.NoError(t, err)
	reviewText := string(reviewBytes)
	assert.NotContains(t, reviewText, "**Verdict**: REJECT",
		"failed provider's partial REJECT must not short-circuit the merged verdict")
	// gemini failed → 1/2 ratio = 50% → degraded label expected.
	assert.Contains(t, reviewText, "(degraded — 1/2 providers responded)",
		"degraded label must reflect 1 success out of 2 configured providers")
}
