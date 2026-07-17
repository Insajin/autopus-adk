// Package cli implements the check command.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/evalregression"
)

func newCheckCmd() *cobra.Command {
	var (
		archFlag            bool
		cc21Flag            bool
		loreFlag            bool
		hygieneFlag         bool
		initialPromptFlag   bool
		monitorCommandsFlag bool
		quietFlag           bool
		warnOnlyFlag        bool
		stagedFlag          bool
		messageFlag         string
		gateFlag            string
		dir                 string

		evalRegressionFlag                          bool
		evalRegressionArtifactFlag                  string
		evalRegressionAttestationFlag               string
		evalRegressionMaxAgeFlag                    time.Duration
		evalRegressionExpectedKeyIDFlag             string
		evalRegressionExpectedTrustLaneFlag         string
		evalRegressionExpectedSourceEnvironmentFlag string
		evalRegressionExpectedTargetEnvironmentFlag string
		evalRegressionExpectedSourceRevisionFlag    string
		evalRegressionExpectedWorkspaceScopeFlag    string
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
			policy := evalregression.EvalRegressionAttestationPolicyV2{
				ExpectedKeyID:     evalRegressionExpectedKeyIDFlag,
				TrustLane:         evalRegressionExpectedTrustLaneFlag,
				SourceEnvironment: evalRegressionExpectedSourceEnvironmentFlag,
				TargetEnvironment: evalRegressionExpectedTargetEnvironmentFlag,
				SourceRevision:    evalRegressionExpectedSourceRevisionFlag,
				WorkspaceScope:    evalRegressionExpectedWorkspaceScopeFlag,
			}
			if evalRegressionFlag {
				if reason, ok := evalregression.ValidateEvalRegressionAttestationPolicyV2(policy); !ok {
					if !quietFlag {
						fmt.Fprintln(out, "eval-regression: "+reason)
					}
					return fmt.Errorf("check failed")
				}
				if gateFlag != "" {
					return fmt.Errorf("--eval-regression cannot be combined with --gate")
				}
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
			allOK := runChecks(flags, archFlag, cc21Flag, loreFlag, hygieneFlag, initialPromptFlag, monitorCommandsFlag, evalRegressionFlag, evalRegressionArtifactFlag, evalRegressionAttestationFlag, evalRegressionMaxAgeFlag, policy, dir, out, quietFlag, warnOnlyFlag, stagedFlag, messageFlag)
			if !allOK {
				return fmt.Errorf("check failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&archFlag, "arch", false, "Check architecture rules (file size limit)")
	cmd.Flags().BoolVar(&cc21Flag, "cc21", false, "Run SPEC-CC21-001 checks (effort frontmatter + initialPrompt guard)")
	cmd.Flags().BoolVar(&loreFlag, "lore", false, "Check Lore commit format")
	cmd.Flags().BoolVar(&hygieneFlag, "hygiene", false, "Check staged generated/runtime drift")
	cmd.Flags().BoolVar(&initialPromptFlag, "initial-prompt-guard", false, "Check subagent files for forbidden initialPrompt field (SPEC-CC21-001 R11b)")
	cmd.Flags().BoolVar(&monitorCommandsFlag, "monitor-commands", false, "Lint Monitor commands for line-buffered grep guards")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&warnOnlyFlag, "warn-only", false, "Exit 0 even if checks fail (advisory mode)")
	cmd.Flags().BoolVar(&stagedFlag, "staged", false, "Check only git-staged files (for pre-commit hook)")
	cmd.Flags().StringVar(&messageFlag, "message", "", "Commit message file path (for commit-msg hook)")
	cmd.Flags().StringVar(&gateFlag, "gate", "", "Run a named gate check (e.g. phase2)")
	cmd.Flags().StringVar(&dir, "dir", "", "Project root directory")
	cmd.Flags().BoolVar(&evalRegressionFlag, "eval-regression", false, "Fail closed on a blocked/missing/stale/unsafe eval_regression_report.v1 artifact (SPEC-EVAL-REGRESSION-CI-001)")
	cmd.Flags().StringVar(&evalRegressionArtifactFlag, "eval-regression-artifact", "", "Path to the eval_regression_report.v1 artifact")
	cmd.Flags().StringVar(&evalRegressionAttestationFlag, "eval-regression-attestation", "", "Path to the eval_regression_attestation.v2 sidecar (defaults to a path derived from the artifact)")
	cmd.Flags().DurationVar(&evalRegressionMaxAgeFlag, "eval-regression-max-age", 24*time.Hour, "Freshness window for the eval-regression artifact")
	cmd.Flags().StringVar(&evalRegressionExpectedKeyIDFlag, "eval-regression-expected-key-id", "", "Required trusted key ID for the eval-regression attestation")
	cmd.Flags().StringVar(&evalRegressionExpectedTrustLaneFlag, "eval-regression-expected-trust-lane", "", "Required trust lane for the eval-regression attestation")
	cmd.Flags().StringVar(&evalRegressionExpectedSourceEnvironmentFlag, "eval-regression-expected-source-environment", "", "Required source environment for the eval-regression attestation")
	cmd.Flags().StringVar(&evalRegressionExpectedTargetEnvironmentFlag, "eval-regression-expected-target-environment", "", "Required target environment for the eval-regression attestation")
	cmd.Flags().StringVar(&evalRegressionExpectedSourceRevisionFlag, "eval-regression-expected-source-revision", "", "Required source revision for the eval-regression attestation")
	cmd.Flags().StringVar(&evalRegressionExpectedWorkspaceScopeFlag, "eval-regression-expected-workspace-scope", "", "Required workspace scope for the eval-regression attestation")

	return cmd
}

// runChecks executes the selected checks and returns true if all pass.
// If no specific check flag is set, all checks run.
// When warnOnly is true, violations are advisory except an invalid v2 trust
// policy, which remains mandatory and fail-closed.
// When staged is true, arch check only examines git-staged files.
// When messageFile is non-empty, lore check validates that file instead of the last commit.
func runChecks(flags globalFlags, archFlag, cc21Flag, loreFlag, hygieneFlag, initialPromptFlag, monitorCommandsFlag, evalRegressionFlag bool, evalRegressionArtifact, evalRegressionAttestation string, evalRegressionMaxAge time.Duration, evalRegressionPolicy evalregression.EvalRegressionAttestationPolicyV2, dir string, out io.Writer, quiet, warnOnly, staged bool, messageFile string) bool {
	runAll := !archFlag && !cc21Flag && !loreFlag && !hygieneFlag && !initialPromptFlag && !monitorCommandsFlag && !evalRegressionFlag
	allOK := true
	evalRegressionPolicyOK := true

	if hygieneFlag || runAll {
		if !checkHygiene(dir, out, quiet) {
			allOK = false
		}
	}
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
	if evalRegressionFlag {
		if _, ok := evalregression.ValidateEvalRegressionAttestationPolicyV2(evalRegressionPolicy); !ok {
			evalRegressionPolicyOK = false
		}
		attestationPath := evalRegressionAttestation
		if strings.TrimSpace(attestationPath) == "" {
			attestationPath = deriveEvalRegressionAttestationPath(evalRegressionArtifact)
		}
		if !checkEvalRegressionStrict(dir, evalRegressionArtifact, attestationPath, evalRegressionMaxAge, time.Now(), evalregression.CommittedEvalRegressionPublicKeys(), evalRegressionPolicy, out, quiet, warnOnly) {
			allOK = false
		}
	}

	if warnOnly && evalRegressionPolicyOK {
		return true
	}
	return allOK
}
