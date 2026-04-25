package cli

import "github.com/spf13/cobra"

const legacyLocalHostWorkerNotice = "Legacy local-host worker mode only. Canonical installed desktop/runtime flows use Autopus Desktop or `autopus-desktop-runtime ...` commands."

func markLegacyLocalHostWorker(cmd *cobra.Command) {
	if cmd.Long == "" {
		cmd.Long = legacyLocalHostWorkerNotice
		return
	}
	cmd.Long = legacyLocalHostWorkerNotice + "\n\n" + cmd.Long
}
