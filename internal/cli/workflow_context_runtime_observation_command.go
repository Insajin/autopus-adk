package cli

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

func newWorkflowContextObservationCmd() *cobra.Command {
	var manifest, output, format string
	var explicitLive bool
	cmd := &cobra.Command{
		Use:           "cohort",
		Short:         "Validate one explicit-live OMP context observation cohort",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !explicitLive {
				return errors.New("workflow context-runtime cohort: --explicit-live is required")
			}
			if strings.TrimSpace(manifest) == "" || manifest == "-" {
				return errors.New("workflow context-runtime cohort: a manifest file is required")
			}
			if output != "-" || strings.ToLower(strings.TrimSpace(format)) != "json" {
				return errors.New("workflow context-runtime cohort: only --output - --format json is supported")
			}
			observation, err := loadPipelineOMPContextObservation(manifest)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetEscapeHTML(false)
			return encoder.Encode(observation)
		},
	}
	cmd.Flags().StringVar(&manifest, "manifest", "", "Path to one body-free observation manifest")
	cmd.Flags().StringVar(&output, "output", "", "Validated observation output (-)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json)")
	cmd.Flags().BoolVar(&explicitLive, "explicit-live", false, "Acknowledge external provider execution")
	return cmd
}
