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
		archFlag            bool
		cc21Flag            bool
		loreFlag            bool
		initialPromptFlag   bool
		monitorCommandsFlag bool
		quietFlag           bool
		warnOnlyFlag        bool
		stagedFlag          bool
		messageFlag         string
		gateFlag            string
		dir                 string
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

			flags := globalFlagsFromContext(cmd.Context())
			allOK := runChecks(flags, archFlag, cc21Flag, loreFlag, initialPromptFlag, monitorCommandsFlag, dir, out, quietFlag, warnOnlyFlag, stagedFlag, messageFlag)
			if !allOK {
				return fmt.Errorf("check failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&archFlag, "arch", false, "Check architecture rules (file size limit)")
	cmd.Flags().BoolVar(&cc21Flag, "cc21", false, "Run SPEC-CC21-001 checks (effort frontmatter + initialPrompt guard)")
	cmd.Flags().BoolVar(&loreFlag, "lore", false, "Check Lore commit format")
	cmd.Flags().BoolVar(&initialPromptFlag, "initial-prompt-guard", false, "Check subagent files for forbidden initialPrompt field (SPEC-CC21-001 R11b)")
	cmd.Flags().BoolVar(&monitorCommandsFlag, "monitor-commands", false, "Lint Monitor commands for line-buffered grep guards")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&warnOnlyFlag, "warn-only", false, "Exit 0 even if checks fail (advisory mode)")
	cmd.Flags().BoolVar(&stagedFlag, "staged", false, "Check only git-staged files (for pre-commit hook)")
	cmd.Flags().StringVar(&messageFlag, "message", "", "Commit message file path (for commit-msg hook)")
	cmd.Flags().StringVar(&gateFlag, "gate", "", "Run a named gate check (e.g. phase2)")
	cmd.Flags().StringVar(&dir, "dir", "", "Project root directory")

	return cmd
}

// runChecks executes the selected checks and returns true if all pass.
// If no specific check flag is set, all checks run.
// When warnOnly is true, violations are still printed but the function always returns true.
// When staged is true, arch check only examines git-staged files.
// When messageFile is non-empty, lore check validates that file instead of the last commit.
func runChecks(flags globalFlags, archFlag, cc21Flag, loreFlag, initialPromptFlag, monitorCommandsFlag bool, dir string, out io.Writer, quiet, warnOnly, staged bool, messageFile string) bool {
	runAll := !archFlag && !cc21Flag && !loreFlag && !initialPromptFlag && !monitorCommandsFlag
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
	if cc21Flag {
		if !checkAgentEffort(dir, out, quiet) {
			allOK = false
		}
		if !checkEffortRuntime(flags, out, quiet) {
			allOK = false
		}
		if !checkTaskCreatedModePrecedence(dir, flags, out, quiet) {
			allOK = false
		}
		if !checkInitialPrompt(dir, out, quiet) {
			allOK = false
		}
		if !checkMonitorCommands(dir, out, quiet) {
			allOK = false
		}
	}
	if initialPromptFlag || runAll {
		if !checkInitialPrompt(dir, out, quiet) {
			allOK = false
		}
	}
	if monitorCommandsFlag {
		if !checkMonitorCommands(dir, out, quiet) {
			allOK = false
		}
	}

	if warnOnly {
		return true
	}
	return allOK
}
