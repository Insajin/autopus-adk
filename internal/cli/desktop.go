package cli

import "github.com/spf13/cobra"

func newDesktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Desktop-owned runtime commands",
		Long:  "Machine-oriented desktop runtime surfaces. The desktop app and `autopus-desktop-runtime ...` are canonical; this ADK `auto desktop ...` surface is retained as a migration shim for compatibility and harness flows.",
	}

	cmd.AddCommand(
		newDesktopAuthCmd(),
		newDesktopStatusCmd(),
		newDesktopSessionCmd(),
		newDesktopSidecarCmd(),
		newDesktopEnsureCmd(),
	)

	return cmd
}
