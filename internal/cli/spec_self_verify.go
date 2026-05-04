package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func newSpecSelfVerifyCmd() *cobra.Command {
	var (
		specID    string
		dimension string
		status    string
		reason    string
	)

	cmd := &cobra.Command{
		Use:   "self-verify",
		Short: "Record SPEC self-verification results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if specID == "" {
				return fmt.Errorf("--record is required")
			}
			if dimension == "" {
				return fmt.Errorf("--dimension is required")
			}

			normalizedStatus := spec.ChecklistStatus(strings.ToUpper(strings.TrimSpace(status)))
			switch normalizedStatus {
			case spec.ChecklistStatusPass, spec.ChecklistStatusFail, spec.ChecklistStatusNA:
				// valid
			default:
				return fmt.Errorf("invalid --status %q: expected PASS, FAIL, or N/A", status)
			}

			resolved, err := spec.ResolveSpecDir(".", specID)
			if err != nil {
				return fmt.Errorf("SPEC 로드 실패: %w", err)
			}

			entry := spec.SelfVerifyEntry{
				Dimension: dimension,
				Status:    normalizedStatus,
				Reason:    strings.TrimSpace(reason),
			}
			if err := spec.AppendSelfVerifyEntry(resolved.SpecDir, entry); err != nil {
				return fmt.Errorf("self-verify 기록 실패: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "self-verify 기록 완료: %s\n", specID)
			return nil
		},
	}

	cmd.Flags().StringVar(&specID, "record", "", "SPEC ID to record")
	cmd.Flags().StringVar(&dimension, "dimension", "", "checklist dimension")
	cmd.Flags().StringVar(&status, "status", "", "result status (PASS or FAIL)")
	cmd.Flags().StringVar(&reason, "reason", "", "optional reason")

	return cmd
}
