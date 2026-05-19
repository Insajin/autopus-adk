package qualityloop

import "strings"

func qameshGateReasons(input FailureInput, existing []string) []string {
	if !isQAMESHFailure(input, existing) {
		return nil
	}
	reasons := make([]string, 0)
	switch strings.ToLower(strings.TrimSpace(input.QAMESHStatus)) {
	case "passed", "skipped", "deferred":
		reasons = appendUnique(reasons, "qamesh_evidence_not_failed")
	}
	if !input.DeterministicEvidence {
		reasons = appendUnique(reasons, "qamesh_non_deterministic")
	}
	if input.RedactionStatus == "failed" || input.RedactionStatus == "unredacted" {
		reasons = appendUnique(reasons, "redaction_failed")
	}
	if len(input.EvidenceRefs) == 0 {
		reasons = appendUnique(reasons, "missing_replay")
	}
	if len(input.OwnedPaths) == 0 {
		reasons = appendUnique(reasons, "qamesh_unsupported_feedback_target")
	}
	for _, path := range input.OwnedPaths {
		if unsafeText(path) || isGeneratedSurfacePath(path) {
			reasons = appendUnique(reasons, "qamesh_unsafe_feedback_target")
		}
	}
	return reasons
}

func isQAMESHFailure(input FailureInput, reasons []string) bool {
	return input.QAMESHStatus != "" ||
		strings.Contains(strings.ToLower(input.SourceArtifactType), "qamesh") ||
		(strings.Contains(strings.ToLower(input.SourceID), "qamesh") && contains(reasons, "qamesh_failed_check"))
}

func replayFailureReason(signal LifecycleSignal) string {
	switch {
	case signal.ReplayRunIndexMissing:
		return "missing_replay"
	case signal.ReplayOutsideProject:
		return "replay_outside_project"
	case signal.ReplayNonDeterministic:
		return "replay_non_deterministic"
	case signal.ReplayMissingACMapping:
		return "replay_missing_acceptance_mapping"
	case signal.ReplayFreshness == "stale":
		return "stale_replay"
	case signal.ReplayStatus != "" && signal.ReplayStatus != "passed":
		return "replay_not_passed"
	default:
		return ""
	}
}

func safeCandidateRefs(workspaceID string, refs []string) ([]string, []string) {
	safe := make([]string, 0, len(refs))
	reasons := make([]string, 0)
	for _, ref := range refs {
		if unsafeText(ref) {
			reasons = appendUnique(reasons, "unsafe_evidence_ref")
			continue
		}
		if crossWorkspaceRef(workspaceID, ref) {
			reasons = appendUnique(reasons, "cross_workspace_ref")
			continue
		}
		safe = appendUnique(safe, ref)
	}
	return safe, reasons
}

func candidateSafetyRefs(input FailureInput) []string {
	refs := make([]string, 0)
	refs = append(refs, input.EvidenceGapRefs...)
	refs = append(refs, input.SourceRefs...)
	refs = append(refs, input.SourceHashes...)
	refs = append(refs, input.AffectedRefs...)
	refs = append(refs, input.OwnedPaths...)
	refs = append(refs, input.TargetArtifact, input.ExpectedValidation, input.RollbackPath)
	if action := proposedActionFor(input, KindQAMESHRepairHandoff); action != "" {
		refs = append(refs, action)
	}
	return uniqueStrings(refs)
}

func routeMetadataFor(kind string) []string {
	if kind == KindModelRoutingPolicy {
		return []string{"[NEW] planned addition", "approval_gated"}
	}
	return nil
}

func replayPlanFor(kind string) string {
	switch kind {
	case KindSkillEvolveCandidate:
		return "replay affected deterministic oracle before approval"
	case KindQAMESHRepairHandoff:
		return "auto qa feedback replay"
	default:
		return "deterministic replay or post-apply evidence"
	}
}

func approvalGateFor(kind string) string {
	if kind == KindModelRoutingPolicy {
		return "human approval required before model selection change"
	}
	return "human approval required before apply"
}

func safetyGateFor(decision SafetyDecision) string {
	if decision.Accepted {
		return "passed"
	}
	return "blocked"
}
