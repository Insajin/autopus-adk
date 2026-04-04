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
	)

	root := &cobra.Command{
		Use:           "auto",
		Short:         "Autopus-ADK: Agentic Development Kit",
		Long:          "Autopus-ADK는 코딩 CLI에 하네스를 설치하는 Go 기반 셋업 도구입니다.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ./autopus.yaml)")

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
	root.AddCommand(newWorkerCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newLearnCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			if short {
				fmt.Fprintln(out, version.Version())
				return
			}
			tui.Banner(out)
			fmt.Fprintln(out, version.String())
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print version number only (no banner)")
	return cmd
}

// Execute runs the CLI.
func Execute() {
	// Initialize styles with NO_COLOR guard for non-TTY environments.
	// Must run before any lipgloss.NewStyle() or .Render() call.
	tui.InitStyles()

	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
