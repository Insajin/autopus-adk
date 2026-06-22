package cli

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// NewWorkflowCmd builds the `auto workflow` command tree (doctor/render/gate).
// A nil prober or runner selects the production default; tests inject fakes via
// this exported constructor to keep the seams hermetic.
func NewWorkflowCmd(prober workflow.Prober, runner workflow.CommandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "workflow",
		Short:         "Inspect and gate the opt-in deterministic workflow route",
		Long:          "Commands for the claude-scoped `/auto go --workflow` Route A: doctor capability gate, dry-run render, and the deterministic gate JS->Go bridge.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newWorkflowDoctorCmd(prober))
	cmd.AddCommand(newWorkflowGateCmd(runner))
	cmd.AddCommand(newWorkflowRenderCmd())
	cmd.AddCommand(newWorkflowMergeCmd())
	return cmd
}

// newWorkflowCmd is the production registration entry point used by root.go.
func newWorkflowCmd() *cobra.Command {
	return NewWorkflowCmd(nil, nil)
}

// newWorkflowDoctorCmd runs the capability gate and exits non-zero when the
// overall verdict is fail (S4/S12), zero when it passes (S14).
func newWorkflowDoctorCmd(prober workflow.Prober) *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "Probe workflow capabilities and version pin (hard-gates required primitives)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := prober
			if p == nil {
				p = newLiveProber()
			}
			report := workflow.EvaluateCapabilities(p)
			data, err := report.EncodeJSON()
			if err != nil {
				return fmt.Errorf("encode capability report: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			if report.Overall == workflow.OverallFail {
				return fmt.Errorf("workflow doctor: capability gate failed (overall=fail)")
			}
			return nil
		},
	}
}

// liveProber is the production capability prober. Without access to the closed
// claude-code Workflow primitive registry, it infers the availability of the
// claude-scoped primitives from the claude binary presence and reads the
// version from `claude --version`. This is a defensible heuristic for the
// hard-gate and is documented as such.
type liveProber struct {
	version string
	present bool
}

func newLiveProber() liveProber {
	path, err := exec.LookPath("claude")
	if err != nil {
		return liveProber{}
	}
	out, err := exec.Command(path, "--version").Output() //nolint:gosec // fixed binary, no user input
	if err != nil {
		return liveProber{present: true}
	}
	return liveProber{version: parseClaudeVersion(string(out)), present: true}
}

func (p liveProber) Version() string { return p.version }

func (p liveProber) Probe(string) bool { return p.present }

var versionTokenRe = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func parseClaudeVersion(out string) string {
	if m := versionTokenRe.FindString(out); m != "" {
		return m
	}
	return ""
}

// execCommandRunner is the production CommandRunner: it runs the command and
// returns the process exit code.
type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // commands come from operator flags
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode(), err
	}
	return 1, err
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
