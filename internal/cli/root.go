// Package cli defines Cobra-based CLI commands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/version"
)

// NewRootCmd creates the root command.
// Uses local variables instead of package-level to prevent data races in parallel tests.
func NewRootCmd() *cobra.Command {
	// Declare flag variables locally so each invocation has independent state.
	var (
		verbose    bool
		configPath string
		think      bool
		ultraThink bool
		autoMode   bool
		loopMode   bool
		multiMode  bool
		quality    string
		effort     string
		taskMode   string
	)

	root := &cobra.Command{
		Use:              "auto",
		Short:            "Autopus-ADK: Agentic Development Kit",
		Long:             "Autopus-ADK는 코딩 CLI에 하네스를 설치하는 Go 기반 셋업 도구입니다.",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			flags, err := collectGlobalFlags(cmd, configPath)
			if err != nil {
				return err
			}
			ctx := withGlobalFlags(cmd.Context(), flags)
			cmd.Root().SetContext(ctx)
			cmd.SetContext(ctx)
			return nil
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ./autopus.yaml)")
	root.PersistentFlags().BoolVar(&think, "think", false, "Enable step-by-step reasoning mode")
	root.PersistentFlags().BoolVar(&ultraThink, "ultrathink", false, "Enable deeper step-by-step reasoning mode")
	root.PersistentFlags().BoolVar(&autoMode, "auto", false, "Run without confirmation prompts")
	root.PersistentFlags().BoolVar(&loopMode, "loop", false, "Retry quality gates until pass or circuit break")
	root.PersistentFlags().BoolVar(&multiMode, "multi", false, "Enable multi-provider review/orchestration mode")
	root.PersistentFlags().StringVar(&quality, "quality", "", "Quality mode preset (ultra/balanced)")
	root.PersistentFlags().StringVar(&effort, "effort", "", "Explicit effort value (overrides quality-mode mapping)")
	root.PersistentFlags().StringVar(&taskMode, "task-created-mode", "", "Override TaskCreated mode for this invocation (warn|enforce)")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newPlatformCmd())
	root.AddCommand(newArchCmd())
	root.AddCommand(newLoreCmd())
	root.AddCommand(newSpecCmd())
	root.AddCommand(newLSPCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newDocsCmd())
	root.AddCommand(newHashCmd())
	root.AddCommand(newSkillCmd())
	root.AddCommand(newOrchestraCmd())
	root.AddCommand(newSetupCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newTelemetryCmd())
	root.AddCommand(newIssueCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newExperimentCmd())
	// @AX:NOTE [AUTO] @AX:REASON: Phase 2 addition — registers `auto test` and `auto test run` subcommands; added as part of SPEC-E2E-001
	root.AddCommand(newAutoTestCmd())
	root.AddCommand(newAgentCmd())
	root.AddCommand(newReactCmd())
	root.AddCommand(newTerminalCmd())
	root.AddCommand(newPipelineCmd())
	root.AddCommand(newPermissionCmd())
	root.AddCommand(newEffortCmd())
	root.AddCommand(newDesktopCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newWorkerCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newLearnCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	var short bool
	var showPath bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if short {
				fmt.Fprintln(out, version.Version())
				return nil
			}

			pathInfo, err := resolveCurrentBinaryPath()
			if err != nil {
				return err
			}
			if showPath {
				fmt.Fprintln(out, pathInfo.ManagedPath())
				return nil
			}

			tui.Banner(out)
			fmt.Fprintln(out, version.String())
			fmt.Fprintf(out, "path: %s\n", pathInfo.ManagedPath())
			if pathInfo.IsSymlinked() {
				fmt.Fprintf(out, "invoked via: %s\n", pathInfo.ExecutablePath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print version number only (no banner)")
	cmd.Flags().BoolVar(&showPath, "path", false, "Print the canonical binary path only")
	return cmd
}

// Execute runs the CLI.
func Execute() {
	// Initialize styles with NO_COLOR guard for non-TTY environments.
	// Must run before any lipgloss.NewStyle() or .Render() call.
	tui.InitStyles()

	if err := NewRootCmd().Execute(); err != nil {
		if isJSONFatalError(err) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
