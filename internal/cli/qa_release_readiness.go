package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/release"
	"github.com/insajin/autopus-adk/pkg/qa/releasereadiness"
)

// qaReleaseReadinessOptions holds the flags for `auto qa release-readiness`.
type qaReleaseReadinessOptions struct {
	ProjectDir string
	Approve    bool
	Decline    bool
	JSONOut    bool
	Format     string
}

// newQAReleaseReadinessCmd constructs the explicit `auto qa release-readiness`
// command. The user-invoked command is the only entry point into the
// release-readiness flow: there is deliberately no init(), scheduler, hook, or
// cron registration, so the orchestration never runs implicitly (AC-006).
func newQAReleaseReadinessCmd() *cobra.Command {
	var opts qaReleaseReadinessOptions
	cmd := &cobra.Command{
		Use:   "release-readiness",
		Short: "Re-synthesize project-local Journey Packs to match current code, show a deterministic diff, gate on one approval, then dispatch cross-surface execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAReleaseReadiness(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory to analyze")
	cmd.Flags().BoolVar(&opts.Approve, "approve", false, "Approve the diff, persist regenerated packs, and dispatch cross-surface execution")
	cmd.Flags().BoolVar(&opts.Decline, "decline", false, "Explicitly decline the diff (no write, no execution)")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

// runQAReleaseReadiness resolves output mode, runs the orchestration, and emits
// either the JSON envelope or a concise human summary line.
func runQAReleaseReadiness(cmd *cobra.Command, opts qaReleaseReadinessOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}

	payload, err := releasereadiness.Orchestrate(releasereadiness.Options{
		ProjectDir: opts.ProjectDir,
		Approve:    opts.Approve,
		Decline:    opts.Decline,
	})
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, "qa_release_readiness_failed", nil, nil, nil)
		}
		return err
	}

	status := releaseReadinessStatus(payload)
	if jsonMode {
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"%s phase=%s added=%d changed=%d removed=%d files_written=%d lanes_executed=%d verdict=%s\n",
		payload.SchemaVersion, payload.Phase,
		payload.Diff.AddedCount, payload.Diff.ChangedCount, payload.Diff.RemovedCount,
		payload.FilesWritten, payload.LanesExecuted, payload.Verdict.Status)
	return nil
}

// releaseReadinessStatus maps a payload to a JSON envelope status. Non-executed
// phases (diff presented, declined, analyzed) are always OK because they are
// gate-pending, not failures. An executed run with a non-passing verdict
// surfaces as WARN so the deterministic gate decision is visible.
func releaseReadinessStatus(payload releasereadiness.Payload) jsonEnvelopeStatus {
	if payload.Phase == string(releasereadiness.PhaseExecuted) &&
		payload.Verdict.Status != string(release.GateStatusPassed) {
		return jsonStatusWarn
	}
	return jsonStatusOK
}
