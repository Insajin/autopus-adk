// Package cli implements the check command.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

func newCheckCmd() *cobra.Command {
	var (
		archFlag  bool
		loreFlag  bool
		quietFlag bool
		dir       string
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run harness rule checks",
		Long:  "하네스 규칙 검사를 수행합니다. hooks에서 자동 호출되며, 수동 실행도 가능합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("cannot get current directory: %w", err)
				}
			}

			out := cmd.OutOrStdout()
			if !quietFlag {
				tui.BannerWithInfo(out, "autopus-adk", "check")
			}

			allOK := runChecks(archFlag, loreFlag, dir, out, quietFlag)
			if !allOK {
				return fmt.Errorf("check failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&archFlag, "arch", false, "Check architecture rules (file size limit)")
	cmd.Flags().BoolVar(&loreFlag, "lore", false, "Check Lore commit format")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&dir, "dir", "", "Project root directory")

	return cmd
}

// runChecks executes the selected checks and returns true if all pass.
// If neither arch nor lore is selected, all checks run.
func runChecks(archFlag, loreFlag bool, dir string, out io.Writer, quiet bool) bool {
	runAll := !archFlag && !loreFlag
	allOK := true

	if archFlag || runAll {
		if !checkArch(dir, out, quiet) {
			allOK = false
		}
	}
	if loreFlag || runAll {
		if !checkLore(dir, out, quiet) {
			allOK = false
		}
	}
	return allOK
}
