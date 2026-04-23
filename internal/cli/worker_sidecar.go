package cli

import "github.com/spf13/cobra"

func newWorkerSidecarCmd() *cobra.Command {
	return newRuntimeSidecarCmd(
		"sidecar",
		"Start the worker sidecar (compatibility NDJSON surface)",
		"Compatibility shim for the desktop-owned sidecar surface. Prefer `auto desktop sidecar`.\nAll stdout output is line-delimited NDJSON runtime events.",
	)
}
