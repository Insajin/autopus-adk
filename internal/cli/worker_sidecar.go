package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/host"
)

func newWorkerSidecarCmd() *cobra.Command {
	input := host.Input{}
	var desktopLaunchNonce string
	cmd := &cobra.Command{
		Use:   "sidecar",
		Short: "Start the worker sidecar (machine-readable NDJSON)",
		Long: `Starts the shared worker host in machine-oriented sidecar mode.
All stdout output is line-delimited NDJSON runtime events.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return host.RunSidecar(ctx, input, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&input.ConfigPath, "config", "", "Worker config path override")
	cmd.Flags().StringVar(&input.MCPConfigPath, "mcp-config", "", "MCP config path override")
	cmd.Flags().StringVar(&input.CredentialsPath, "credentials", "", "Credentials file path override")
	cmd.Flags().StringVar(&desktopLaunchNonce, "desktop-launch-nonce", "", "Desktop launch correlation nonce")
	return cmd
}
