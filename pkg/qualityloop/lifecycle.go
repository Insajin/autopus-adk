package qualityloop

func TransitionLifecycle(candidate ImprovementCandidate, signal LifecycleSignal) ImprovementCandidate {
	next := candidate
	next.Active = false
	next.ProviderWriteCallCount = 0
	next.RawPayloadPresent = false
	replayRefs, replayReasons := safeCandidateRefs(next.WorkspaceID, signal.ReplayEvidenceRefs)
	postApplyRefs, postApplyReasons := safeCandidateRefs(next.WorkspaceID, signal.PostApplyEvidenceRefs)
	next.ReplayEvidenceRefs = appendUniqueMany(next.ReplayEvidenceRefs, replayRefs)
	next.ApprovalRefs = appendUniqueMany(next.ApprovalRefs, signal.HumanApprovalRefs)
	if len(replayReasons) > 0 || len(postApplyReasons) > 0 {
		next.Status = StatusQuarantined
		next.SafetyReasonCodes = appendUniqueMany(next.SafetyReasonCodes, append(replayReasons, postApplyReasons...))
		next.ApplyEnabled = false
		next.Verified = false
		return next
	}

	if reason := replayFailureReason(signal); reason != "" {
		next.LastReplayStatus = firstNonEmpty(signal.ReplayStatus, reason)
		next.ReplayAttemptCount++
		next.Status = StatusReplayFailed
		next.ReasonCodes = appendUnique(next.ReasonCodes, reason)
		next.ApplyEnabled = false
		next.Verified = false
		return next
	}

	if signal.ReplayStatus != "" {
		next.LastReplayStatus = signal.ReplayStatus
		next.ReplayAttemptCount++
		if signal.ReplayStatus != "passed" {
			next.Status = StatusReplayFailed
			next.ApplyEnabled = false
			next.Verified = false
			return next
		}
		next.Status = StatusReplayPassed
	}

	if signal.RequiresApproval && len(next.ApprovalRefs) == 0 {
		next.Status = StatusApprovalRequired
		next.ApplyEnabled = false
		next.Verified = false
		return next
	}

	if len(signal.HumanApprovalRefs) > 0 && next.Status == StatusApprovalRequired {
		next.Status = StatusApproved
		next.ApplyEnabled = true
	}

	if signal.ApplyCompleted {
		if next.Status != StatusApproved || len(next.ApprovalRefs) == 0 {
			next.Status = StatusApprovalRequired
			next.ReasonCodes = appendUnique(next.ReasonCodes, "approval_required")
			next.ApplyEnabled = false
			next.Verified = false
			return next
		}
		next.ApplyEnabled = false
		next.Status = StatusApplied
	}

	if signal.OriginalBlockerCleared && len(postApplyRefs) > 0 && next.Status == StatusApplied && len(next.ApprovalRefs) > 0 {
		next.Status = StatusVerified
		next.Verified = true
		next.ReplayEvidenceRefs = appendUniqueMany(next.ReplayEvidenceRefs, postApplyRefs)
		return next
	}

	next.Verified = false
	return next
}

func RegisterRepeatFailure(candidate ImprovementCandidate, failure FailureInput) ImprovementCandidate {
	next := candidate
	if next.FailureFingerprint != "" && failure.FailureFingerprint != "" && next.FailureFingerprint != failure.FailureFingerprint {
		next.RelatedCandidateIDs = appendUnique(next.RelatedCandidateIDs, stableCandidateID(failure, next.RecommendedRoute, normalizedReasons(failure)))
		return next
	}
	next.AttemptCount++
	next.Active = false
	if next.MaxReplayAttempts <= 0 {
		next.MaxReplayAttempts = 2
	}
	if next.ReplayAttemptCount >= next.MaxReplayAttempts {
		next.Status = StatusBlocked
		next.NonConvergenceReason = "max_replay_attempts_exhausted"
		next.RepairActionEnabled = false
		next.ApplyEnabled = false
	}
	return next
}
