package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type workflowContextRuntimeCommandOptions struct {
	projectDir string
	format     string
}

type workflowContextRuntimePolicyOutput struct {
	Enabled bool                           `json:"enabled"`
	Policy  WorkflowContextEffectivePolicy `json:"policy"`
}

func newWorkflowContextRuntimeCmd() *cobra.Command {
	opts := workflowContextRuntimeCommandOptions{}
	cmd := &cobra.Command{
		Use:           "context-runtime",
		Short:         "Inspect the effective OMP context runtime supervisor policy",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.ToLower(strings.TrimSpace(opts.format)) != "json" {
				return fmt.Errorf("workflow context-runtime: unsupported format %q", opts.format)
			}
			cfg, err := loadHarnessConfigForDir(opts.projectDir, globalFlags{})
			if err != nil {
				return fmt.Errorf("workflow context-runtime: %w", err)
			}
			policy, enabled, err := workflowContextPolicyFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("workflow context-runtime: %w", err)
			}
			data, err := json.Marshal(workflowContextRuntimePolicyOutput{Enabled: enabled, Policy: policy})
			if err != nil {
				return fmt.Errorf("workflow context-runtime: encode policy: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.projectDir, "project-dir", ".", "Project root containing autopus.yaml")
	cmd.Flags().StringVar(&opts.format, "format", "json", "Output format (json)")
	return cmd
}
