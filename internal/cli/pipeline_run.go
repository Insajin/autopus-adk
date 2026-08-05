package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/learn"
	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// pipelineRunConfig holds parsed flag values for the pipeline run command.
type pipelineRunConfig struct {
	Platform               string
	Strategy               string
	ExecutionOwner         string
	executionOwnerExplicit bool
	Continue               bool
	DryRun                 bool
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
		Use:          "run <spec-id>",
		Short:        "Execute a full pipeline for a SPEC",
		SilenceUsage: true,
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

	cmd.Flags().StringVar(&cfg.Platform, "platform", "", "AI platform to use (claude, codex, gemini, omp). Auto-detected when omitted.")
	cmd.Flags().Var(
		newExecutionOwnerValue(&cfg.ExecutionOwner, &cfg.executionOwnerExplicit),
		"execution-owner",
		"OMP DAG owner: exact value omp or orca. Defaults to omp.",
	)
	// @AX:NOTE [AUTO]: magic constant — default strategy "sequential" encodes execution policy; change with care
	cmd.Flags().Var(newStrategyValue("sequential", &cfg.Strategy), "strategy", "Execution strategy: sequential.")
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
	if v != "sequential" {
		return fmt.Errorf("invalid strategy %q: must be sequential", v)
	}
	*s.val = v
	return nil
}

// @AX:NOTE [AUTO]: magic constants — platform probe order ["claude", "codex", "agy"] and fallback "claude" are implicit policy
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

// @AX:ANCHOR [AUTO]: CLI integration boundary — wires cobra command args into pipeline engine (fan-in: CLI + tests)
// @AX:REASON [AUTO]: Resolves SPEC identity, executable backend, and canonical receipt storage before dispatch.
// @AX:WARN [AUTO]: Pipeline launch has more than eight validation, backend-selection, persistence, and review branches.
// @AX:REASON [AUTO]: Every preflight failure must persist a blocked receipt before any phase authority is dispatched.
// runPipeline executes the pipeline for the given SPEC ID.
func runPipeline(cmd *cobra.Command, specID string, cfg *pipelineRunConfig) error {
	if err := pipeline.ValidateSpecID(specID); err != nil {
		return err
	}
	ownerDecision, err := resolvePipelineExecutionOwner(cfg)
	if err != nil {
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
	projectDir, err := os.Getwd()
	if err != nil {
		return pipelineBlockedError(specID, resolvedSpec.SnapshotHash, gitHash, requestedStrategy,
			fmt.Errorf("pipeline: resolve project directory: %w", err))
	}
	projectDir = filepath.Clean(projectDir)
	if platform == "omp" {
		ownerReceipt, ownerReceiptPath, receiptErr := persistPipelineExecutionOwnerReceipt(
			specID, ownerDecision,
		)
		if receiptErr != nil {
			return receiptErr
		}
		if ownerDecision.Owner == pipelineExecutionOwnerOrca {
			return emitPipelineExecutionOwnerHandoff(cmd, ownerReceipt, ownerReceiptPath)
		}
	}

	cp, err := LoadCheckpointIfContinue(specID, cfg.Continue)
	if err != nil {
		return err
	}

	var backend pipeline.PhaseBackend
	if !cfg.DryRun {
		if platform == "omp" {
			backend, err = newPipelineOMPBackendForRun(projectDir, specID, resolvedSpec, gitHash)
		} else {
			backend, err = newPipelineProviderBackend(platform)
		}
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
		ProjectDir:    projectDir,
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

func newPipelineOMPBackendForRun(
	projectDir string,
	specID string,
	resolvedSpec resolvedPipelineSpec,
	gitHash string,
) (*pipelineOMPBackend, error) {
	executable, err := exec.LookPath("omp")
	if err != nil {
		return nil, fmt.Errorf("pipeline: platform %q executable %q is unavailable: %w", "omp", "omp", err)
	}
	executable, executableID, err := canonicalPipelineOMPExecutable(executable)
	if err != nil {
		return nil, err
	}
	environment, err := normalizePipelineOMPEnvironment(os.Environ())
	if err != nil {
		return nil, err
	}
	phaseModels, err := loadPipelineOMPPhaseModelsWithAuthority(projectDir, executable, executableID, environment)
	if err != nil {
		return nil, fmt.Errorf("pipeline: load OMP model routes: %w", err)
	}
	return newPipelineOMPBackend(pipelineOMPBackendConfig{
		Executable: executable, ProjectDir: projectDir, SpecID: specID, SpecDir: resolvedSpec.Dir,
		SnapshotHash: resolvedSpec.SnapshotHash, GitCommitHash: gitHash,
		// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: each canonical OMP phase is bounded to 30 minutes.
		Environment: environment, PhaseModels: phaseModels, MaxTime: 30 * time.Minute, executableID: executableID,
		ManagedActive: newPipelineOMPManagedActiveCoordinator(),
	})
}
