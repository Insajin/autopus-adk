package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/selfupdate"
)

// reportSelfUpdateAvailability is the whole --check surface. The availability
// line must never stand alone: on a manager-owned install the CLI cannot perform
// the update it is announcing, and an operator who reads only the first line
// goes on to run `--self` against a binary its installer owns.
func reportSelfUpdateAvailability(
	cmd *cobra.Command,
	currentVersion string,
	info *selfupdate.ReleaseInfo,
) {
	if info == nil {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "업데이트 가능: v%s → %s\n", currentVersion, info.TagName)
	warnManagerOwnedSelfUpdate(cmd)
}

// warnManagerOwnedSelfUpdate tells a --check caller that this CLI cannot install
// the release it just advertised.
//
// The ownership gate lives in installSelfUpdateReleaseWithOperation, which
// --check returns before ever reaching, so a Homebrew or Autopus Desktop install
// reads a bare actionable "업데이트 가능" from the probe and then fails closed on
// the `--self` run it invited. Path resolution failures stay silent: a probe must
// not turn an unreadable executable path into a check error.
func warnManagerOwnedSelfUpdate(cmd *cobra.Command) {
	pathInfo, err := resolveCurrentBinaryPath()
	if err != nil || !pathInfo.IsManagerOwned() {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), managerRequiredUpdateError(pathInfo.ManagedPath()))
}
