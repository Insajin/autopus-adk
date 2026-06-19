package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// newWorkflowGateCmd is the JS->Go execution bridge for the deterministic gate.
// It runs the build and test commands through the CommandRunner seam and prints
// the structured {verdict, verdict_source, build_exit, test_exit} JSON. It
// always exits 0 — the verdict lives in the JSON, which the workflow JS reads to
// branch (S16).
func newWorkflowGateCmd(runner workflow.CommandRunner) *cobra.Command {
	var buildCmd []string
	var testCmd []string

	cmd := &cobra.Command{
		Use:           "gate",
		Short:         "Run build/test and emit an exit-code-derived verdict JSON (JS->Go bridge)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := runner
			if r == nil {
				r = execCommandRunner{}
			}
			result := workflow.EvaluateGate(cmd.Context(), r, buildCmd, testCmd)
			data, err := result.EncodeJSON()
			if err != nil {
				return fmt.Errorf("encode gate result: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&buildCmd, "build", []string{"go", "build", "./..."}, "Build command (deterministic gate)")
	cmd.Flags().StringArrayVar(&testCmd, "test", []string{"go", "test", "./..."}, "Test command (deterministic gate)")
	return cmd
}
