package workflow

import "testing"

func TestRunGateRemediation_S1(t *testing.T) {
	evals := []GateSignature{
		{BuildExit: 1, TestExit: 0}, // initial fail
		{BuildExit: 0, TestExit: 1}, // different signature fail
		{BuildExit: 0, TestExit: 0}, // pass
	}

	dec := RunGateRemediation(2, evals)
	if dec.FixerAttempts != 2 {
		t.Errorf("expected FixerAttempts 2, got %d", dec.FixerAttempts)
	}
	if !dec.SegmentBLaunched {
		t.Errorf("expected SegmentBLaunched true, got false")
	}
	if dec.Aborted {
		t.Errorf("expected Aborted false, got true")
	}
}

func TestRunGateRemediation_S2(t *testing.T) {
	evals := []GateSignature{
		{BuildExit: 2, TestExit: 0}, // initial fail
		{BuildExit: 2, TestExit: 0}, // same signature consecutive fail
	}

	dec := RunGateRemediation(3, evals)
	if !dec.Aborted {
		t.Errorf("expected Aborted true, got false")
	}
	if dec.AbortReason != "circuit_break_no_progress" {
		t.Errorf("expected AbortReason 'circuit_break_no_progress', got '%s'", dec.AbortReason)
	}
	if dec.FixerAttempts != 1 {
		t.Errorf("expected FixerAttempts 1, got %d", dec.FixerAttempts)
	}
	if dec.SegmentBLaunched {
		t.Errorf("expected SegmentBLaunched false, got true")
	}
}

func TestRunReviewBarrier_S5(t *testing.T) {
	rounds := []ConsolidatedVerdict{
		{Barrier: true, Reason: "request_changes"},
		{Barrier: true, Reason: "request_changes"},
		{Barrier: true, Reason: "request_changes"},
	}

	dec := RunReviewBarrier(2, rounds)
	if dec.FixerAttempts != 2 {
		t.Errorf("expected FixerAttempts 2, got %d", dec.FixerAttempts)
	}
	if !dec.Aborted {
		t.Errorf("expected Aborted true, got false")
	}
	if dec.AbortReason != "review_budget_exhausted" {
		t.Errorf("expected AbortReason 'review_budget_exhausted', got '%s'", dec.AbortReason)
	}
	if dec.ReleaseHygieneReached {
		t.Errorf("expected ReleaseHygieneReached false, got true")
	}
}

func TestConsolidateReviewVerdict_S6(t *testing.T) {
	// reviewer Approve, security Fail
	v := ConsolidateReviewVerdict(true, true)
	if !v.Barrier {
		t.Errorf("expected Barrier true, got false")
	}
	if v.Reason != "security_fail" {
		t.Errorf("expected Reason 'security_fail', got '%s'", v.Reason)
	}

	// reviewer Request Changes, security Pass
	v2 := ConsolidateReviewVerdict(false, false)
	if !v2.Barrier {
		t.Errorf("expected Barrier true, got false")
	}
	if v2.Reason != "request_changes" {
		t.Errorf("expected Reason 'request_changes', got '%s'", v2.Reason)
	}

	// reviewer Approve, security Pass
	v3 := ConsolidateReviewVerdict(true, false)
	if v3.Barrier {
		t.Errorf("expected Barrier false, got true")
	}
	if v3.Reason != "" {
		t.Errorf("expected Reason '', got '%s'", v3.Reason)
	}
}
