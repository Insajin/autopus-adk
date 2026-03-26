package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// newOrchestraCmd creates the orchestra root command.
// @AX:ANCHOR: [AUTO] CLI entry point — registers all 7 orchestra subcommands; changes here affect all orchestra routes
func newOrchestraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestra",
		Short: "다중 모델 오케스트레이션으로 코드를 분석한다",
		Long: `orchestra는 여러 코딩 CLI를 동시에 실행하여 합의, 파이프라인,
토론, 최속 전략으로 결과를 병합하는 다중 모델 오케스트레이션 엔진입니다.`,
	}

	cmd.AddCommand(newOrchestraReviewCmd())
	cmd.AddCommand(newOrchestraPlanCmd())
	cmd.AddCommand(newOrchestraSecureCmd())
	cmd.AddCommand(newOrchestraBrainstormCmd())
	cmd.AddCommand(newOrchestraJobStatusCmd())
	cmd.AddCommand(newOrchestraJobWaitCmd())
	cmd.AddCommand(newOrchestraJobResultCmd())

	return cmd
}

// newOrchestraReviewCmd creates the code review subcommand.
func newOrchestraReviewCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		judge     string
		noDetach  bool
	)

	cmd := &cobra.Command{
		Use:   "review [files...]",
		Short: "여러 모델로 코드를 리뷰한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일을 리뷰하고 결과를 병합합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pass only explicitly set flags; empty string means "use config"
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			prompt := buildReviewPrompt(args)
			return runOrchestraCommand(cmd.Context(), "review", flagStrategy, flagProviders, timeout, judge, prompt, noDetach, keepRelay)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().StringVar(&judge, "judge", "", "debate 전략에서 최종 판정 프로바이더")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}

// newOrchestraPlanCmd creates the plan subcommand.
func newOrchestraPlanCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
	)

	cmd := &cobra.Command{
		Use:   "plan \"description\"",
		Short: "여러 모델로 구현 계획을 수립한다",
		Long:  "여러 코딩 CLI를 사용하여 기능 구현 계획을 합의 방식으로 수립합니다.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			prompt := fmt.Sprintf("다음 기능 구현 계획을 수립해주세요:\n\n%s", args[0])
			return runOrchestraCommand(cmd.Context(), "plan", flagStrategy, flagProviders, timeout, "", prompt, false, keepRelay)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}

// newOrchestraSecureCmd creates the security analysis subcommand.
func newOrchestraSecureCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
	)

	cmd := &cobra.Command{
		Use:   "secure [files...]",
		Short: "여러 모델로 보안 취약점을 분석한다",
		Long:  "여러 코딩 CLI를 사용하여 지정된 파일의 보안 취약점을 분석합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			flagStrategy := flagStringIfChanged(cmd, "strategy", strategy)
			flagProviders := flagStringSliceIfChanged(cmd, "providers", providers)
			keepRelay, _ := cmd.Flags().GetBool("keep-relay-output")
			prompt := buildSecurePrompt(args)
			return runOrchestraCommand(cmd.Context(), "secure", flagStrategy, flagProviders, timeout, "", prompt, false, keepRelay)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}

