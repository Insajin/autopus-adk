package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestRunSpecReviewLoop_ConvergesOnDegradedPassWithOnlyAdvisoryFindings(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-ADVISORY-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{{
				Provider: "claude",
				Output:   `{"verdict":"PASS","summary":"Advisory only","findings":[{"severity":"suggestion","category":"completeness","scope_ref":"SPEC-REVIEW-ADVISORY-001","location":"SPEC-REVIEW-ADVISORY-001","description":"Consider adding a small example.","suggestion":"Optional wording improvement."}]}`,
			}},
			FailedProviders: []orchestra.FailedProvider{
				{Name: "codex", FailureClass: "timeout", Error: "timed out"},
				{Name: "gemini", FailureClass: "execution_error", Error: "empty output"},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, spec.VerdictPass, result.Verdict)
	assert.Equal(t, 1, callCount, "advisory-only feedback must not force another review iteration")

	findings, err := spec.LoadFindings(specDir)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, spec.FindingStatusDeferred, findings[0].Status)
	assert.False(t, spec.IsActiveBlockingFinding(findings[0]))
}

func TestRunSpecReviewLoop_PromotesReviseWithOnlyAdvisoryFindings(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-SUGGESTION-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{
					Provider: "claude",
					Output:   `{"verdict":"REVISE","summary":"Only advisory feedback","findings":[{"severity":"suggestion","category":"style","scope_ref":"plan.md","location":"plan.md","description":"Consider tightening the wording.","suggestion":"Optional wording improvement."}]}`,
				},
				{Provider: "gemini", Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, spec.VerdictPass, result.Verdict)
}

func TestRunSpecReviewLoop_StopsAfterProviderOnlyFailures(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-PROVIDER-FAIL-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{
					Provider: "claude",
					Output:   `{"verdict":"REVISE","summary":"provider failed","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:claude","location":"provider:claude","description":"provider timed out","suggestion":"Retry provider transport."}]}`,
				},
				{
					Provider: "codex",
					Output:   `{"verdict":"REVISE","summary":"provider failed","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:codex","location":"provider:codex","description":"provider returned empty output","suggestion":"Retry provider transport."}]}`,
				},
				{
					Provider: "gemini",
					Output:   `{"verdict":"REVISE","summary":"provider failed","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:gemini","location":"provider:gemini","description":"provider timed out","suggestion":"Retry provider transport."}]}`,
				},
			},
			FailedProviders: []orchestra.FailedProvider{
				{Name: "claude", FailureClass: "timeout", Error: "timed out"},
				{Name: "codex", FailureClass: "empty_output", Error: "empty output"},
				{Name: "gemini", FailureClass: "timeout", Error: "timed out"},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, spec.VerdictRevise, result.Verdict)
	assert.Equal(t, 1, callCount, "provider-only failures must not burn every revision timeout")
}

func TestRunSpecReviewLoop_AppliesRevisionWatchdogToOrchestra(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-WATCHDOG-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	callCount := 0
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(ctx context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		callCount++
		<-ctx.Done()
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{{
				Provider: "claude",
				TimedOut: true,
				Output:   `{"verdict":"REVISE","summary":"provider timed out","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:claude","location":"provider:claude","description":"provider timed out","suggestion":"Retry with a shorter context."}]}`,
			}},
			FailedProviders: []orchestra.FailedProvider{{
				Name:         "claude",
				FailureClass: "timeout",
				Error:        ctx.Err().Error(),
			}},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	params := reviewLoopParams(specID, specDir)
	params.timeout = 1
	params.maxRevisions = 3
	params.providers = []orchestra.ProviderConfig{{Name: "claude", Binary: "claude"}}

	start := time.Now()
	result, err := runSpecReviewLoop(params, doc, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Less(t, time.Since(start), 2*time.Second, "revision watchdog must bound orchestra execution")
	assert.Equal(t, spec.VerdictRevise, result.Verdict)
	assert.Equal(t, 1, callCount, "watchdog provider failure must not burn every revision")
}

func TestRunSpecReviewLoop_UsesVerifyModeWhenPriorFindingsExistAtRevisionZero(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-VERIFY-BOOTSTRAP-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	priorFindings := []spec.ReviewFinding{{
		ID:          "F-001",
		Severity:    "major",
		Category:    spec.FindingCategoryCorrectness,
		ScopeRef:    "REQ-001",
		Description: "Existing open finding must be verified, not rediscovered.",
		Status:      spec.FindingStatusOpen,
	}}

	var capturedPrompt string
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		capturedPrompt = cfg.Prompt
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: `{"verdict":"PASS","summary":"fixed","finding_statuses":[{"id":"F-001","status":"resolved","reason":"closed"}],"findings":[]}`},
				{Provider: "codex", Output: `{"verdict":"PASS","summary":"fixed","finding_statuses":[{"id":"F-001","status":"resolved","reason":"closed"}],"findings":[]}`},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, priorFindings)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, spec.VerdictPass, result.Verdict)
	assert.Contains(t, capturedPrompt, "Instructions (Verify Mode)")
	assert.Contains(t, capturedPrompt, "Prior Findings Checklist")
	assert.NotContains(t, capturedPrompt, "Review the SPEC and respond with:")
}

func TestRunSpecReviewLoop_VerifyStatusRequiresProviderAgreement(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-REVIEW-VERIFY-MERGE-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	priorFindings := []spec.ReviewFinding{{
		ID:          "F-001",
		Severity:    "major",
		Category:    spec.FindingCategoryCorrectness,
		ScopeRef:    "REQ-001",
		Description: "Existing blocker",
		Status:      spec.FindingStatusOpen,
	}}

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{
			Responses: []orchestra.ProviderResponse{
				{Provider: "claude", Output: `{"verdict":"PASS","summary":"fixed","finding_statuses":[{"id":"F-001","status":"resolved","reason":"closed"}],"findings":[]}`},
				{Provider: "codex", Output: `{"verdict":"PASS","summary":"not fixed","finding_statuses":[{"id":"F-001","status":"open","reason":"still open"}],"findings":[]}`},
			},
		}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	result, err := runSpecReviewLoop(reviewLoopParams(specID, specDir), doc, priorFindings)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, spec.FindingStatusOpen, result.Findings[0].Status)
	assert.Equal(t, spec.VerdictRevise, result.Verdict)
}

func reviewLoopParams(specID, specDir string) specReviewLoopParams {
	return specReviewLoopParams{
		ctx:          context.Background(),
		specID:       specID,
		specDir:      specDir,
		strategy:     "debate",
		timeout:      10,
		maxRevisions: 3,
		threshold:    0.67,
		gate:         config.ReviewGateConf{},
		providers: []orchestra.ProviderConfig{
			{Name: "claude", Binary: "claude"},
			{Name: "codex", Binary: "codex"},
			{Name: "gemini", Binary: "gemini"},
		},
	}
}
