package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
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
	// @AX:NOTE: [AUTO] magic constant — default strategy "sequential" encodes execution policy; change with care
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

// @AX:NOTE: [AUTO] magic constants — platform probe order ["claude", "codex", "agy"] and fallback "claude" are implicit policy
// resolvePlatform returns the platform to use: the value as-is when non-empty,
// or the first AI binary found in PATH (claude, codex, agy).
func resolvePlatform(platform string) string {
	if platform != "" {
		return platform
	}
	for _, candidate := range []struct {
		binary   string
		provider string
	}{
		{binary: "claude", provider: "claude"},
		{binary: "codex", provider: "codex"},
		{binary: "agy", provider: "gemini"},
	} {
		if _, err := exec.LookPath(candidate.binary); err == nil {
			return candidate.provider
		}
	}
	// Fall back to "claude" as the default when nothing is found in PATH.
	return "claude"
}

// @AX:ANCHOR: [AUTO] CLI integration boundary — wires cobra command args into pipeline engine (fan-in: CLI + tests)
// @AX:REASON: Resolves SPEC identity, executable backend, and canonical receipt storage before dispatch.
// runPipeline executes the pipeline for the given SPEC ID.
func runPipeline(cmd *cobra.Command, specID string, cfg *pipelineRunConfig) error {
	if err := pipeline.ValidateSpecID(specID); err != nil {
		return err
	}
	gitHash, _ := getCurrentGitHash()
	requestedStrategy := pipeline.Strategy(cfg.Strategy)
	resolvedSpec, err := resolvePipelineSpec(specID)
	if err != nil {
		return pipelineBlockedError(specID, "", gitHash, requestedStrategy, err)
	}
	if err := pipeline.ValidateStrategy(requestedStrategy); err != nil {
		return pipelineBlockedError(specID, resolvedSpec.SnapshotHash, gitHash, requestedStrategy, err)
	}
	platform := resolvePlatform(cfg.Platform)
	flags := globalFlagsFromContext(cmd.Context())

	cp, err := LoadCheckpointIfContinue(specID, cfg.Continue)
	if err != nil {
		return err
	}

	var backend pipeline.PhaseBackend
	if !cfg.DryRun {
		backend, err = newPipelineProviderBackend(platform)
		if err != nil {
			return pipelineBlockedError(specID, resolvedSpec.SnapshotHash, gitHash, requestedStrategy, err)
		}
	}

	// Initialize learn store if learnings directory exists.
	var learnStore *learn.Store
	learningsDir := filepath.Join(".autopus", "learnings")
	if _, statErr := os.Stat(learningsDir); statErr == nil {
		learnStore, _ = learn.NewStore(".")
	}

	engineCfg := pipeline.EngineConfig{
		SpecID:        specID,
		SpecDir:       resolvedSpec.Dir,
		Platform:      platform,
		Strategy:      requestedStrategy,
		Backend:       backend,
		Checkpoint:    cp,
		DryRun:        cfg.DryRun,
		SnapshotHash:  resolvedSpec.SnapshotHash,
		GitCommitHash: gitHash,
		RunConfig: pipeline.RunConfig{
			SpecID:        specID,
			CheckpointDir: pipelineStateDir,
			LearnStore:    learnStore,
		},
	}

	engine := pipeline.NewSubprocessEngine(engineCfg)
	result, err := engine.Run(cmd.Context())
	if err != nil {
		return fmt.Errorf("pipeline run failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Pipeline complete: %d phases executed\n", len(result.PhaseResults))
	if flags.MultiMode && !cfg.DryRun {
		fmt.Fprintf(cmd.ErrOrStderr(), "Running multi-provider review for %s\n", specID)
		if err := runSpecReview(cmd.Context(), specID, "", 0); err != nil {
			return fmt.Errorf("pipeline multi review failed: %w", err)
		}
	}
	return nil
}
