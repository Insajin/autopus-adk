package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newTelemetryRecordCmd creates `auto telemetry record` — an internal command
// used by agents to record pipeline, phase, agent-run, and lead-time
// telemetry events. Every invocation is a separate process, so each action
// appends its events and exits.
func newTelemetryRecordCmd() *cobra.Command {
	var p recordParams

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a telemetry event (internal agent use)",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("telemetry record: get cwd: %w", err)
			}
			return runTelemetryRecord(baseDir, p)
		},
	}

	cmd.Flags().StringVar(&p.specID, "spec-id", "", "SPEC identifier")
	cmd.Flags().StringVar(&p.agent, "agent", "", "Agent name")
	cmd.Flags().StringVar(&p.phase, "phase", "", "Phase name")
	cmd.Flags().StringSliceVar(&p.dependsOn, "depends-on", nil, "Phases this phase waits for (with --phase on start|agent)")
	cmd.Flags().StringVar(&p.action, "action", "", "Action: start | agent | end | milestone | action | defect | gate | estimate")
	cmd.Flags().StringVar(&p.status, "status", "PASS", "Status: PASS or FAIL")
	cmd.Flags().IntVar(&p.files, "files", 0, "Number of files modified (agent) or touched (defect)")
	cmd.Flags().IntVar(&p.tokens, "tokens", 0, "Estimated token count")
	cmd.Flags().StringVar(&p.qualityMode, "quality-mode", "balanced", "Quality mode (ultra|balanced)")
	cmd.Flags().StringVar(&p.usageJSON, "usage-json", "", "Path to a normalized usage envelope JSON file")
	cmd.Flags().StringVar(&p.acceptanceStatus, "acceptance-status", "", "Objective acceptance status: PASS or FAIL")

	cmd.Flags().StringVar(&p.name, "name", "", "Milestone name (first_vertical_slice or custom)")
	cmd.Flags().StringVar(&p.kind, "kind", "", "Action kind: reread | rerun")
	cmd.Flags().StringVar(&p.target, "target", "", "Action target: path or command")
	cmd.Flags().StringVar(&p.reason, "reason", "", "Action reason")
	cmd.Flags().StringVar(&p.defectID, "id", "", "Defect identifier")
	cmd.Flags().StringVar(&p.discoveredPhase, "discovered-phase", "", "Phase that discovered the defect")
	cmd.Flags().StringVar(&p.fixedPhase, "fixed-phase", "", "Phase that fixed the defect")
	cmd.Flags().BoolVar(&p.escaped, "escaped", false, "Defect escaped the phase that should have caught it")
	cmd.Flags().BoolVar(&p.repeat, "repeat", false, "Defect repeats a finding already reported on identical inputs")
	cmd.Flags().StringVar(&p.gate, "gate", "", "Gate identifier")
	cmd.Flags().StringVar(&p.applicability, "applicability", "", "Gate applicability: required | reusable | not_applicable | blocked")
	cmd.Flags().BoolVar(&p.resolved, "resolved", false, "Gate evidence produced or reused")
	cmd.Flags().DurationVar(&p.estimateMin, "min", 0, "Estimated minimum lead time (e.g. 30m)")
	cmd.Flags().DurationVar(&p.estimateMax, "max", 0, "Estimated maximum lead time (e.g. 2h)")

	return cmd
}
