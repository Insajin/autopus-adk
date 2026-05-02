package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newQACmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "qa",
		Short:         "Normalize QA evidence and generate repair prompts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newQAEvidenceCmd())
	cmd.AddCommand(newQAFeedbackCmd())
	return cmd
}

func requireFlag(name, value string) error {
	if value == "" {
		return fmt.Errorf("missing --%s", name)
	}
	return nil
}
