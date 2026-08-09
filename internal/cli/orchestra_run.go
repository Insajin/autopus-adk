package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// newOrchestraRunCmd creates the `auto orchestra run` subcommand.
// This is the entry point for the subprocess-based orchestration pipeline.
func newOrchestraRunCmd() *cobra.Command {
	var (
		strategy         string
		providers        []string
		rounds           string
		timeout          int
		judge            string
		subprocess       bool
		dryRun           bool
		jsonOut          bool
		format           string
		requireAgreement float64
	)

	cmd := &cobra.Command{
		Use:   "run [topic]",
		Short: "Run subprocess-based orchestration pipeline",
		Long:  "Execute a multi-provider debate pipeline using subprocess execution with JSON schema enforcement.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := strings.Join(args, " ")
			timeoutChanged := cmd.Flags().Changed("timeout")
			jsonMode, err := resolveJSONMode(jsonOut, format)
			if err != nil {
				return err
			}
			return runSubprocessPipeline(cmd, topic, strategy, providers, rounds, timeout, timeoutChanged, judge, subprocess, dryRun, jsonMode, requireAgreement)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "debate", "Orchestration strategy (debate|consensus|recheck)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "Provider list (default: all configured)")
	cmd.Flags().StringVar(&rounds, "rounds", "standard", "Round preset: fast, standard, deep")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "Per-provider timeout (seconds)")
	cmd.Flags().StringVar(&judge, "judge", "", "Judge provider name")
	cmd.Flags().BoolVar(&subprocess, "subprocess", false, "Force subprocess backend (default: auto-detect)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Output prompts to files without executing")
	cmd.Flags().Float64Var(&requireAgreement, "require-agreement", 0,
		"Block a consensus run whose provider agreement ratio falls below this floor (0-1; 0 disables)")
	addJSONFlags(cmd, &jsonOut, &format)

	return cmd
}

