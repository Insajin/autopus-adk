package cli

import (
	"github.com/spf13/cobra"
)

func newDesktopStatusCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show desktop runtime readiness",
		Long:  "Desktop-owned runtime readiness surface. Use `--json` for the canonical machine-readable contract. For installed desktop operation, prefer the desktop app status action or `autopus-desktop-runtime desktop status`; this ADK path is retained as a migration shim for compatibility and harness verification.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}
			helperArgs := []string{"desktop", "status"}
			helperArgs = appendBoolFlag(helperArgs, "json", jsonMode)
			helperArgs = appendStringFlag(helperArgs, "format", format, cmd.Flags().Changed("format"))
			if jsonMode {
				return delegateRuntimeHelperJSON(cmd, helperArgs)
			}
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}
