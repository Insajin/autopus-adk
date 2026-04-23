package cli

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

func newDesktopSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Show desktop runtime session bootstrap as JSON",
		Long:  "Returns the desktop-owned runtime session bootstrap payload as machine-readable JSON. Packaged runtime source/build ownership now lives in `autopus-desktop/runtime-helper`; this ADK path is retained for compatibility.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return delegateRuntimeHelperStream(cmd, []string{"desktop", "session"})
		},
	}
}

func writeDesktopSessionJSON(out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(setup.LoadDesktopSession())
}
