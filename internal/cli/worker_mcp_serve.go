package cli

import "github.com/spf13/cobra"

func newWorkerMCPServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "mcp-server",
		Aliases: []string{"mcp-serve"},
		Hidden:  true,
		Short:   "Run the worker MCP server over stdio",
		Long:    "Compatibility shim for the legacy worker-owned MCP surface. Prefer `auto mcp server`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeMCPServe(cmd)
		},
	}
}
