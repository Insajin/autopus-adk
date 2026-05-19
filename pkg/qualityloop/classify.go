package qualityloop

type classification struct {
	taxonomy         string
	kind             string
	status           string
	confidence       float64
	policy           string
	method           string
	evidenceStrength string
}

func classify(input FailureInput, reasons []string) classification {
	if contains(reasons, "generated_surface_mutation_forbidden") {
		return classification{TaxonomyUnsafeMutationBoundary, KindSafetyPolicyPatch, StatusRejected, 1.00, PolicyDisabled, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "secret_risk", "redaction_failed", "cross_workspace_ref", "unsafe_evidence_ref", "raw_payload_present", "unsafe_action_requested", "safety_policy_gap", "prompt_injection_risk") {
		return classification{TaxonomySafetyPolicyGap, KindSafetyPolicyPatch, StatusQuarantined, 0.84, PolicyHumanReviewRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "qamesh_evidence_not_failed", "qamesh_non_deterministic", "qamesh_unsupported_feedback_target", "qamesh_unsafe_feedback_target") {
		return classification{TaxonomyStaleOrMissingEvidence, KindProductBugFix, StatusReplayFailed, 0.80, PolicyDisabled, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "missing_connector_source", "missing_company_source", "source_setup_gap", "workspace_profile_gap", "non_canonical_source", "setup_gap") {
		return classification{TaxonomySourceSetupGap, KindSourceSetupMission, StatusRouted, 0.90, PolicyApprovalRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "qamesh_failed_check") {
		return classification{TaxonomyProductBug, KindQAMESHRepairHandoff, StatusAwaitingReplay, 0.93, PolicyReplayRequired, MethodDeterministicOracle, EvidenceDeterministic}
	}
	if containsAny(reasons, "skill_instruction_gap", "playbook_import_unresolved", "operating_pack_scanner_block", "role_mismatch", "reviewer_gate_bypassed", "bad_escalation") {
		if contains(reasons, "operating_pack_scanner_block") {
			return classification{TaxonomySkillOrPlaybookGap, KindOperatingPackCandidate, StatusQuarantined, 0.84, PolicyApprovalRequired, MethodContractMapping, evidenceStrength(input)}
		}
		return classification{TaxonomySkillOrPlaybookGap, KindSkillEvolveCandidate, StatusQuarantined, 0.82, PolicyReplayRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "fabricated_success", "fabricated_evidence", "unsupported_completion_claim", "llm_judge_only", "default_metric_only", "oracle_structural_only") {
		if contains(reasons, "llm_judge_only") || input.EvidenceStrength == EvidenceLLMOnly {
			return classification{TaxonomyPromptContractGap, KindEvidenceDiscipline, StatusApprovalRequired, 0.35, PolicyAdvisoryOnly, MethodLLMAssistedReview, EvidenceLLMOnly}
		}
		return classification{TaxonomyPromptContractGap, KindEvidenceDiscipline, StatusRouted, 0.85, PolicyReplayRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "wrong_model_class", "cost_latency_mismatch", "quality_regression_by_route", "fallback_model_missing", "model_policy_unapproved") {
		return classification{TaxonomyModelRoutingGap, KindModelRoutingPolicy, StatusApprovalRequired, 0.76, PolicyApprovalRequired, MethodMixedEvidence, evidenceStrength(input)}
	}
	if containsAny(reasons, "flaky_evaluator", "grader_drift", "oracle_changed", "threshold_policy_unapproved", "insufficient_human_sample", "false_pass_suspected", "false_fail_suspected") {
		return classification{TaxonomyEvaluatorOrOracleGap, KindEvalCalibrationTask, StatusRouted, 0.88, PolicyHumanReviewRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "stale_replay", "missing_replay", "stale_evidence", "expired_evidence", "missing_evidence") {
		return classification{TaxonomyStaleOrMissingEvidence, KindProductBugFix, StatusReplayFailed, 0.80, PolicyReplayRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "launch_hard_gate_failed", "launch_degraded_advisory") {
		return classification{TaxonomyStaleOrMissingEvidence, KindLaunchGateBlocker, StatusRouted, 0.90, PolicyApprovalRequired, MethodContractMapping, evidenceStrength(input)}
	}
	if containsAny(reasons, "contract_mismatch", "card_validator_bug", "runtime_behavior_bug", "canary_build_fail", "canary_e2e_fail", "canary_endpoint_fail", "acceptance_mismatch") {
		kind := KindProductBugFix
		if input.TargetArtifact != "" || contains(reasons, "contract_mismatch") || contains(reasons, "acceptance_mismatch") {
			kind = KindImplementationSpec
		}
		return classification{TaxonomyProductBug, kind, StatusRouted, 0.92, PolicyApprovalRequired, MethodDeterministicOracle, EvidenceDeterministic}
	}
	if containsAny(reasons, "ambiguous_acceptance", "expectation_mismatch", "missing_user_decision", "scope_boundary_unclear", "documentation_gap") {
		return classification{TaxonomyUserExpectationGap, KindUserExpectationCandidate, StatusApprovalRequired, 0.50, PolicyHumanReviewRequired, MethodMixedEvidence, EvidenceConflicting}
	}
	return classification{"unsupported", "unsupported", StatusRejected, 0.0, PolicyDisabled, MethodContractMapping, EvidenceMissing}
}

func evidenceStrength(input FailureInput) string {
	if input.EvidenceStrength != "" {
		return input.EvidenceStrength
	}
	if input.ConflictingLLMNarrative {
		return EvidenceConflicting
	}
	if input.DeterministicEvidence {
		return EvidenceDeterministic
	}
	if len(input.EvidenceRefs) == 0 {
		return EvidenceMissing
	}
	return EvidenceMixed
}

func confidenceBand(confidence float64, input FailureInput, strength string) string {
	if input.ConflictingLLMNarrative || strength == EvidenceLLMOnly || strength == EvidenceConflicting || confidence < 0.60 {
		return BandLow
	}
	if confidence >= 0.80 {
		return BandHigh
	}
	return BandMedium
}
