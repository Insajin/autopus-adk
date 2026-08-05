package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompPlatformDependencies struct {
	newRunner func() omp.OMPModelCatalogRunner
	activate  func(context.Context, string, *config.HarnessConfig) error
	now       func() time.Time
}

func defaultOMPPlatformDependencies() ompPlatformDependencies {
	return ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return newOMPOperatorExecRunner() },
		activate: func(ctx context.Context, root string, cfg *config.HarnessConfig) error {
			recognized, err := updateHarnessPlatform(ctx, root, "omp", cfg)
			if !recognized && err == nil {
				return fmt.Errorf("OMP update path is unavailable")
			}
			return err
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

func normalizeOMPPlatformDependencies(deps ompPlatformDependencies) ompPlatformDependencies {
	defaults := defaultOMPPlatformDependencies()
	if deps.newRunner == nil {
		deps.newRunner = defaults.newRunner
	}
	if deps.activate == nil {
		deps.activate = defaults.activate
	}
	if deps.now == nil {
		deps.now = defaults.now
	}
	return deps
}

func newPlatformOMPCmd(dir *string) *cobra.Command {
	return newPlatformOMPCmdWithDependencies(dir, defaultOMPPlatformDependencies())
}

func newPlatformOMPCmdWithDependencies(dir *string, deps ompPlatformDependencies) *cobra.Command {
	deps = normalizeOMPPlatformDependencies(deps)
	cmd := &cobra.Command{
		Use:   "omp",
		Short: "Inspect and manage installed OMP models",
	}
	cmd.AddCommand(newPlatformOMPModelsCmd(dir, deps))
	cmd.AddCommand(newPlatformOMPProfileCmd(dir, deps))
	cmd.AddCommand(newPlatformOMPExplainCmd(dir, deps))
	return cmd
}

func newPlatformOMPModelsCmd(dir *string, deps ompPlatformDependencies) *cobra.Command {
	var jsonOutput bool
	var format string
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the installed exact OMP model catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonMode, err := resolveOMPOutputMode(cmd, jsonOutput, format)
			if err != nil {
				return err
			}
			root, err := resolveDir(*dir)
			if err != nil {
				return err
			}
			return runOMPModelsCommand(cmd, root, jsonMode, deps.newRunner())
		},
	}
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func newPlatformOMPProfileCmd(dir *string, deps ompPlatformDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Plan or apply an OMP model profile",
	}
	cmd.AddCommand(newPlatformOMPProfileInitCmd(dir, deps))
	cmd.AddCommand(newPlatformOMPProfileApplyCmd(dir, deps))
	return cmd
}

func newPlatformOMPProfileInitCmd(dir *string, deps ompPlatformDependencies) *cobra.Command {
	var name string
	var plan bool
	var jsonOutput bool
	var format string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Propose a deterministic six-capability profile without writing files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonMode, err := resolveOMPOutputMode(cmd, jsonOutput, format)
			if err != nil {
				return err
			}
			root, err := resolveDir(*dir)
			if err != nil {
				return err
			}
			return runOMPProfileInitCommand(cmd, root, name, plan, jsonMode, deps.newRunner())
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Profile name")
	cmd.Flags().BoolVar(&plan, "plan", false, "Show the zero-write apply plan")
	addJSONFlags(cmd, &jsonOutput, &format)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newPlatformOMPProfileApplyCmd(dir *string, deps ompPlatformDependencies) *cobra.Command {
	var jsonOutput bool
	var format string
	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Persist, select, and activate an OMP model profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveOMPOutputMode(cmd, jsonOutput, format)
			if err != nil {
				return err
			}
			root, err := resolveDir(*dir)
			if err != nil {
				return err
			}
			return runOMPProfileApplyCommand(
				cmd, root, args[0], jsonMode, deps.newRunner(), deps.activate,
			)
		},
	}
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func newPlatformOMPExplainCmd(dir *string, deps ompPlatformDependencies) *cobra.Command {
	var jsonOutput bool
	var format string
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain effective OMP agent model routing and fallbacks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonMode, err := resolveOMPOutputMode(cmd, jsonOutput, format)
			if err != nil {
				return err
			}
			root, err := resolveDir(*dir)
			if err != nil {
				return err
			}
			projection := buildOMPPlatformProjection(
				cmd.Context(), root, deps.newRunner(), deps.now(),
			)
			return renderOMPExplain(cmd, projection, jsonMode)
		},
	}
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func resolveOMPOutputMode(cmd *cobra.Command, jsonOutput bool, format string) (bool, error) {
	jsonMode, err := resolveJSONMode(jsonOutput, format)
	if err == nil {
		return jsonMode, nil
	}
	if jsonOutput {
		return false, writeJSONResultAndExit(
			cmd, jsonStatusError, err, "invalid_format", map[string]any{}, nil, nil,
		)
	}
	return false, err
}
