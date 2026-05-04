package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

// specReviewLoopParams holds all parameters needed by the revision loop.
type specReviewLoopParams struct {
	ctx          context.Context
	specID       string
	specDir      string
	strategy     string
	timeout      int
	maxRevisions int
	threshold    float64
	gate         config.ReviewGateConf
	providers    []orchestra.ProviderConfig
	codeContext  string
}

// runSpecReviewLoop executes the REVISE loop and returns the final merged result.
func runSpecReviewLoop(p specReviewLoopParams, doc *spec.SpecDocument, priorFindings []spec.ReviewFinding) (*spec.ReviewResult, error) {
	var finalResult *spec.ReviewResult

	for revision := 0; revision <= p.maxRevisions; revision++ {
		// REQ-02: reload spec on each revision so external edits are picked up.
		if revision > 0 {
			reloaded, err := spec.Load(p.specDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "경고: spec 재로드 실패 (이전 버전 유지): %v\n", err)
			} else {
				doc = reloaded
			}
		}

		opts := buildPromptOpts(priorFindings, revision, p.specDir, p.gate)
		prompt := spec.BuildReviewPrompt(doc, p.codeContext, opts) //nolint:govet

		orchCfg := orchestra.OrchestraConfig{
			Providers:      p.providers,
			Strategy:       orchestra.Strategy(p.strategy),
			Prompt:         prompt,
			TimeoutSeconds: p.timeout,
			JudgeProvider:  p.gate.Judge,
			NoJudge:        true,
		}

		fmt.Fprintf(os.Stderr, "SPEC 리뷰 시작: %s (전략: %s, 리비전: %d)\n", p.specID, p.strategy, revision)

		result, err := specReviewRunOrchestra(p.ctx, orchCfg)
		if err != nil {
			return nil, fmt.Errorf("리뷰 실행 실패: %w", err)
		}

		// Parse verdicts from each provider
		var reviews []spec.ReviewResult
		for _, resp := range result.Responses {
			r := spec.ParseVerdict(p.specID, resp.Output, resp.Provider, revision, nilIfEmpty(priorFindings))
			reviews = append(reviews, r)
		}

		// SPEC-SPECREV-001 REQ-VERD-1: build per-provider health from orchestra.
		configuredNames := make([]string, 0, len(p.providers))
		for _, pc := range p.providers {
			configuredNames = append(configuredNames, pc.Name)
		}
		providerStatuses := spec.BuildProviderStatuses(result.Responses, result.FailedProviders, configuredNames)
		failedCount := len(configuredNames) - spec.CountProviderStatus(providerStatuses, "success")

		// REQ-01: use supermajority threshold instead of unanimous PASS.
		// SPEC-SPECREV-001 REQ-VERD-3: optionally drop failed providers from the denom.
		finalVerdict := spec.MergeVerdictsWithDenomMode(
			reviews, p.threshold, len(p.providers),
			p.gate.ExcludeFailedFromDenom, failedCount,
		)

		// Flatten all provider findings.
		var allFindings []spec.ReviewFinding
		var allChecklistOutcomes []spec.ChecklistOutcome
		var allResponses []string
		for _, r := range reviews {
			allFindings = append(allFindings, r.Findings...)
			allChecklistOutcomes = append(allChecklistOutcomes, r.ChecklistOutcomes...)
			allResponses = append(allResponses, r.Responses...)
		}

		// REQ-06: MergeSupermajority then DeduplicateFindings.
		allFindings = spec.MergeSupermajority(allFindings, len(reviews), p.threshold)
		allFindings = spec.DeduplicateFindings(allFindings)

		merged := &spec.ReviewResult{
			SpecID:            p.specID,
			Verdict:           finalVerdict,
			Findings:          allFindings,
			ChecklistOutcomes: allChecklistOutcomes,
			Responses:         allResponses,
			Revision:          revision,
			ProviderStatuses:  providerStatuses,
		}

		// Apply scope lock in verify mode
		if revision > 0 {
			merged.Findings = spec.ApplyScopeLock(merged.Findings, priorFindings, spec.ReviewModeVerify)
		}

		// A mid-pipeline write failure must abort (issue #38).
		if persistErr := spec.PersistFindings(p.specDir, merged.Findings); persistErr != nil {
			return nil, fmt.Errorf("review findings 저장 실패 (SPEC: %s, revision: %d): %w", p.specID, revision, persistErr)
		}
		if persistErr := spec.PersistReview(p.specDir, merged); persistErr != nil {
			return nil, fmt.Errorf("review.md 저장 실패 (SPEC: %s, revision: %d): %w", p.specID, revision, persistErr)
		}

		finalResult = merged

		// PASS: no open or regressed findings
		if finalVerdict == spec.VerdictPass && !hasActiveFindings(merged.Findings) {
			break
		}

		// Circuit breaker: halt if no progress
		if revision > 0 && spec.ShouldTripCircuitBreaker(priorFindings, merged.Findings) {
			fmt.Fprintf(os.Stderr, "경고: 서킷 브레이커 작동 — 진행 없음, 리뷰 중단\n")
			break
		}

		// Max revisions reached
		if revision >= p.maxRevisions {
			fmt.Fprintf(os.Stderr, "경고: 최대 리비전 (%d) 도달\n", p.maxRevisions)
			break
		}

		priorFindings = merged.Findings
	}

	return finalResult, nil
}

// buildPromptOpts builds ReviewPromptOptions for the current revision.
func buildPromptOpts(priorFindings []spec.ReviewFinding, revision int, specDir string, gate config.ReviewGateConf) spec.ReviewPromptOptions {
	opts := spec.ReviewPromptOptions{
		SpecDir:            specDir,
		PassCriteria:       gate.PassCriteria,
		DocContextMaxLines: gate.DocContextMaxLines,
	}
	if revision == 0 || len(priorFindings) == 0 {
		opts.Mode = spec.ReviewModeDiscover
		return opts
	}
	opts.Mode = spec.ReviewModeVerify
	opts.PriorFindings = priorFindings
	return opts
}
