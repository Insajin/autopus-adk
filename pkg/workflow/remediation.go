package workflow

// GateSignature represents the exit codes of a build/test run.
type GateSignature struct {
	BuildExit int
	TestExit  int
}

// GateRemediationDecision holds the verdict computed by RunGateRemediation.
type GateRemediationDecision struct {
	FixerAttempts    int
	SegmentBLaunched bool
	Aborted          bool
	AbortReason      string
}

// RunGateRemediation computes the bounded remediation decision for the gate.
func RunGateRemediation(budget int, evals []GateSignature) GateRemediationDecision {
	decision := GateRemediationDecision{}
	for i, ev := range evals {
		if ev.BuildExit == 0 && ev.TestExit == 0 {
			decision.SegmentBLaunched = true
			return decision
		}

		// Check circuit breaker (consecutive identical signatures)
		if i > 0 && evals[i] == evals[i-1] {
			decision.Aborted = true
			decision.AbortReason = "circuit_break_no_progress"
			return decision
		}

		// Check if we can spawn another fixer
		if decision.FixerAttempts >= budget {
			decision.Aborted = true
			decision.AbortReason = "budget_exhausted"
			return decision
		}

		decision.FixerAttempts++
	}
	return decision
}

// ConsolidatedVerdict holds the consolidated review status.
type ConsolidatedVerdict struct {
	Barrier bool
	Reason  string
}

// ConsolidateReviewVerdict combines reviewer and security-auditor votes.
// Security FAIL always outranks code-quality reviewer APPROVE.
func ConsolidateReviewVerdict(reviewerApprove, securityFail bool) ConsolidatedVerdict {
	if securityFail {
		return ConsolidatedVerdict{
			Barrier: true,
			Reason:  "security_fail",
		}
	}
	if !reviewerApprove {
		return ConsolidatedVerdict{
			Barrier: true,
			Reason:  "request_changes",
		}
	}
	return ConsolidatedVerdict{
		Barrier: false,
		Reason:  "",
	}
}

// ReviewBarrierDecision holds the outcome of the review barrier loop.
type ReviewBarrierDecision struct {
	FixerAttempts         int
	Aborted               bool
	AbortReason           string
	ReleaseHygieneReached bool
}

// RunReviewBarrier computes the bounded review barrier outcome.
func RunReviewBarrier(budget int, rounds []ConsolidatedVerdict) ReviewBarrierDecision {
	decision := ReviewBarrierDecision{}
	for _, round := range rounds {
		if !round.Barrier {
			decision.ReleaseHygieneReached = true
			return decision
		}

		if decision.FixerAttempts >= budget {
			decision.Aborted = true
			decision.AbortReason = "review_budget_exhausted"
			return decision
		}

		decision.FixerAttempts++
	}
	return decision
}
