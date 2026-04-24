package cli

import "github.com/spf13/cobra"

func runRuntimeMCPServe(cmd *cobra.Command) error {
	return delegateRuntimeHelperStream(cmd, []string{"mcp", "server"})
}
