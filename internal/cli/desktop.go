package cli

import "github.com/spf13/cobra"

func newDesktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Desktop-owned runtime commands",
		Long:  "Machine-oriented desktop runtime surfaces. Use these instead of the legacy `auto worker` JSON/bootstrap entrypoints.",
	}

	cmd.AddCommand(
		newWorkerStatusCmd(),
		newWorkerSessionCmd(),
		newWorkerSidecarCmd(),
	)

	return cmd
}
