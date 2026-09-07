package cli

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/spec"
)

func noProviderReviewsSucceeded(reviews []spec.ReviewResult, statuses []spec.ProviderStatus) bool {
	return len(reviews) == 0 && len(statuses) > 0 && spec.CountProviderStatus(statuses, "success") == 0
}

// resolveReviewVerdict decides the final verdict and states why it blocks.
// Every REVISE or REJECT it returns carries at least one blocking reason;
// conversely, a verdict whose findings are all advisory and whose reviewers
// raised no other blocker normalizes to PASS instead of leaving an unexplained
// REVISE behind (issue #187).
func resolveReviewVerdict(
	merged *spec.ReviewResult,
	reviews []spec.ReviewResult,
) (spec.ReviewVerdict, []spec.BlockingReason) {
	if len(reviews) == 0 {
		return spec.VerdictRevise, []spec.BlockingReason{{Policy: spec.BlockingPolicyNoUsableReview}}
	}
	reasons := spec.CollectBlockingReasons(merged.Findings)
	reasons = append(reasons, verdictScopedBlockingReasons(merged, reviews)...)
	if len(reasons) == 0 {
		return spec.VerdictPass, nil
	}
	if hasBlockingPolicy(reasons, spec.BlockingPolicyProviderReject, spec.BlockingPolicyJudgeReject) {
		return spec.VerdictReject, reasons
	}
	return spec.VerdictRevise, reasons
}

// verdictScopedBlockingReasons collects the blockers that are not tied to a
// merged finding. Judge precedence is preserved: when a valid judge produced the
// verdict, the raw reviewer votes are no longer consulted, so a provider REJECT
// the judge overruled cannot resurrect itself here.
func verdictScopedBlockingReasons(merged *spec.ReviewResult, reviews []spec.ReviewResult) []spec.BlockingReason {
	if judgeVerdictApplied(merged) {
		if strings.EqualFold(strings.TrimSpace(merged.Judge.Verdict), string(spec.VerdictReject)) {
			return []spec.BlockingReason{{Policy: spec.BlockingPolicyJudgeReject}}
		}
		return nil
	}
	for _, r := range reviews {
		if r.Verdict == spec.VerdictReject {
			return []spec.BlockingReason{{Policy: spec.BlockingPolicyProviderReject}}
		}
	}
	if merged.Verdict != spec.VerdictRevise {
		return nil
	}
	// A reviewer that voted REVISE on a failing checklist item states a blocker
	// the finding merge does not carry, so it must survive normalization.
	for _, r := range reviews {
		if r.Verdict == spec.VerdictRevise && reviewHasFailingChecklist(r.ChecklistOutcomes) {
			return []spec.BlockingReason{{Policy: spec.BlockingPolicyReviewerChecklist}}
		}
	}
	return nil
}

// judgeVerdictApplied reports whether the merged verdict came from a judge whose
// output parsed and validated.
func judgeVerdictApplied(merged *spec.ReviewResult) bool {
	return merged != nil && merged.Judge != nil && merged.Judge.Status == "ok"
}

func hasBlockingPolicy(reasons []spec.BlockingReason, policies ...string) bool {
	for _, reason := range reasons {
		for _, policy := range policies {
			if reason.Policy == policy {
				return true
			}
		}
	}
	return false
}

// reviewAwaitsAuthorChanges reports whether the previous revision ended on a
// blocker only the author can clear. Provider-infrastructure failures are
// excluded: re-running those can genuinely change the outcome, while
// re-reviewing byte-identical input against a stated content blocker cannot.
func reviewAwaitsAuthorChanges(result *spec.ReviewResult) bool {
	if result == nil || result.Verdict == spec.VerdictPass {
		return false
	}
	for _, reason := range result.BlockingReasons {
		if reason.Policy != spec.BlockingPolicyNoUsableReview {
			return true
		}
	}
	return false
}

func reviewHasFailingChecklist(outcomes []spec.ChecklistOutcome) bool {
	for _, outcome := range outcomes {
		if outcome.Status == spec.ChecklistStatusFail {
			return true
		}
	}
	return false
}

// buildPromptOpts builds ReviewPromptOptions for the current revision.
func buildPromptOpts(priorFindings []spec.ReviewFinding, revision int, specDir string, gate config.ReviewGateConf) spec.ReviewPromptOptions {
	opts := spec.ReviewPromptOptions{
		SpecDir:            specDir,
		PassCriteria:       gate.PassCriteria,
		DocContextMaxLines: gate.DocContextMaxLines,
	}
	if len(priorFindings) == 0 {
		opts.Mode = spec.ReviewModeDiscover
		return opts
	}
	opts.Mode = spec.ReviewModeVerify
	opts.PriorFindings = priorFindings
	return opts
}
