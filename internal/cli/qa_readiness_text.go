package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

// writeQAReadinessText renders the projection for a human. It follows the
// qa report / qa run convention: one key=value status line, then one prefixed
// line per detail row, then the `next:` hints. The projection carries lanes,
// setup gaps, evidence refs, and feedback actions, so a two-token
// "<verdict> <timestamp>" line threw away everything the command computed.
func writeQAReadinessText(cmd *cobra.Command, projection *readiness.Projection) {
	out := cmd.OutOrStdout()
	counts := projection.CheckCounts
	fmt.Fprintf(out,
		"qa readiness %s lanes=%d checks=%d passed=%d failed=%d skipped=%d blocked=%d setup_gaps=%d evidence=%d last_run=%s\n",
		projection.ReleaseVerdict, len(projection.Lanes), counts.Total, counts.Passed,
		counts.Failed, counts.Skipped, counts.Blocked, len(projection.SetupGaps),
		len(projection.EvidenceRefs), projection.LastRunTime)
	for _, lane := range projection.Lanes {
		fmt.Fprintf(out, "lane: %s %s\n", lane.Lane, lane.Status)
	}
	for _, gap := range projection.SetupGaps {
		fmt.Fprintf(out, "setup_gap: %s\n", setupGapDetail(gap))
	}
	for _, ref := range projection.EvidenceRefs {
		fmt.Fprintf(out, "evidence: %s\n", ref.ManifestPath)
	}
	if projection.TrendSummary != "" {
		fmt.Fprintf(out, "trend: %s\n", projection.TrendSummary)
	}
	for _, next := range readinessNextCommands(projection) {
		fmt.Fprintf(out, "next: %s\n", next)
	}
}

func setupGapDetail(gap readiness.SetupGap) string {
	detail := gap.Class
	if gap.Lane != "" {
		detail = gap.Lane + " " + detail
	}
	if gap.Reason != "" {
		detail += " (" + gap.Reason + ")"
	}
	return detail
}

// readinessNextCommands names the commands that act on what the projection just
// reported: the visual report always, and the repair handoff only when the
// projection itself says the action is enabled.
func readinessNextCommands(projection *readiness.Projection) []string {
	next := []string{"auto qa report"}
	for _, action := range projection.FeedbackActions {
		if action.Enabled && action.CommandDisplay != "" {
			next = append(next, action.CommandDisplay)
		}
	}
	return next
}
