package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
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

// runSpecReview executes the full SPEC review pipeline with REVISE loop.
func runSpecReview(ctx context.Context, specID, strategy string, timeout int) error {
	resolved, err := spec.ResolveSpecDir(".", specID)
	if err != nil {
		return fmt.Errorf("SPEC 로드 실패: %w", err)
	}
	specDir := resolved.SpecDir

	doc, err := spec.Load(specDir)
	if err != nil {
		// A load failure often means the spec.md has no parseable ID or is structurally empty.
		return fmt.Errorf("SPEC 본문이 비어있습니다: %s (%w)", specID, err)
	}

	// REQ-05b: guard against empty spec body before entering the loop.
	if doc.RawContent == "" {
		return fmt.Errorf("SPEC 본문이 비어있습니다: %s", specID)
	}

	flags := globalFlagsFromContext(ctx)

	configDir, err := resolveConfigDir(nil, flags.ConfigPath)
	if err != nil {
		return fmt.Errorf("설정 경로 확인 실패: %w", err)
	}

	cfg, err := config.Load(configDir)
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
	maxRevisions := gate.MaxRevisions
	if maxRevisions <= 0 {
		maxRevisions = defaultMaxRevisions
	}

	threshold := gate.VerdictThreshold
	if threshold <= 0 {
		threshold = 0.67
	}

	providerNames := resolveSpecReviewProviderNames(cfg, flags.MultiMode)
	providers := configureSpecReviewProviders(specReviewConfigProviders(cfg, providerNames))
	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다. 설치를 확인하세요: %v", providerNames)
	}
	if flags.MultiMode && len(providers) < 2 {
		return fmt.Errorf("--multi requires at least 2 installed providers (resolved: %v)", providerNames)
	}

	// Collect code context once. Limit is derived adaptively from the number of
	// files cited in the SPEC, with optional frontmatter override and config ceiling.
	var codeContext string
	if gate.AutoCollectContext {
		_, applied, _, _ := resolveSpecReviewContextLimit(".", specDir, gate.ContextMaxLines, os.Stderr)
		var ctxErr error
		codeContext, ctxErr = spec.CollectContextForSpec(".", specDir, applied)
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "경고: 코드 컨텍스트 수집 실패: %v\n", ctxErr)
		}
	}

	// Load any prior findings (from a previous interrupted run)
	priorFindings, _ := spec.LoadFindings(specDir)

	loopParams := specReviewLoopParams{
		ctx:          ctx,
		specID:       specID,
		specDir:      specDir,
		strategy:     strategy,
		timeout:      timeout,
		maxRevisions: maxRevisions,
		threshold:    threshold,
		gate:         gate,
		providers:    providers,
		codeContext:  codeContext,
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
		if f.Status == spec.FindingStatusOpen || f.Status == spec.FindingStatusRegressed {
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

func mergeProviderNames(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string

	for _, group := range groups {
		for _, name := range group {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			merged = append(merged, name)
		}
	}

	return merged
}

func mergeStringValues(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string

	for _, group := range groups {
		for _, value := range group {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			merged = append(merged, value)
		}
	}

	return merged
}

func sortedProviderKeys(providers map[string]config.ProviderEntry) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveSpecReviewTimeout(cfg *config.HarnessConfig, requested int) int {
	if requested > 0 {
		return requested
	}
	if cfg != nil && cfg.Orchestra.TimeoutSeconds > 0 {
		return cfg.Orchestra.TimeoutSeconds
	}
	return 120
}
