package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

const (
	defaultMaxRevisions        = 3
	specReviewResultReadyGrace = 5 * time.Second
	// loopModeMinRevisions is the floor applied to the revision budget when the
	// global --loop flag is set (SPEC-SPECREV-002 REQ-003). It is intentionally
	// larger than defaultMaxRevisions so the effect is observable under default
	// config; the circuit breaker still terminates early when no progress is made.
	loopModeMinRevisions = 5
)

// loopAwareMaxRevisions applies the --loop floor to a configured revision
// budget. When loopMode is set, the result is at least loopModeMinRevisions;
// otherwise the configured value is returned unchanged (floor semantics).
func loopAwareMaxRevisions(configured int, loopMode bool) int {
	if loopMode && configured < loopModeMinRevisions {
		return loopModeMinRevisions
	}
	return configured
}

// resolveSpecReviewMaxRevisions derives the effective revision budget from the
// review gate config and the --loop flag. A non-positive gate.MaxRevisions
// falls back to defaultMaxRevisions before the loop floor is applied.
func resolveSpecReviewMaxRevisions(gate config.ReviewGateConf, loopMode bool) int {
	configured := gate.MaxRevisions
	if configured <= 0 {
		configured = defaultMaxRevisions
	}
	return loopAwareMaxRevisions(configured, loopMode)
}

// wrapSpecLoadError wraps a spec.Load failure with a neutral prefix that names
// the SPEC ID without asserting an empty body (SPEC-SPECREV-002 REQ-005). The
// cause is preserved via %w so errors.Is keeps working.
func wrapSpecLoadError(specID string, err error) error {
	return fmt.Errorf("SPEC 로드 실패 (%s): %w", specID, err)
}

// newSpecReviewCmd creates the "spec review" subcommand.
func newSpecReviewCmd() *cobra.Command {
	var (
		strategy        string
		timeout         int
		forceSubprocess bool
		forcePlain      bool
	)

	cmd := &cobra.Command{
		Use:   "review <SPEC-ID>",
		Short: "Run multi-provider review on a SPEC document",
		Long:  "Execute a multi-provider review gate using the orchestra engine to validate a SPEC document.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]
			return runSpecReviewWithOptions(cmd.Context(), specID, strategy, timeout, specReviewOptions{
				forceSubprocess: forceSubprocess || forcePlain,
			})
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "review strategy (default: from config)")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 0, "timeout in seconds (default: from config)")
	cmd.Flags().BoolVar(&forceSubprocess, "subprocess", false, "Force headless subprocess backend for SPEC review")
	cmd.Flags().BoolVar(&forcePlain, "plain", false, "Alias for --subprocess; bypass interactive pane backend")

	return cmd
}

type specReviewOptions struct {
	forceSubprocess bool
}

// runSpecReview executes the full SPEC review pipeline with REVISE loop.
func runSpecReview(ctx context.Context, specID, strategy string, timeout int) error {
	return runSpecReviewWithOptions(ctx, specID, strategy, timeout, specReviewOptions{})
}

