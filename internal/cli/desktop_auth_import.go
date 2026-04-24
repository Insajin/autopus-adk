package cli

import "github.com/spf13/cobra"

func newDesktopAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "auth",
		Short:  "Desktop auth helper surfaces",
		Hidden: true,
	}
	cmd.AddCommand(newDesktopAuthImportCmd())
	return cmd
}

func newDesktopAuthImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "import",
		Short:  "Persist desktop runtime auth payload from stdin JSON",
		Long:   "Hidden compatibility shim for the desktop-owned runtime auth import surface.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return delegateRuntimeHelperStream(cmd, []string{"desktop", "auth", "import"})
		},
	}
}
