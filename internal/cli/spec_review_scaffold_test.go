package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

// TestRunSpecReviewRejectsEmptySpecContent verifies that runSpecReview returns an
// error containing "SPEC 본문이 비어있습니다" when the loaded SPEC has empty RawContent,
// and that the orchestra mock is never called (CallCount == 0).
func TestRunSpecReviewRejectsEmptySpecContent(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()
	specID := "SPEC-EMPTY-CONTENT-001"
	specDir := filepath.Join(dir, ".autopus", "specs", specID)
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	// Write an empty spec.md so that spec.Load sets RawContent == ""
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(""), 0o644))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		return &orchestra.OrchestraResult{}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	// TARGET API: runSpecReview must validate RawContent before calling orchestra
	execErr := runSpecReview(context.Background(), specID, "consensus", 10)
	require.Error(t, execErr)
	assert.Contains(t, execErr.Error(), "SPEC 본문이 비어있습니다")
	assert.Equal(t, 0, callCount, "orchestra must not be called when SPEC body is empty")
}

// TestRunSpecReviewMergesDuplicateFindingsAcrossProviders verifies that identical
// findings from 3 providers are deduplicated to a single finding with ID == "F-001"
// and all stored finding IDs are globally unique.
func TestRunSpecReviewMergesDuplicateFindingsAcrossProviders(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-DEDUP-001")
	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	origRunner := specReviewRunOrchestra
	// Each of the 3 providers emits the identical finding
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: "VERDICT: PASS\nFINDING: [major] [correctness] pkg/spec/reviewer.go:54 seq restarts at 1 per provider"},
				{Provider: "codex", Output: "VERDICT: PASS\nFINDING: [major] [correctness] pkg/spec/reviewer.go:54 seq restarts at 1 per provider"},
				{Provider: "gemini", Output: "VERDICT: PASS\nFINDING: [major] [correctness] pkg/spec/reviewer.go:54 seq restarts at 1 per provider"},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-DEDUP-001", "consensus", 10))

	// Verify persisted findings are deduplicated
	findings, err := spec.LoadFindings(specDir)
	require.NoError(t, err)
	assert.Len(t, findings, 1, "3 identical findings must be deduplicated to 1")
	assert.Equal(t, "F-001", findings[0].ID, "deduplicated finding must have globally sequential ID F-001")

	// All IDs must be unique
	idSet := make(map[string]bool)
	for _, f := range findings {
		assert.False(t, idSet[f.ID], "duplicate finding ID %q detected", f.ID)
		idSet[f.ID] = true
	}
}

// TestRunSpecReviewReloadsSpecBetweenRevisions verifies that spec.md is reloaded
// between revision iterations so that edits to spec.md are reflected in subsequent prompts.
func TestRunSpecReviewReloadsSpecBetweenRevisions(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-RELOAD-001")

	// Write initial spec.md containing only REQ-A
	initialContent := "# SPEC-RELOAD-001\n\n## Title\nReload Test\n\n## Requirements\n\n- REQ-A: WHEN event occurs THEN system SHALL respond\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(initialContent), 0o644))

	setFakeProviderOnPath(t, dir, "claude")

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	revision := 0
	var capturedPrompts []string

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		capturedPrompts = append(capturedPrompts, cfg.Prompt)

		if revision == 0 {
			// After first review, mutate spec.md to add REQ-B
			updatedContent := initialContent + "- REQ-B: WHEN condition is met THEN system SHALL update state\n"
			_ = os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(updatedContent), 0o644)
			revision++
			return &orchestra.OrchestraResult{
				Responses: []orchestra.ProviderResponse{
					{Provider: "claude", Output: "VERDICT: REVISE\nFINDING: [major] [correctness] REQ-A missing details"},
					{Provider: "gemini", Output: "VERDICT: REVISE"},
				},
			}, nil
		}
		// revision 1: return PASS from both providers so SPEC-SPECREV-001 AC-VERD-1
		// dropped-provider rule does not block the PASS path under the default
		// 2-provider config.
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: "VERDICT: PASS"},
				{Provider: "gemini", Output: "VERDICT: PASS"},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	require.NoError(t, runSpecReview(context.Background(), "SPEC-RELOAD-001", "consensus", 10))

	// Must have run at least 2 iterations
	require.GreaterOrEqual(t, len(capturedPrompts), 2, "review loop must have run at least 2 iterations")

	// revision 1 prompt must contain REQ-B (newly loaded content)
	assert.Contains(t, capturedPrompts[1], "REQ-B", "revision 1 prompt must reflect reloaded spec.md with REQ-B")

	// Final verdict must be PASS — circuit breaker must not have prevented completion
	doc, loadErr := spec.Load(specDir)
	require.NoError(t, loadErr)
	assert.Equal(t, "approved", doc.Status, "final verdict PASS must approve the spec")
}
