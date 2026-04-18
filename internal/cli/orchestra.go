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
	cmd.AddCommand(newOrchestraCollectCmd())
	cmd.AddCommand(newOrchestraCleanupCmd())
	cmd.AddCommand(newOrchestraInjectCmd())
	cmd.AddCommand(newOrchestraRunCmd())

	return cmd
}

// newOrchestraReviewCmd and newOrchestraSecureCmd live in orchestra_file_cmds.go.

// newOrchestraPlanCmd creates the plan subcommand.
func newOrchestraPlanCmd() *cobra.Command {
	var (
		strategy  string
		providers []string
		timeout   int
		rounds    int
		noDetach  bool
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
			thresholdFlag, _ := cmd.Flags().GetFloat64("threshold")
			resolvedRounds := resolveRounds(flagStrategy, rounds)
			prompt := args[0]
			return runOrchestraCommand(cmd.Context(), "plan", flagStrategy, flagProviders, timeout, "", prompt, resolvedRounds, thresholdFlag, OrchestraFlags{NoDetach: noDetach, KeepRelay: keepRelay})
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "오케스트레이션 전략 (consensus|pipeline|debate|fastest|relay)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "사용할 프로바이더 목록")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "타임아웃 (초)")
	cmd.Flags().Float64("threshold", 0, "consensus 전략 합의 임계값 (0.0-1.0)")
	cmd.Flags().IntVar(&rounds, "rounds", 0, "debate 라운드 수 (1-10, debate 전략 전용)")
	cmd.Flags().BoolVar(&noDetach, "no-detach", false, "Disable auto-detach mode")
	cmd.Flags().Bool("keep-relay-output", false, "relay 전략 실행 후 임시 파일 보존")

	return cmd
}

// runOrchestraCommand resolves config and runs the orchestration.
// It loads autopus.yaml first, resolves strategy and providers via config,
// and falls back to buildProviderConfigs when config is unavailable.
// @AX:ANCHOR: [AUTO] fan_in=4 CLI callers (review, plan, secure, brainstorm); rounds param added for debate strategy
func runOrchestraCommand(
	ctx context.Context,
	commandName string,
	flagStrategy string,
	flagProviders []string,
	timeout int,
	judge string,
	prompt string,
	rounds int,
	threshold float64,
	flags OrchestraFlags,
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

	resolvedThreshold, err := resolveAndValidateThreshold(orchConf, configErr, commandName, threshold)
	if err != nil {
		return err
	}

	s := orchestra.Strategy(strategyStr)
	if !s.IsValid() {
		return fmt.Errorf("유효하지 않은 전략: %q (가능한 값: consensus, pipeline, debate, fastest, relay)", strategyStr)
	}

	if len(providers) == 0 {
		return fmt.Errorf("사용 가능한 프로바이더가 없습니다")
	}

	// Validate --rounds: must be 1-10 and only with debate strategy
	if rounds > 0 && s != orchestra.StrategyDebate {
		return fmt.Errorf("--rounds는 debate 전략에서만 사용할 수 있습니다")
	}
	if rounds > 10 {
		return fmt.Errorf("--rounds 값은 1-10 범위여야 합니다 (입력: %d)", rounds)
	}

	nd := flags.NoDetach
	keepRelay := flags.KeepRelay
	noJudge := flags.NoJudge
	yieldRounds := flags.YieldRounds
	contextAware := flags.ContextAware
	subprocessMode := flags.SubprocessMode
	term := terminal.DetectTerminal()
	// Auto-enable interactive pane mode for cmux/tmux terminals (SPEC-ORCH-006)
	interactive := term != nil && term.Name() != "plain"

	// Hook mode requires `auto init` to install hooks first (SPEC-ORCH-007 R5/R6).
	hookMode := false
	sessionID := ""
	if interactive {
		hookMode = isHookModeAvailable()
		if hookMode {
			sessionID = fmt.Sprintf("orch-%d", time.Now().UnixMilli())
		}
	}

	cfg := orchestra.OrchestraConfig{
		Providers:          providers,
		Strategy:           s,
		Prompt:             prompt,
		TimeoutSeconds:     timeout,
		JudgeProvider:      judge,
		DebateRounds:       rounds,
		ConsensusThreshold: resolvedThreshold,
		Terminal:           term,
		NoDetach:           nd,
		KeepRelayOutput:    keepRelay,
		Interactive:        interactive,
		HookMode:           hookMode,
		SessionID:          sessionID,
		NoJudge:            noJudge,
		YieldRounds:        yieldRounds,
		ContextAware:       contextAware,
		SubprocessMode:     subprocessMode,
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

	// R9: --no-judge outputs structured JSON when round history is available.
	if noJudge && len(result.RoundHistory) > 0 {
		yieldOut := orchestra.BuildYieldOutputFromResult(result, sessionID)
		if writeErr := orchestra.WriteYieldOutput(os.Stdout, yieldOut); writeErr != nil {
			return fmt.Errorf("write JSON output: %w", writeErr)
		}
	} else {
		fmt.Printf("%s\n", result.Merged)
		if path, saveErr := saveOrchestraResult(commandName, strategyStr, providerNames, result); saveErr == nil {
			fmt.Fprintf(os.Stderr, "결과 저장: %s\n", path)
		}
	}
	fmt.Fprintf(os.Stderr, "\n요약: %s (총 %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}