// runSubprocessPipeline executes the subprocess-based orchestration pipeline.
// @AX:WARN: [AUTO] high-branch CLI pipeline — config, invoker judge, backend, dry-run, and failure gates converge here
// @AX:REASON: [AUTO] more than eight conditional branches determine externally visible run behavior
func runSubprocessPipeline(
	cmd *cobra.Command,
	topic, strategyStr string,
	providerNames []string,
	roundsPreset string,
	timeout int,
	timeoutChanged bool,
	judgeName string,
	forceSubprocess bool,
	dryRun bool,
	jsonMode bool,
	requireAgreement float64,
) error {
	ctx := cmd.Context()
	requestedStrategy := orchestra.Strategy(strings.ToLower(strings.TrimSpace(strategyStr)))
	if requestedStrategy != orchestra.StrategyDebate &&
		requestedStrategy != orchestra.StrategyConsensus &&
		requestedStrategy != orchestra.StrategyRecheck {
		return fmt.Errorf("unsupported orchestra run strategy %q (use debate, consensus, or recheck)", strategyStr)
	}
	if requireAgreement < 0 || requireAgreement > 1 {
		return fmt.Errorf("--require-agreement must be between 0 and 1, got %v", requireAgreement)
	}
	if requireAgreement > 0 && requestedStrategy != orchestra.StrategyConsensus {
		return fmt.Errorf("--require-agreement needs --strategy consensus; %q produces no agreement ratio", strategyStr)
	}
	explicitProviderSelection := len(providerNames) > 0
	flagJudge := strings.TrimSpace(judgeName)
	explicitJudge := flagJudge != ""
	runtimeFlags := globalFlagsFromContext(ctx)
	harnessCfg, configErr := orchestraRunLoadConfig(runtimeFlags)
	var orchConf *config.OrchestraConf
	if harnessCfg != nil {
		orchConf = &harnessCfg.Orchestra
	}

	var providerConfigs []orchestra.ProviderConfig
	if configErr != nil || orchConf == nil {
		if len(providerNames) == 0 {
			providerNames = defaultProviders()
		}
		providerConfigs = orchestraRunBuildProviders(providerNames, runtimeFlags.Quality, runtimeFlags.Effort)
	} else {
		providerConfigs = resolveProviders(orchConf, "run", providerNames)
		if judgeName == "" {
			judgeName = resolveJudge(orchConf, "run", "")
		}
		if explicitProviderSelection && !explicitJudge && judgeName != "" && !hasProviderConfig(providerConfigs, judgeName) && len(providerConfigs) > 0 {
			judgeName = providerConfigs[0].Name
		}
		timeout = resolveCommandTimeout(orchConf, timeout, timeoutChanged)
	}

	if configErr != nil || orchConf == nil {
		timeout = resolveCommandTimeout(nil, timeout, timeoutChanged)
	}
	providerConfigs = resolveCodexProviderCapabilities(ctx, providerConfigs)

	if len(providerConfigs) == 0 {
		return fmt.Errorf("no providers available")
	}
	invokingProvider := ""
	judgeSelectionSource := ""
	if requestedStrategy == orchestra.StrategyDebate {
		invokingProvider = detectOrchestraInvokingProvider()
		judgeName = resolveInvocationJudge(
			flagJudge,
			judgeName,
			invokingProvider,
		)
		judgeSelectionSource = invocationJudgeSelectionSource(flagJudge, invokingProvider)
	}
	configuredNames := providerConfigNames(providerConfigs)

	// Resolve round preset.
	roundCount, ok := orchestra.RoundPresets[roundsPreset]
	if !ok {
		return fmt.Errorf("unknown round preset %q (use fast, standard, or deep)", roundsPreset)
	}

	// Resolve judge config.
	var judgeCfg orchestra.ProviderConfig
	if judgeName != "" {
		for _, p := range providerConfigs {
			if p.Name == judgeName {
				judgeCfg = p
				break
			}
		}
		if judgeCfg.Name == "" {
			judgeCfg = orchestra.ProviderConfig{Name: judgeName, Binary: judgeName}
		}
	} else if len(providerConfigs) > 0 {
		judgeCfg = providerConfigs[0] // default: first provider
	}

	// Build prompt data.
	promptData := orchestra.PromptData{
		ProjectName:    "autopus-adk",
		ProjectSummary: "Agentic Development Kit CLI",
		TechStack:      "Go",
		MustReadFiles:  []string{"ARCHITECTURE.md", "go.mod"},
		Topic:          topic,
		MaxTurns:       10,
		TargetModule:   ".",
	}

	if dryRun {
		return executeDryRun(topic, promptData, providerConfigs, roundCount)
	}

	// Choose backend (REQ-003). Inject the detected terminal so SelectBackend
	// returns the interactive pane backend on cmux/tmux terminals and the headless
	// subprocess backend on plain/CI terminals or when --subprocess is forced.
	cfg := orchestra.OrchestraConfig{
		Providers:             providerConfigs,
		RequestedProviders:    append([]string(nil), configuredNames...),
		ConfiguredProviders:   append([]string(nil), configuredNames...),
		Strategy:              requestedStrategy,
		Prompt:                topic,
		JudgeProvider:         judgeName,
		InvokingProvider:      invokingProvider,
		JudgeSelectionSource:  judgeSelectionSource,
		SubprocessMode:        forceSubprocess,
		TimeoutSeconds:        timeout,
		Terminal:              detectStructuredTerminal(),
		FallbackMode:          orchestra.FallbackModeSubprocess,
		MinimumAgreementRatio: requireAgreement,
	}
	// SPEC-ORCH-022 T8: enable hook-IPC collection before backend selection so a
	// pane-capable, hook-installed context collects via done-file instead of
	// screen polling.
	applyHookMode(&cfg)
	backend := orchestraRunBackendFactory(cfg)

	pipelineCfg := orchestra.SubprocessPipelineConfig{
		Backend:        backend,
		Providers:      providerConfigs,
		Topic:          topic,
		PromptData:     promptData,
		Rounds:         roundCount,
		Judge:          judgeCfg,
		TimeoutSeconds: timeout,
	}

	names := make([]string, len(providerConfigs))
	for i, p := range providerConfigs {
		names[i] = p.Name
	}
	terminalName := ""
	if cfg.Terminal != nil {
		terminalName = cfg.Terminal.Name()
	}
	fmt.Fprintf(os.Stderr, "Strategy: %s | Providers: %s | Rounds: %s (%d) | Backend: %s (terminal=%s, hook=%t)\n",
		strategyStr, strings.Join(names, ", "), roundsPreset, roundCount+1, backend.Name(), terminalName, cfg.HookMode)

	result, err := executeOrchestraRunStrategy(ctx, requestedStrategy, cfg, pipelineCfg)
	if err != nil {
		return fmt.Errorf("subprocess pipeline failed: %w", err)
	}
	if result == nil {
		return fmt.Errorf("subprocess pipeline failed: orchestration returned no result")
	}
	// A blocked gate is the case a caller most needs evidence for, so the receipt
	// is emitted before the command fails rather than discarded with the error.
	if shouldTreatOrchestraResultAsFailure(result) {
		blocked := fmt.Errorf("subprocess pipeline failed: %w", synthesizeOrchestraFailureError(result))
		if jsonMode {
			return writeJSONResultAndExit(
				cmd, jsonStatusError, blocked, "orchestra_run_blocked", result.RunReceipt, nil, nil,
			)
		}
		return blocked
	}

	// REQ-009: when all providers failed and no usable responses were collected,
	// surface an actionable error instead of printing an empty result.
	if len(result.Responses) == 0 && len(result.FailedProviders) > 0 {
		return bothBackendsUnavailableError(
			fmt.Sprintf("%d provider(s) configured, 0 responses received", len(providerConfigs)),
		)
	}

	if jsonMode {
		return writeJSONResult(cmd, orchestraRunJSONStatus(result), result.RunReceipt, nil, nil)
	}
	fmt.Println(result.Merged)
	fmt.Fprintf(os.Stderr, "\nSummary: %s (total %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

// orchestraRunJSONStatus maps a completed run onto the shared CLI envelope status
// so a caller can gate on provider disagreement without parsing rendered markdown.
func orchestraRunJSONStatus(result *orchestra.OrchestraResult) jsonEnvelopeStatus {
	if result.Degraded || result.GateStatus == "degraded" {
		return jsonStatusWarn
	}
	return jsonStatusOK
}

func hasProviderConfig(providers []orchestra.ProviderConfig, name string) bool {
	for _, provider := range providers {
		if provider.Name == name {
			return true
		}
	}
	return false
}