// runOrchestraCommand resolves config and runs the orchestration.
// It loads autopus.yaml first, resolves strategy and providers via config,
// and falls back to buildProviderConfigs when config is unavailable.
// @AX:ANCHOR: [AUTO] fan_in=4 CLI callers (review, plan, secure, brainstorm); variadic boolFlags order is load-bearing
func runOrchestraCommand(
	ctx context.Context,
	commandName string,
	flagStrategy string,
	flagProviders []string,
	timeout int,
	judge string,
	prompt string,
	boolFlags ...bool,
) error {
	// @AX:NOTE [AUTO] REQ-11 opportunistic GC — fires on every orchestra invocation; 1h TTL
	_, _ = orchestra.CleanupStaleJobs(os.TempDir(), 1*time.Hour)

	// Attempt to load config; fall back to hardcoded defaults on failure
	orchConf, configErr := loadOrchestraConfig()

	var (
		strategyStr string
		providers   []orchestra.ProviderConfig
	)

	if configErr != nil || orchConf == nil {
		// Config load failed: use CLI flags directly or hardcoded defaults
		strategyStr = flagStrategy
		if strategyStr == "" {
			strategyStr = "consensus"
		}
		names := flagProviders
		if len(names) == 0 {
			names = defaultProviders()
		}
		providers = buildProviderConfigs(names)
	} else {
		// Config loaded: resolve strategy, providers, and judge with priority
		strategyStr = resolveStrategy(orchConf, commandName, flagStrategy)
		providers = resolveProviders(orchConf, commandName, flagProviders)
		// Resolve judge from config when not explicitly set via CLI flag
		if judge == "" {
			judge = resolveJudge(orchConf, commandName, "")
		}
	}

	s := orchestra.Strategy(strategyStr)
	if !s.IsValid() {
		return fmt.Errorf("유효하지 않은 전략: %q (가능한 값: consensus, pipeline, debate, fastest, relay)", strategyStr)
	}

	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다")
	}

	// @AX:WARN: [AUTO] positional variadic bool extraction — boolFlags[0]=noDetach, boolFlags[1]=keepRelay; order must match all callers
	nd := len(boolFlags) > 0 && boolFlags[0]
	keepRelay := len(boolFlags) > 1 && boolFlags[1]
	cfg := orchestra.OrchestraConfig{
		Providers:       providers,
		Strategy:        s,
		Prompt:          prompt,
		TimeoutSeconds:  timeout,
		JudgeProvider:   judge,
		Terminal:        terminal.DetectTerminal(),
		NoDetach:        nd,
		KeepRelayOutput: keepRelay,
	}

	providerNames := make([]string, len(providers))
	for i, p := range providers {
		providerNames[i] = p.Name
	}
	fmt.Fprintf(os.Stderr, "전략: %s, 프로바이더: %s\n", strategyStr, strings.Join(providerNames, ", "))

	// @AX:NOTE [AUTO] REQ-1 auto-detach branch — returns job ID to stdout, status to stderr; skips RunOrchestra
	termName := ""
	if cfg.Terminal != nil {
		termName = cfg.Terminal.Name()
	}
	if orchestra.ShouldDetach(termName, isStdoutTTY(), cfg.NoDetach) {
		jobID, err := orchestra.RunPaneOrchestraDetached(ctx, cfg)
		if err != nil {
			return fmt.Errorf("detach mode failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Detached: job %s\n", jobID)
		fmt.Printf("%s\n", jobID)
		return nil
	}

	result, err := orchestra.RunOrchestra(ctx, cfg)
	if err != nil {
		return fmt.Errorf("오케스트레이션 실패: %w", err)
	}

	fmt.Printf("%s\n", result.Merged)
	fmt.Fprintf(os.Stderr, "\n요약: %s (총 %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

// buildProviderConfigs converts provider names to ProviderConfig slice.
// This is the hardcoded fallback used when config is unavailable.
// @AX:NOTE: [AUTO] hardcoded provider registry — add new providers here and in agenticArgs when expanding provider support
func buildProviderConfigs(names []string) []orchestra.ProviderConfig {
	// Known provider mappings: binary + default args
	knownProviders := map[string]orchestra.ProviderConfig{
		// claude: non-interactive mode via stdin
		"claude": {Name: "claude", Binary: "claude", Args: []string{"-p"}, PaneArgs: []string{"-p"}, PromptViaArgs: false},
		// codex: quiet mode via stdin
		"codex": {Name: "codex", Binary: "codex", Args: []string{"-q"}, PaneArgs: []string{"-q"}, PromptViaArgs: false},
		// gemini: prompt passed as last argument (not stdin)
		"gemini": {Name: "gemini", Binary: "gemini", Args: []string{"-p"}, PaneArgs: []string{"-p"}, PromptViaArgs: true},
	}

	var result []orchestra.ProviderConfig
	for _, name := range names {
		if p, ok := knownProviders[name]; ok {
			result = append(result, p)
		} else {
			// Unknown provider: use name as binary
			result = append(result, orchestra.ProviderConfig{
				Name:   name,
				Binary: name,
				Args:   []string{},
			})
		}
	}
	return result
}

// defaultProviders returns the hardcoded default provider list.
func defaultProviders() []string {
	return []string{"claude", "codex", "gemini"}
}

