package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func newWorkflowContextObserveSessionCmd() *cobra.Command {
	var input, output, format string
	var explicitLive bool
	options := workflowContextObserveSessionOptions{}
	cmd := &cobra.Command{
		Use:           "observe-session",
		Short:         "Run a production-equivalent explicit-live OMP observation session",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !explicitLive {
				return errors.New("workflow context-runtime observe-session requires --explicit-live")
			}
			if input != "-" || output != "-" || strings.ToLower(strings.TrimSpace(format)) != "jsonl" {
				return errors.New("workflow context-runtime observe-session requires --input-jsonl - --output - --format jsonl")
			}
			if err := validateWorkflowContextObserveSessionOptions(options); err != nil {
				return err
			}
			return RunWorkflowContextObserveSession(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), options)
		},
	}
	cmd.Flags().BoolVar(&explicitLive, "explicit-live", false, "Acknowledge external provider execution")
	cmd.Flags().StringVar(&input, "input-jsonl", "", "Read strict handshake/call/shutdown JSONL from stdin (-)")
	cmd.Flags().StringVar(&output, "output", "", "Write strict response JSONL (-)")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format (jsonl)")
	cmd.Flags().StringVar(&options.ProjectDir, "project-dir", ".", "Candidate project root")
	cmd.Flags().StringVar(&options.SpecID, "spec-id", "", "SPEC identity")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Signed cohort provider identity")
	cmd.Flags().StringVar(&options.Model, "model", "", "Signed cohort model identity")
	cmd.Flags().StringVar(&options.Endpoint, "endpoint", "", "Task-owned loopback broker endpoint")
	cmd.Flags().StringVar(&options.CredentialLocator, "credential-locator", "", "Dedicated credential environment key")
	cmd.Flags().StringVar(&options.Executable, "omp", "omp", "OMP executable")
	cmd.Flags().StringVar(&options.TargetGitCommit, "target-git-commit", "", "Exact target project commit")
	return cmd
}
