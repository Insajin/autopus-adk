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
		archFlag     bool
		loreFlag     bool
		quietFlag    bool
		warnOnlyFlag bool
		stagedFlag   bool
		messageFlag  string
		gateFlag     string
		dir          string
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

			if gateFlag != "" {
				mode := GateModeMandatory
				if warnOnlyFlag {
					mode = GateModeAdvisory
				}
				result := GateCheck(GateConfig{
					GateName: gateFlag,
					Mode:     mode,
					Dir:      dir,
				})
				if result.Err != nil {
					return result.Err
				}
				if result.Warning != "" {
					fmt.Fprintln(out, "Warning:", result.Warning)
				}
				if !result.Passed {
					return fmt.Errorf("%s", result.Message)
				}
				return nil
			}

			allOK := runChecks(archFlag, loreFlag, dir, out, quietFlag, warnOnlyFlag, stagedFlag, messageFlag)
			if !allOK {
				return fmt.Errorf("check failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&archFlag, "arch", false, "Check architecture rules (file size limit)")
	cmd.Flags().BoolVar(&loreFlag, "lore", false, "Check Lore commit format")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&warnOnlyFlag, "warn-only", false, "Exit 0 even if checks fail (advisory mode)")
	cmd.Flags().BoolVar(&stagedFlag, "staged", false, "Check only git-staged files (for pre-commit hook)")
	cmd.Flags().StringVar(&messageFlag, "message", "", "Commit message file path (for commit-msg hook)")
	cmd.Flags().StringVar(&gateFlag, "gate", "", "Run a named gate check (e.g. phase2)")
	cmd.Flags().StringVar(&dir, "dir", "", "Project root directory")

	return cmd
}

// runChecks executes the selected checks and returns true if all pass.
// If neither arch nor lore is selected, all checks run.
// When warnOnly is true, violations are still printed but the function always returns true.
// When staged is true, arch check only examines git-staged files.
// When messageFile is non-empty, lore check validates that file instead of the last commit.
func runChecks(archFlag, loreFlag bool, dir string, out io.Writer, quiet, warnOnly, staged bool, messageFile string) bool {
	runAll := !archFlag && !loreFlag
	allOK := true

	if archFlag || runAll {
		if !checkArch(dir, out, quiet, staged) {
			allOK = false
		}
	}
	if loreFlag || runAll {
		if messageFile != "" {
			if !checkLoreFromFile(messageFile, out, quiet) {
				allOK = false
			}
		} else {
			if !checkLore(dir, out, quiet) {
				allOK = false
			}
		}
	}

	if warnOnly {
		return true
	}
	return allOK
}
