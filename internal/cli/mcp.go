package cli

import "github.com/spf13/cobra"

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Desktop and developer MCP integration commands",
		Long:  "Desktop/runtime MCP command surface. The desktop app and `autopus-desktop-runtime mcp ...` are canonical; this ADK `auto mcp ...` surface is retained as a migration shim for compatibility and harness flows.",
	}

	cmd.AddCommand(newMCPServerCmd())
	return cmd
}

func newMCPServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve"},
		Short:   "Run the Autopus MCP server over stdio",
		Long:    "Delegates stdio MCP serving to the desktop-owned runtime helper. For installed desktop operation, prefer the desktop app MCP action or `autopus-desktop-runtime mcp server`; this ADK command is a migration shim.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeMCPServe(cmd)
		},
	}
}
