package cli

import (
	"github.com/spf13/cobra"
)

func newConnectStatusCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show connect readiness and remaining manual steps",
		Long:  "Migration compatibility wrapper for the desktop-owned connect status surface. For installed desktop operation, use the desktop app status action or `autopus-desktop-runtime connect status`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}
			helperArgs := []string{"connect", "status"}
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
