package cli

import "github.com/spf13/cobra"

func newDesktopSidecarCmd() *cobra.Command {
	return newRuntimeSidecarCmd(
		"sidecar",
		"Start the desktop runtime sidecar (machine-readable NDJSON)",
		"Starts the shared desktop runtime host in machine-oriented sidecar mode.\nAll stdout output is line-delimited NDJSON runtime events.\nFor installed desktop operation, prefer the desktop app sidecar/runtime action or `autopus-desktop-runtime desktop sidecar`; this ADK path is retained as a migration shim for compatibility and harness verification.",
	)
}

func newRuntimeSidecarCmd(use, short, long string) *cobra.Command {
	var configPath string
	var mcpConfigPath string
	var credentialsPath string
	var desktopLaunchNonce string

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			helperArgs := []string{"desktop", "sidecar"}
			helperArgs = appendStringFlag(helperArgs, "config", configPath, cmd.Flags().Changed("config"))
			helperArgs = appendStringFlag(helperArgs, "mcp-config", mcpConfigPath, cmd.Flags().Changed("mcp-config"))
			helperArgs = appendStringFlag(helperArgs, "credentials", credentialsPath, cmd.Flags().Changed("credentials"))
			helperArgs = appendStringFlag(helperArgs, "desktop-launch-nonce", desktopLaunchNonce, cmd.Flags().Changed("desktop-launch-nonce"))
			return delegateRuntimeHelperStream(cmd, helperArgs)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Worker config path override")
	cmd.Flags().StringVar(&mcpConfigPath, "mcp-config", "", "MCP config path override")
	cmd.Flags().StringVar(&credentialsPath, "credentials", "", "Credentials file path override")
	cmd.Flags().StringVar(&desktopLaunchNonce, "desktop-launch-nonce", "", "Desktop launch correlation nonce")
	return cmd
}
