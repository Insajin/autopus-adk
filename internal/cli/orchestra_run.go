package cli

import (
	"context"
	"encoding/json"
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
		strategy   string
		providers  []string
		rounds     string
		timeout    int
		judge      string
		subprocess bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "run [topic]",
		Short: "Run subprocess-based orchestration pipeline",
		Long:  "Execute a multi-provider debate pipeline using subprocess execution with JSON schema enforcement.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := strings.Join(args, " ")
			timeoutChanged := cmd.Flags().Changed("timeout")
			return runSubprocessPipeline(cmd.Context(), topic, strategy, providers, rounds, timeout, timeoutChanged, judge, subprocess, dryRun)
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "debate", "Orchestration strategy (debate|consensus)")
	cmd.Flags().StringSliceVarP(&providers, "providers", "p", nil, "Provider list (default: all configured)")
	cmd.Flags().StringVar(&rounds, "rounds", "standard", "Round preset: fast, standard, deep")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 120, "Per-provider timeout (seconds)")
	cmd.Flags().StringVar(&judge, "judge", "", "Judge provider name")
	cmd.Flags().BoolVar(&subprocess, "subprocess", false, "Force subprocess backend (default: auto-detect)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Output prompts to files without executing")

	return cmd
}

// runSubprocessPipeline executes the subprocess-based orchestration pipeline.
func runSubprocessPipeline(
	ctx context.Context,
	topic, strategyStr string,
	providerNames []string,
	roundsPreset string,
	timeout int,
	timeoutChanged bool,
	judgeName string,
	forceSubprocess bool,
	dryRun bool,
) error {
	explicitProviderSelection := len(providerNames) > 0
	explicitJudge := judgeName != ""
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
		Providers:      providerConfigs,
		SubprocessMode: forceSubprocess,
		TimeoutSeconds: timeout,
		Terminal:       detectStructuredTerminal(),
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
	fmt.Fprintf(os.Stderr, "Strategy: %s | Providers: %s | Rounds: %s (%d)\n",
		strategyStr, strings.Join(names, ", "), roundsPreset, roundCount+1)

	result, err := orchestraRunExecutePipeline(ctx, pipelineCfg)
	if err != nil {
		return fmt.Errorf("subprocess pipeline failed: %w", err)
	}

	// REQ-009: when all providers failed and no usable responses were collected,
	// surface an actionable error instead of printing an empty result.
	if len(result.Responses) == 0 && len(result.FailedProviders) > 0 {
		return bothBackendsUnavailableError(
			fmt.Sprintf("%d provider(s) configured, 0 responses received", len(providerConfigs)),
		)
	}

	fmt.Println(result.Merged)
	fmt.Fprintf(os.Stderr, "\nSummary: %s (total %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

func hasProviderConfig(providers []orchestra.ProviderConfig, name string) bool {
	for _, provider := range providers {
		if provider.Name == name {
			return true
		}
	}
	return false
}

// executeDryRun writes prompts to files without executing providers.
func executeDryRun(topic string, data orchestra.PromptData, providers []orchestra.ProviderConfig, rounds int) error {
	pb, err := orchestra.NewPromptBuilder()
	if err != nil {
		return fmt.Errorf("dry-run: %w", err)
	}

	r1Prompt, manifest, err := pb.BuildDebaterR1WithManifest(data)
	if err != nil {
		return fmt.Errorf("dry-run: r1 prompt: %w", err)
	}

	safeTopic := sanitizeFilename(topic)
	if safeTopic == "" {
		return fmt.Errorf("dry-run: topic does not produce a safe filename")
	}
	r1File := fmt.Sprintf("orchestra-r1-%s.md", safeTopic)
	if writeErr := writeNewPrivateFile(r1File, []byte(r1Prompt)); writeErr != nil {
		return fmt.Errorf("dry-run: write r1: %w", writeErr)
	}
	fmt.Printf("Round 1 prompt: %s\n", r1File)

	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: dry-run sidecar suffix is asserted by prompt manifest tooling.
	manifestFile := strings.TrimSuffix(r1File, ".md") + ".manifest.json"
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("dry-run: manifest: %w", err)
	}
	if writeErr := writeNewPrivateFile(manifestFile, append(manifestBody, '\n')); writeErr != nil {
		return fmt.Errorf("dry-run: write manifest: %w", writeErr)
	}
	fmt.Printf("Round 1 prompt layer manifest: %s\n", manifestFile)

	schema := &orchestra.SchemaBuilder{}
	for _, role := range []string{"debater_r1", "debater_r2", "judge"} {
		path, cleanup, schemaErr := schema.WriteToFile(role)
		if schemaErr != nil {
			return fmt.Errorf("dry-run: schema %s: %w", role, schemaErr)
		}
		fmt.Printf("Schema (%s): %s\n", role, path)
		_ = cleanup // keep files in dry-run mode
	}

	fmt.Fprintf(os.Stderr, "Dry run complete. %d providers, %d rounds.\n", len(providers), rounds+1)
	return nil
}

func writeNewPrivateFile(path string, body []byte) error {
	if path == "" {
		return fmt.Errorf("empty output path")
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to overwrite symlink: %s", path)
		}
		return fmt.Errorf("refuse to overwrite existing file: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// sanitizeFilename replaces non-alphanumeric characters for safe filenames.
func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}