func runSpecReviewWithOptions(ctx context.Context, specID, strategy string, timeout int, opts specReviewOptions) error {
	resolved, err := spec.ResolveSpecDir(".", specID)
	if err != nil {
		return fmt.Errorf("SPEC 로드 실패: %w", err)
	}
	specDir := resolved.SpecDir

	doc, err := spec.Load(specDir)
	if err != nil {
		// The real cause (malformed frontmatter, missing ID header, etc.) is
		// preserved instead of asserting an empty body.
		return wrapSpecLoadError(specID, err)
	}

	// REQ-05b: guard against empty spec body before entering the loop.
	if doc.RawContent == "" {
		return fmt.Errorf("SPEC 본문이 비어있습니다: %s", specID)
	}

	flags := globalFlagsFromContext(ctx)

	cfg, err := loadHarnessConfigForFlags(flags)
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	gate := cfg.Spec.ReviewGate
	if strategy == "" {
		strategy = gate.Strategy
	}
	if strategy == "" && flags.MultiMode {
		strategy = string(orchestra.StrategyDebate)
	}
	timeout = resolveSpecReviewTimeout(cfg, timeout)
	// SPEC-SPECREV-002 REQ-003: consume the global --loop flag so the revision
	// budget honors the loop floor (inert seam otherwise).
	maxRevisions := resolveSpecReviewMaxRevisions(gate, flags.LoopMode)

	threshold := gate.VerdictThreshold
	if threshold <= 0 {
		threshold = 0.67
	}

	providerNames := resolveSpecReviewProviderNames(cfg, flags.MultiMode)
	providers := configureSpecReviewProviders(resolveCodexProviderCapabilities(ctx, specReviewConfigProviders(cfg, providerNames)))
	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다. 설치를 확인하세요: %v", providerNames)
	}
	if flags.MultiMode && len(providers) < 2 {
		fmt.Fprintf(os.Stderr, "경고: --multi review requested but only one provider is installed; falling back to single-provider review (resolved: %v)\n", providerNames)
	}

	// Collect code context once. Limit is derived adaptively from the number of
	// files cited in the SPEC, with optional frontmatter override and config ceiling.
	var codeContext string
	if gate.AutoCollectContext {
		contextCeiling := effectiveSpecReviewContextCeiling(gate.ContextMaxLines)
		_, applied, _, _ := resolveSpecReviewContextLimit(".", specDir, contextCeiling, os.Stderr)
		var ctxErr error
		codeContext, ctxErr = spec.CollectContextForSpec(".", specDir, applied)
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "경고: 코드 컨텍스트 수집 실패: %v\n", ctxErr)
		}
	}

	// Load any prior findings (from a previous interrupted run)
	priorFindings, _ := spec.LoadFindings(specDir)

	loopParams := specReviewLoopParams{
		ctx:            ctx,
		specID:         specID,
		specDir:        specDir,
		strategy:       strategy,
		timeout:        timeout,
		maxRevisions:   maxRevisions,
		threshold:      threshold,
		gate:           gate,
		providers:      providers,
		codeContext:    codeContext,
		subprocessMode: opts.forceSubprocess || resolveSubprocessMode(&cfg.Orchestra),
	}

	finalResult, err := runSpecReviewLoop(loopParams, doc, priorFindings)
	if err != nil {
		return err
	}

	// Output final result
	if finalResult != nil {
		if persistErr := syncReviewedSpecStatus(specDir, finalResult); persistErr != nil {
			return fmt.Errorf("SPEC 상태 업데이트 실패 (SPEC: %s): %w", specID, persistErr)
		}
		fmt.Printf("SPEC 리뷰 완료: %s\n", specID)
		fmt.Printf("판정: %s\n", finalResult.Verdict)
		if len(finalResult.Findings) > 0 {
			// Issue #44: surface status breakdown instead of raw count so operators
			// can tell at a glance whether any findings are still open.
			fmt.Printf("발견 사항: %s\n", spec.SummarizeFindings(finalResult.Findings).Format())
		}
		printChecklistSummary(finalResult.ChecklistOutcomes)
	}

	return nil
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
		if spec.IsActiveBlockingFinding(f) {
			return true
		}
	}
	return false
}

// buildReviewProviders builds provider configs, skipping missing binaries.
func buildReviewProviders(names []string) []orchestra.ProviderConfig {
	all := buildProviderConfigs(names)
	return filterInstalledProviders(all)
}

func buildReviewProvidersWithConfig(cfg *config.HarnessConfig, names []string) []orchestra.ProviderConfig {
	if cfg == nil {
		return buildReviewProviders(names)
	}
	all := resolveProviders(&cfg.Orchestra, "review", names)
	return filterInstalledProviders(all)
}

func filterInstalledProviders(all []orchestra.ProviderConfig) []orchestra.ProviderConfig {
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

func configureSpecReviewProviders(providers []orchestra.ProviderConfig) []orchestra.ProviderConfig {
	configured := make([]orchestra.ProviderConfig, len(providers))
	copy(configured, providers)

	for i := range configured {
		configured[i].ResultReadyPatterns = mergeStringValues(configured[i].ResultReadyPatterns, []string{"VERDICT:"})
		if configured[i].ResultReadyGrace <= 0 {
			configured[i].ResultReadyGrace = specReviewResultReadyGrace
		}
	}

	return configured
}

func resolveSpecReviewProviderNames(cfg *config.HarnessConfig, multi bool) []string {
	if cfg == nil {
		return nil
	}

	names := mergeProviderNames(cfg.Spec.ReviewGate.Providers)
	if !multi {
		return names
	}

	if cmd, ok := cfg.Orchestra.Commands["review"]; ok {
		names = mergeProviderNames(names, cmd.Providers)
	}

	return mergeProviderNames(names, sortedProviderKeys(cfg.Orchestra.Providers), defaultProviders())
}
