package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/release"
	"github.com/insajin/autopus-adk/pkg/qa/releasereadiness"
)

// qaReleaseReadinessOptions holds the flags for `auto qa release-readiness`.
type qaReleaseReadinessOptions struct {
	ProjectDir       string
	Approve          bool
	Decline          bool
	RuntimeProviders []string
	JSONOut          bool
	Format           string
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
	cmd.Flags().StringArrayVar(&opts.RuntimeProviders, "runtime-provider", nil, "Desktop observation runtime provider (local or orca; exactly one)")
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
	runtimeProvider, err := parseQARuntimeProvider(cmd, jsonMode, opts.RuntimeProviders)
	if err != nil {
		return err
	}
	if err := requireQARuntimeProvider(cmd, jsonMode, runtimeProvider, projectRequiresQARuntimeProvider(opts.ProjectDir)); err != nil {
		return err
	}

	payload, err := releasereadiness.Orchestrate(releasereadiness.Options{
		ProjectDir:      opts.ProjectDir,
		Approve:         opts.Approve,
		Decline:         opts.Decline,
		RuntimeProvider: runtimeProvider,
	})
	if err != nil {
		code := "qa_release_readiness_failed"
		if errors.Is(err, releasereadiness.ErrApprovalIntentConflict) {
			code = "qa_release_readiness_conflicting_intent"
		}
		if jsonMode {
			return writeJSONResultAndExit(cmd, jsonStatusError, err, code, nil, nil, nil)
		}
		return err
	}

	status := releaseReadinessStatus(payload)
	if jsonMode {
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	writeQAReleaseReadinessText(cmd, payload)
	return nil
}

// writeQAReleaseReadinessText renders the human summary. added/changed are what
// approval writes; unmatched packs are reported separately and explicitly as
// untouched so the line can never be read as a deletion proposal.
func writeQAReleaseReadinessText(cmd *cobra.Command, payload releasereadiness.Payload) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out,
		"qa release-readiness %s phase=%s surfaces=%s added=%d changed=%d files_written=%d lanes_executed=%d\n",
		payload.Verdict.Status, payload.Phase, surfaceList(payload.AnalyzedSurfaces),
		payload.Diff.AddedCount, payload.Diff.ChangedCount,
		payload.FilesWritten, payload.LanesExecuted)
	if payload.Diff.UnmatchedCount > 0 {
		fmt.Fprintf(out, "unmatched: %d existing pack(s) no analyzed surface accounts for; approval leaves them untouched\n",
			payload.Diff.UnmatchedCount)
	}
	for _, row := range payload.LaneRows {
		fmt.Fprintf(out, "lane: %s %s\n", row.Lane, laneRowDetail(row))
	}
	for _, next := range releaseReadinessNextCommands(payload) {
		fmt.Fprintf(out, "next: %s\n", next)
	}
}

func surfaceList(surfaces []string) string {
	if len(surfaces) == 0 {
		return "none"
	}
	return strings.Join(surfaces, ",")
}

func laneRowDetail(row releasereadiness.LaneRow) string {
	detail := row.Status
	if row.ReasonCode != "" {
		detail += " (" + row.ReasonCode + ")"
	}
	if row.FailureSummary != "" {
		detail += " " + row.FailureSummary
	}
	return detail
}

// releaseReadinessNextCommands names the one command that moves the operator
// forward from the phase they just reached, so the gate is discoverable from
// the output rather than only from the docs.
func releaseReadinessNextCommands(payload releasereadiness.Payload) []string {
	switch payload.Phase {
	case string(releasereadiness.PhaseDiffPresented):
		return []string{"auto qa release-readiness --approve", "auto qa release-readiness --decline"}
	case string(releasereadiness.PhaseAnalyzed):
		// Nothing was regenerable, so approving again changes nothing; the
		// actionable move is to run the packs the project already has.
		return []string{"auto qa run"}
	case string(releasereadiness.PhaseExecuted):
		return []string{"auto qa report"}
	}
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
