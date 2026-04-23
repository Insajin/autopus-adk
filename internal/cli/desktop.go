package cli

import "github.com/spf13/cobra"

func newDesktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Desktop-owned runtime commands",
		Long:  "Machine-oriented desktop runtime surfaces. Canonical packaged source/build ownership now lives in `autopus-desktop/runtime-helper`; this ADK surface remains for harness and compatibility flows.",
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
