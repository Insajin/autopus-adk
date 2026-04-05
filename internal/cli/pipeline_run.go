package cli

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// pipelineRunConfig holds parsed flag values for the pipeline run command.
type pipelineRunConfig struct {
	Platform string
	Strategy string
	Continue bool
	DryRun   bool
}

// newPipelineRunCmd creates the `auto pipeline run <spec-id>` subcommand.
func newPipelineRunCmd() *cobra.Command {
	cfg := &pipelineRunConfig{}
	return newPipelineRunCmdWithConfig(cfg)
}

// newPipelineRunCmdWithConfig creates the pipeline run command bound to the
// given config pointer, allowing tests to inspect parsed flag values.
func newPipelineRunCmdWithConfig(cfg *pipelineRunConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <spec-id>",
		Short: "Execute a full pipeline for a SPEC",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("spec-id argument is required: auto pipeline run <spec-id>")
			}
			if err := pipeline.ValidateSpecID(args[0]); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]
			return runPipeline(cmd, specID, cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.Platform, "platform", "", "AI platform to use (claude, codex, gemini). Auto-detected when omitted.")
	cmd.Flags().Var(newStrategyValue("sequential", &cfg.Strategy), "strategy", "Execution strategy: sequential or parallel.")
	cmd.Flags().BoolVar(&cfg.Continue, "continue", false, "Resume from the last saved checkpoint.")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "Build prompts without invoking the backend.")

	return cmd
}

// strategyValue is a pflag.Value implementation that validates strategy on Set.
type strategyValue struct {
	val *string
}

// newStrategyValue creates a strategyValue with the given default.
func newStrategyValue(defaultVal string, p *string) *strategyValue {
	*p = defaultVal
	return &strategyValue{val: p}
}

// String returns the current value.
func (s *strategyValue) String() string { return *s.val }

// Type returns the flag type name.
func (s *strategyValue) Type() string { return "strategy" }

// Set validates and stores the strategy value.
func (s *strategyValue) Set(v string) error {
	switch v {
	case "sequential", "parallel":
		*s.val = v
		return nil
	default:
		return fmt.Errorf("invalid strategy %q: must be sequential or parallel", v)
	}
}

// @AX:NOTE: [AUTO] magic constants — platform probe order ["claude", "codex", "gemini"] and fallback "claude" are implicit policy
// resolvePlatform returns the platform to use: the value as-is when non-empty,
// or the first AI binary found in PATH (claude, codex, gemini).
func resolvePlatform(platform string) string {
	if platform != "" {
		return platform
	}
	for _, candidate := range []string{"claude", "codex", "gemini"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	// Fall back to "claude" as the default when nothing is found in PATH.
	return "claude"
}

// @AX:ANCHOR: [AUTO] CLI integration boundary — wires cobra command args into pipeline engine (fan-in: CLI + tests)
// @AX:TODO: [AUTO] resolvePlatform has no unit test — PATH-dependent behavior is untested @AX:CYCLE:1
// runPipeline executes the pipeline for the given SPEC ID.
func runPipeline(cmd *cobra.Command, specID string, cfg *pipelineRunConfig) error {
	platform := resolvePlatform(cfg.Platform)

	cp, err := LoadCheckpointIfContinue(specID, cfg.Continue)
	if err != nil {
		return err
	}

	engineCfg := pipeline.EngineConfig{
		SpecID:     specID,
		Platform:   platform,
		Strategy:   pipeline.Strategy(cfg.Strategy),
		Checkpoint: cp,
		DryRun:     cfg.DryRun,
	}

	engine := pipeline.NewSubprocessEngine(engineCfg)
	result, err := engine.Run(context.Background())
	if err != nil {
		return fmt.Errorf("pipeline run failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Pipeline complete: %d phases executed\n", len(result.PhaseResults))
	return nil
}

