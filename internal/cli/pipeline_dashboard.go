package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// newPipelineCmd creates the `auto pipeline` parent command with subcommands.
func newPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Pipeline monitoring and management",
	}

	cmd.AddCommand(newPipelineDashboardCmd())
	cmd.AddCommand(newPipelineRunCmd())

	return cmd
}

// newPipelineDashboardCmd creates the `auto pipeline dashboard <spec-id>` subcommand.
// It renders a one-shot pipeline dashboard to stdout (R8).
func newPipelineDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard <spec-id>",
		Short: "Render pipeline dashboard for a spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]

			if err := pipeline.ValidateSpecID(specID); err != nil {
				return err
			}

			cp, err := loadFlatCheckpoint(specCheckpointPath(specID))
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("load pipeline checkpoint for %s: %w", specID, err)
				}
				// Fallback to all-pending when checkpoint file does not exist.
				cmd.PrintErrln("Warning: no per-SPEC checkpoint found, showing default state")
				phases := make(map[string]pipeline.PhaseStatus)
				for _, phase := range pipeline.DefaultPhases() {
					phases[string(phase.ID)] = pipeline.PhasePending
				}
				data := pipeline.DashboardData{
					Phases: phases,
					Agents: map[string]string{},
				}
				output := pipeline.RenderDashboard(data)
				fmt.Fprintf(cmd.OutOrStdout(), "SPEC: %s\n%s", specID, output)
				return nil
			}

			data := pipeline.MapCheckpointToPhases(cp)
			output := pipeline.RenderDashboard(data)
			fmt.Fprintf(cmd.OutOrStdout(), "SPEC: %s\n%s", specID, output)
			return nil
		},
	}
}
