package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
	orchConf, configErr := orchestraRunLoadConfig()

	var providerConfigs []orchestra.ProviderConfig
	if configErr != nil || orchConf == nil {
		if len(providerNames) == 0 {
			providerNames = defaultProviders()
		}
		providerConfigs = orchestraRunBuildProviders(providerNames)
	} else {
		providerConfigs = resolveProviders(orchConf, "run", providerNames)
		if judgeName == "" {
			judgeName = resolveJudge(orchConf, "run", "")
		}
		timeout = resolveCommandTimeout(orchConf, timeout, timeoutChanged)
	}

	if configErr != nil || orchConf == nil {
		timeout = resolveCommandTimeout(nil, timeout, timeoutChanged)
	}

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

	// Choose backend.
	cfg := orchestra.OrchestraConfig{
		Providers:      providerConfigs,
		SubprocessMode: forceSubprocess,
		TimeoutSeconds: timeout,
	}
	_ = orchestra.SelectBackend(cfg) // validate selection

	pipelineCfg := orchestra.SubprocessPipelineConfig{
		Backend:        orchestraRunBackendFactory(),
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

	fmt.Println(result.Merged)
	fmt.Fprintf(os.Stderr, "\nSummary: %s (total %s)\n", result.Summary, result.Duration.Round(1e6))
	return nil
}

// executeDryRun writes prompts to files without executing providers.
func executeDryRun(topic string, data orchestra.PromptData, providers []orchestra.ProviderConfig, rounds int) error {
	pb, err := orchestra.NewPromptBuilder()
	if err != nil {
		return fmt.Errorf("dry-run: %w", err)
	}

	r1Prompt, err := pb.BuildDebaterR1(data)
	if err != nil {
		return fmt.Errorf("dry-run: r1 prompt: %w", err)
	}

	r1File := fmt.Sprintf("orchestra-r1-%s.md", sanitizeFilename(topic))
	if writeErr := os.WriteFile(r1File, []byte(r1Prompt), 0644); writeErr != nil {
		return fmt.Errorf("dry-run: write r1: %w", writeErr)
	}
	fmt.Printf("Round 1 prompt: %s\n", r1File)

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
