package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

var runOrchestraExecute = orchestra.RunOrchestra

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
	if err := validateOrchestraOutputFormat(flags.OutputFormat); err != nil {
		return err
	}

	// Attempt to load config; fall back to hardcoded defaults on failure.
	runtimeFlags := globalFlagsFromContext(ctx)
	harnessCfg, configErr := loadHarnessConfigForFlags(runtimeFlags)

	var (
		strategyStr string
		orchConf    *config.OrchestraConf
		providers   []orchestra.ProviderConfig
		judgeConfig *orchestra.ProviderConfig
	)

	if configErr != nil || harnessCfg == nil {
		// Config load failed: use CLI flags directly or hardcoded defaults
		strategyStr = flagStrategy
		if strategyStr == "" {
			strategyStr = "consensus"
		}
		names := flagProviders
		if len(names) == 0 {
			names = defaultProviders()
		}
		providers = buildProviderConfigsForRuntime(names, runtimeFlags.Quality, runtimeFlags.Effort)
	} else {
		orchConf = &harnessCfg.Orchestra
		// Config loaded: resolve strategy, providers, and judge with priority
		strategyStr = resolveStrategy(orchConf, commandName, flagStrategy)
		providers = resolveProviders(orchConf, commandName, flagProviders)
		// Resolve judge from config when not explicitly set via CLI flag
		if judge == "" {
			judge = resolveJudge(orchConf, commandName, "")
		}
	}
	providers = resolveCodexProviderCapabilities(ctx, providers)
	initialProviderNames := providerConfigNames(providers)
	requestedProviderNames := append([]string(nil), initialProviderNames...)
	if len(flagProviders) > 0 {
		requestedProviderNames = append([]string(nil), flagProviders...)
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
	providers, riskTierSingleProvider := applyReviewProviderPolicy(
		providers, commandName, flags.RiskTier, flags.RiskInputs, flags.ProvidersExplicit, os.Stderr)

	// Validate --rounds: must be 1-10 and only with debate strategy
	if rounds > 0 && s != orchestra.StrategyDebate {
		return fmt.Errorf("--rounds는 debate 전략에서만 사용할 수 있습니다")
	}
	if rounds > 10 {
		return fmt.Errorf("--rounds 값은 1-10 범위여야 합니다 (입력: %d)", rounds)
	}
	if commandName == "brainstorm" && s == orchestra.StrategyDebate {
		if flags.NoJudge {
			return fmt.Errorf("brainstorm debate: a separate different-family judge is required")
		}
		originalProviders := append([]orchestra.ProviderConfig(nil), providers...)
		var separationErr error
		var judgeFamily string
		providers, judgeFamily, separationErr = separateBrainstormJudge(providers, judge)
		if separationErr != nil {
			return separationErr
		}
		judgeConfig, separationErr = resolveBrainstormJudgeConfig(
			originalProviders, orchConf, commandName, judge, judgeFamily,
			runtimeFlags.Quality, runtimeFlags.Effort,
		)
		if separationErr != nil {
			return separationErr
		}
	}
	configuredProviderNames := providerConfigNames(providers)

	nd := flags.NoDetach
	keepRelay := flags.KeepRelay
	noJudge := flags.NoJudge || riskTierSingleProvider
	yieldRounds := flags.YieldRounds
	contextAware := flags.ContextAware
	subprocessMode := flags.SubprocessMode
	resolvedTimeout := resolveOrchestraTimeout(orchConf, timeout, flags.TimeoutChanged, providers)
	timeout = resolvedTimeout.Seconds
	providers = applyResolvedProviderTimeouts(providers, resolvedTimeout)
	term := terminal.DetectTerminal()
	// Auto-enable interactive pane mode for cmux/tmux terminals (SPEC-ORCH-006)
	interactive := term != nil && term.Name() != "plain"
	monitorRuntime := resolveCC21MonitorRuntime(term, harnessCfg)
	workingDir, _ := os.Getwd()

	// Hook mode requires `auto init` to install hooks first (SPEC-ORCH-007 R5/R6).
	sessionID := ""
	if interactive && monitorRuntime.HookMode {
		sessionID = orchestra.NewSessionID()
	}

	cfg := orchestra.OrchestraConfig{
		Providers:           providers,
		RequestedProviders:  requestedProviderNames,
		ConfiguredProviders: configuredProviderNames,
		Strategy:            s,
		Prompt:              prompt,
		TimeoutSeconds:      timeout,
		JudgeProvider:       judge,
		JudgeConfig:         judgeConfig,
		DebateRounds:        rounds,
		ConsensusThreshold:  resolvedThreshold,
		MinimumProviders:    reviewRiskMinimumProviders(commandName, flags.RiskTier),
		Terminal:            term,
		NoDetach:            nd,
		KeepRelayOutput:     keepRelay,
		Interactive:         interactive,
		HookMode:            monitorRuntime.HookMode,
		SessionID:           sessionID,
		NoJudge:             noJudge,
		YieldRounds:         yieldRounds,
		ContextAware:        contextAware,
		SubprocessMode:      subprocessMode,
		MonitorEnabled:      monitorRuntime.Enabled,
		MonitorTimeout:      monitorRuntime.PatternTimeout,
		WorkingDir:          workingDir,
		FallbackMode:        flags.FallbackMode,
		RequireJudgeFamilySeparation: commandName == "brainstorm" &&
			s == orchestra.StrategyDebate,
	}

	providerNames := providerConfigNames(providers)
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

	result, err := runOrchestraExecute(ctx, cfg)
	if err == nil && shouldTreatOrchestraResultAsFailure(result) {
		err = synthesizeOrchestraFailureError(result)
	}
	if err != nil {
		if result != nil {
			reportPath, reportErr := saveOrchestraFailureReport(commandName, strategyStr, providerNames, resolvedTimeout, result, err)
			if reportErr != nil {
				fmt.Fprintf(os.Stderr, "실패 보고서 저장 실패: %v\n", reportErr)
			}
			fmt.Fprint(os.Stderr, renderOrchestraFailureSummary(resolvedTimeout, result, reportPath))
		}
		return fmt.Errorf("오케스트레이션 실패: %w", err)
	}

	if flags.OutputFormat == orchestraOutputJSON {
		resultPath, saveErr := saveOrchestraResult(commandName, strategyStr, providerNames, resolvedTimeout, result)
		if saveErr != nil {
			return fmt.Errorf("save orchestra result: %w", saveErr)
		}
		fmt.Fprintf(os.Stderr, "결과 저장: %s\n", resultPath)
		fmt.Fprintf(os.Stderr, "Receipt: %s.receipt.json\n", resultPath)
		if writeErr := writeOrchestraCLIOutput(os.Stdout, result, orchestraOutputJSON); writeErr != nil {
			return fmt.Errorf("write JSON output: %w", writeErr)
		}
		return nil
	}

	structured, writeErr := writeOrchestraPrimaryOutput(os.Stdout, result, noJudge, sessionID)
	if writeErr != nil {
		return fmt.Errorf("write JSON output: %w", writeErr)
	}
	if !structured {
		fmt.Printf("%s\n", result.Merged)
		if path, saveErr := saveOrchestraResult(commandName, strategyStr, providerNames, resolvedTimeout, result); saveErr == nil {
			fmt.Fprintf(os.Stderr, "결과 저장: %s\n", path)
			if result.RunReceipt != nil {
				fmt.Fprintf(os.Stderr, "Receipt: %s.receipt.json\n", path)
			}
		}
	}
	if resultIsDegraded(result) {
		reportPath, reportErr := saveOrchestraDegradedReport(commandName, strategyStr, providerNames, resolvedTimeout, result)
		if reportErr != nil {
			fmt.Fprintf(os.Stderr, "진단 보고서 저장 실패: %v\n", reportErr)
		} else {
			fmt.Fprintf(os.Stderr, "진단 저장: %s\n", reportPath)
		}
		fmt.Fprint(os.Stderr, renderOrchestraFailureSummary(resolvedTimeout, result, reportPath))
		fmt.Fprintf(os.Stderr, "상태: degraded\n")
	}
	if result.Reliability != nil && result.Reliability.ArtifactDir != "" {
		fmt.Fprintf(os.Stderr, "아티팩트: %s\n", result.Reliability.ArtifactDir)
	}
	fmt.Fprintf(os.Stderr, "\n요약: %s (총 %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

func writeOrchestraPrimaryOutput(w io.Writer, result *orchestra.OrchestraResult, noJudge bool, sessionID string) (bool, error) {
	if result.Yield != nil {
		return true, orchestra.WriteYieldOutput(w, *result.Yield)
	}
	if noJudge && len(result.RoundHistory) > 0 {
		output := orchestra.BuildYieldOutputFromResult(result, sessionID)
		return true, orchestra.WriteYieldOutput(w, output)
	}
	return false, nil
}
