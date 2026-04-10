package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

const defaultMaxRevisions = 3

// newSpecReviewCmd creates the "spec review" subcommand.
func newSpecReviewCmd() *cobra.Command {
	var (
		strategy string
		timeout  int
	)

	cmd := &cobra.Command{
		Use:   "review <SPEC-ID>",
		Short: "Run multi-provider review on a SPEC document",
		Long:  "Execute a multi-provider review gate using the orchestra engine to validate a SPEC document.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]
			return runSpecReview(cmd.Context(), specID, strategy, timeout)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "review strategy (default: from config)")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 0, "timeout in seconds (default: from config)")

	return cmd
}

// runSpecReview executes the full SPEC review pipeline with REVISE loop.
func runSpecReview(ctx context.Context, specID, strategy string, timeout int) error {
	resolved, err := spec.ResolveSpecDir(".", specID)
	if err != nil {
		return fmt.Errorf("SPEC 로드 실패: %w", err)
	}
	specDir := resolved.SpecDir

	doc, err := spec.Load(specDir)
	if err != nil {
		return fmt.Errorf("SPEC 로드 실패: %w", err)
	}

	cfg, err := config.Load(".")
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	gate := cfg.Spec.ReviewGate
	if strategy == "" {
		strategy = gate.Strategy
	}
	if timeout <= 0 {
		timeout = 120
	}
	maxRevisions := gate.MaxRevisions
	if maxRevisions <= 0 {
		maxRevisions = defaultMaxRevisions
	}

	providers := buildReviewProviders(gate.Providers)
	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다. 설치를 확인하세요: %v", gate.Providers)
	}

	// Collect code context once
	var codeContext string
	if gate.AutoCollectContext {
		var ctxErr error
		codeContext, ctxErr = spec.CollectContextForSpec(".", specDir, gate.ContextMaxLines)
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "경고: 코드 컨텍스트 수집 실패: %v\n", ctxErr)
		}
	}

	// Load any prior findings (from a previous interrupted run)
	priorFindings, _ := spec.LoadFindings(specDir)

	var finalResult *spec.ReviewResult

	for revision := 0; revision <= maxRevisions; revision++ {
		opts := buildPromptOpts(priorFindings, revision)
		prompt := spec.BuildReviewPrompt(doc, codeContext, opts)

		orchCfg := orchestra.OrchestraConfig{
			Providers:      providers,
			Strategy:       orchestra.Strategy(strategy),
			Prompt:         prompt,
			TimeoutSeconds: timeout,
			JudgeProvider:  gate.Judge,
		}

		fmt.Fprintf(os.Stderr, "SPEC 리뷰 시작: %s (전략: %s, 리비전: %d)\n", specID, strategy, revision)

		result, err := orchestra.RunOrchestra(ctx, orchCfg)
		if err != nil {
			return fmt.Errorf("리뷰 실행 실패: %w", err)
		}

		// Parse verdicts from each provider
		var reviews []spec.ReviewResult
		for _, resp := range result.Responses {
			r := spec.ParseVerdict(specID, resp.Output, resp.Provider, revision, nilIfEmpty(priorFindings))
			reviews = append(reviews, r)
		}

		finalVerdict := spec.MergeVerdicts(reviews)

		// Aggregate findings across providers
		merged := &spec.ReviewResult{
			SpecID:   specID,
			Verdict:  finalVerdict,
			Revision: revision,
		}
		for _, r := range reviews {
			merged.Findings = append(merged.Findings, r.Findings...)
			merged.Responses = append(merged.Responses, r.Responses...)
		}

		// Apply scope lock in verify mode
		if revision > 0 {
			merged.Findings = spec.ApplyScopeLock(merged.Findings, priorFindings, spec.ReviewModeVerify)
		}

		// Persist findings and review
		if persistErr := spec.PersistFindings(specDir, merged.Findings); persistErr != nil {
			fmt.Fprintf(os.Stderr, "findings 저장 실패: %v\n", persistErr)
		}
		if persistErr := spec.PersistReview(specDir, merged); persistErr != nil {
			fmt.Fprintf(os.Stderr, "review.md 저장 실패: %v\n", persistErr)
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
		if revision >= maxRevisions {
			fmt.Fprintf(os.Stderr, "경고: 최대 리비전 (%d) 도달\n", maxRevisions)
			break
		}

		priorFindings = merged.Findings
	}

	// Output final result
	if finalResult != nil {
		fmt.Printf("SPEC 리뷰 완료: %s\n", specID)
		fmt.Printf("판정: %s\n", finalResult.Verdict)
		if len(finalResult.Findings) > 0 {
			fmt.Printf("발견 사항: %d건\n", len(finalResult.Findings))
		}
	}

	return nil
}

// buildPromptOpts builds ReviewPromptOptions based on the current revision.
func buildPromptOpts(priorFindings []spec.ReviewFinding, revision int) spec.ReviewPromptOptions {
	if revision == 0 || len(priorFindings) == 0 {
		return spec.ReviewPromptOptions{Mode: spec.ReviewModeDiscover}
	}
	return spec.ReviewPromptOptions{
		Mode:          spec.ReviewModeVerify,
		PriorFindings: priorFindings,
	}
}

// nilIfEmpty returns nil if the slice is empty, otherwise returns the slice.
func nilIfEmpty(findings []spec.ReviewFinding) []spec.ReviewFinding {
	if len(findings) == 0 {
		return nil
	}
	return findings
}

// hasActiveFindings returns true if there are any open or regressed findings.
func hasActiveFindings(findings []spec.ReviewFinding) bool {
	for _, f := range findings {
		if f.Status == spec.FindingStatusOpen || f.Status == spec.FindingStatusRegressed {
			return true
		}
	}
	return false
}

// buildReviewProviders builds provider configs, skipping missing binaries.
func buildReviewProviders(names []string) []orchestra.ProviderConfig {
	all := buildProviderConfigs(names)
	var available []orchestra.ProviderConfig
	for _, p := range all {
		if detect.IsInstalled(p.Binary) {
			available = append(available, p)
		} else {
			fmt.Fprintf(os.Stderr, "경고: %s 바이너리를 찾을 수 없습니다 (건너뜀)\n", p.Binary)
		}
	}
	return available
}
