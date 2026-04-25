package cli

import "github.com/spf13/cobra"

func newWorkerSidecarCmd() *cobra.Command {
	return newRuntimeSidecarCmd(
		"sidecar",
		"Start the worker sidecar (compatibility NDJSON surface)",
		"Compatibility shim for the desktop-owned sidecar surface. Prefer Autopus Desktop or `autopus-desktop-runtime desktop sidecar`.\nAll stdout output is line-delimited NDJSON runtime events.",
	)
}
