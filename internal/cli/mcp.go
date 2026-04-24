package cli

import "github.com/spf13/cobra"

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Desktop and developer MCP integration commands",
		Long:  "Desktop/runtime MCP command surface. Packaged runtime source/build ownership now lives in `autopus-desktop/runtime-helper`; this ADK path is retained for compatibility and harness flows.",
	}

	cmd.AddCommand(newMCPServerCmd())
	return cmd
}

func newMCPServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve"},
		Short:   "Run the Autopus MCP server over stdio",
		Long:    "Delegates stdio MCP serving to the desktop-owned runtime helper. Canonical packaged runtime ownership lives in `autopus-desktop/runtime-helper`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeMCPServe(cmd)
		},
	}
}
