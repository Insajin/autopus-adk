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

// runSpecReview executes the full SPEC review pipeline.
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

	// Collect code context
	var codeContext string
	if gate.AutoCollectContext {
		var ctxErr error
		codeContext, ctxErr = spec.CollectContext(".", gate.ContextMaxLines)
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "경고: 코드 컨텍스트 수집 실패: %v\n", ctxErr)
		}
	}

	prompt := spec.BuildReviewPrompt(doc, codeContext)

	// Build provider configs with fallback for missing binaries
	providers := buildReviewProviders(gate.Providers)
	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다. 설치를 확인하세요: %v", gate.Providers)
	}

	orchCfg := orchestra.OrchestraConfig{
		Providers:      providers,
		Strategy:       orchestra.Strategy(strategy),
		Prompt:         prompt,
		TimeoutSeconds: timeout,
		JudgeProvider:  gate.Judge,
	}

	fmt.Fprintf(os.Stderr, "SPEC 리뷰 시작: %s (전략: %s)\n", specID, strategy)

	result, err := orchestra.RunOrchestra(ctx, orchCfg)
	if err != nil {
		return fmt.Errorf("리뷰 실행 실패: %w", err)
	}

	// Parse verdicts from each provider
	var reviews []spec.ReviewResult
	for _, resp := range result.Responses {
		r := spec.ParseVerdict(specID, resp.Output, resp.Provider, 0)
		reviews = append(reviews, r)
	}

	finalVerdict := spec.MergeVerdicts(reviews)

	// Aggregate findings
	merged := &spec.ReviewResult{
		SpecID:  specID,
		Verdict: finalVerdict,
	}
	for _, r := range reviews {
		merged.Findings = append(merged.Findings, r.Findings...)
		merged.Responses = append(merged.Responses, r.Responses...)
	}

	// Persist review
	if err := spec.PersistReview(specDir, merged); err != nil {
		fmt.Fprintf(os.Stderr, "review.md 저장 실패: %v\n", err)
	}

	// Output
	fmt.Printf("SPEC 리뷰 완료: %s\n", specID)
	fmt.Printf("판정: %s\n", finalVerdict)
	if len(merged.Findings) > 0 {
		fmt.Printf("발견 사항: %d건\n", len(merged.Findings))
	}

	return nil
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
