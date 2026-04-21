package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// @AX:NOTE [AUTO] subcommand registration point for "auto permission" — extend here to add new permission subcommands
func newPermissionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Manage permission detection",
	}
	cmd.AddCommand(newPermissionDetectCmd())
	return cmd
}

func newPermissionDetectCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect parent process permission mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			result := detect.DetectPermissionMode()
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), result.Mode)
			}
			return nil
		},
	}
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}
