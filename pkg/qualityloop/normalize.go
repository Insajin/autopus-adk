package qualityloop

import "strings"

var defaultForbiddenWriteSurfaces = []string{
	".codex/**",
	".opencode/**",
	".claude/**",
	".gemini/**",
	".agents/**",
	".autopus/plugins/**",
	".autopus/design/imports/**",
	".autopus/runtime/**",
	".autopus/brainstorms/**",
	".autopus/canary/**",
	".autopus/orchestra/**",
	".autopus/context/signatures.md",
	"plugin-cache",
	"config.toml",
}

func NormalizeFailures(inputs []FailureInput) (NormalizeResult, error) {
	result := NormalizeResult{Candidates: make([]ImprovementCandidate, 0, len(inputs))}
	skip := aggregateRepeatedFailures(inputs, &result)
	for index, input := range inputs {
		if skip[index] {
			continue
		}
		candidate := candidateFromFailure(input)
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}

func candidateFromFailure(input FailureInput) ImprovementCandidate {
	reasons := normalizedReasons(input)
	if isGeneratedSurfacePath(input.TargetArtifact) {
		reasons = appendUnique(reasons, "generated_surface_mutation_forbidden")
	}
	if len(reasons) == 0 {
		reasons = []string{"unsupported_failure_source"}
	}

	classification := classify(input, reasons)
	confidence := input.ConfidenceOverride
	if confidence <= 0 {
		confidence = classification.confidence
	}
	band := confidenceBand(confidence, input, classification.evidenceStrength)
	lowReview := band == BandLow ||
		classification.evidenceStrength == EvidenceLLMOnly ||
		classification.evidenceStrength == EvidenceConflicting ||
		input.ConflictingLLMNarrative

	status := classification.status
	policy := classification.policy
	if lowReview {
		if status == StatusRouted || status == StatusObserved || status == StatusNormalized {
			status = StatusApprovalRequired
		}
		if policy != PolicyAdvisoryOnly {
			policy = PolicyHumanReviewRequired
		}
	}

	safety := ValidateCandidateSafety(CandidateDraft{
		WorkspaceID:            input.WorkspaceID,
		EvidenceRefs:           input.EvidenceRefs,
		DisplayRefs:            candidateSafetyRefs(input),
		TargetArtifact:         input.TargetArtifact,
		RawPayloadPresent:      input.RawPayloadPresent,
		ProviderWriteCallCount: input.ProviderWriteCallCount,
	})
	if !safety.Accepted {
		reasons = appendUniqueMany(reasons, safety.ReasonCodes)
		if contains(safety.ReasonCodes, "generated_surface_mutation_forbidden") {
			classification.taxonomy = TaxonomyUnsafeMutationBoundary
			classification.kind = KindSafetyPolicyPatch
			status = StatusRejected
			policy = PolicyDisabled
			confidence = maxFloat(confidence, 1.0)
			band = BandHigh
		} else if status != StatusRejected {
			status = safety.Status
		}
	}

	repairEnabled := false
	applyEnabled := false
	id := stableCandidateID(input, classification.kind, reasons)
	sourceFailureRefs := sourceFailureRefs(input)
	candidate := ImprovementCandidate{
		SchemaVersion:               SchemaVersion,
		CandidateID:                 id,
		CandidateKind:               classification.kind,
		Status:                      status,
		Active:                      false,
		WorkspaceID:                 input.WorkspaceID,
		OwningRepo:                  firstNonEmpty(input.OwningRepo, ownerRepoFor(input, classification.kind)),
		Owner:                       firstNonEmpty(input.Owner, ownerFor(classification.kind)),
		FailureFingerprint:          firstNonEmpty(input.FailureFingerprint, id),
		FailureTaxonomy:             classification.taxonomy,
		ReasonCodes:                 reasons,
		ClassificationConfidence:    clampConfidence(confidence),
		ConfidenceBand:              band,
		ClassificationMethod:        classification.method,
		EvidenceStrength:            classification.evidenceStrength,
		EvidenceGapRefs:             append([]string{}, input.EvidenceGapRefs...),
		LowConfidenceReviewRequired: lowReview,
		Severity:                    severityFor(status, classification.taxonomy),
		DeterministicAuthority:      input.DeterministicEvidence && classification.evidenceStrength != EvidenceLLMOnly,
		SourceFailureRefs:           sourceFailureRefs,
		SourceArtifactType:          input.SourceArtifactType,
		SourceHashes:                append([]string{}, input.SourceHashes...),
		EvidenceRefs:                append([]string{}, input.EvidenceRefs...),
		SourceRefs:                  append([]string{}, input.SourceRefs...),
		ForbiddenWriteSurfaces:      append([]string{}, defaultForbiddenWriteSurfaces...),
		AffectedOutputs:             append([]string{}, input.AffectedRefs...),
		AffectedRefs:                append([]string{}, input.AffectedRefs...),
		AffectedAcceptanceIDs:       append([]string{}, input.AffectedAcceptanceIDs...),
		RecommendedRoute:            classification.kind,
		RouteTargets:                routeTargets(input, classification.kind),
		TargetArtifact:              input.TargetArtifact,
		SourceOwnedTargetPath:       input.TargetArtifact,
		GeneratedSurfaceValidation:  generatedSurfaceValidation(input.TargetArtifact),
		Risk:                        riskFor(classification.taxonomy),
		ExpectedValidation:          input.ExpectedValidation,
		RollbackPath:                input.RollbackPath,
		ProposedAction:              proposedActionFor(input, classification.kind),
		RouteMetadata:               routeMetadataFor(classification.kind),
		RepairActionPolicy:          policy,
		RepairActionEnabled:         repairEnabled,
		ApplyEnabled:                applyEnabled,
		Verified:                    false,
		MaxReplayAttempts:           2,
		ReplayPlan:                  replayPlanFor(classification.kind),
		ApprovalGate:                approvalGateFor(classification.kind),
		SafetyGate:                  safetyGateFor(safety),
		SafetyReasonCodes:           safetyReasonCodes(safety),
		ProviderWriteCallCount:      0,
		RedactionStatus:             RedactionMetadataOnly,
		RetentionClass:              RetentionAudit,
		RawPayloadPresent:           false,
		AuditRefs:                   []string{"audit:" + id},
	}
	if candidate.ExpectedValidation == "" {
		candidate.ExpectedValidation = expectedValidationFor(classification.kind)
	}
	if candidate.RollbackPath == "" {
		candidate.RollbackPath = rollbackFor(classification.kind)
	}
	return candidate
}

func normalizedReasons(input FailureInput) []string {
	reasons := make([]string, 0, len(input.ReasonCodes)+1)
	if strings.TrimSpace(input.ReasonCode) != "" {
		reasons = appendUnique(reasons, strings.TrimSpace(input.ReasonCode))
	}
	for _, reason := range input.ReasonCodes {
		if strings.TrimSpace(reason) != "" {
			reasons = appendUnique(reasons, strings.TrimSpace(reason))
		}
	}
	if input.ReplayFreshness == "stale" {
		reasons = appendUnique(reasons, "stale_replay")
	}
	reasons = appendUniqueMany(reasons, qameshGateReasons(input, reasons))
	if input.RawPayloadPresent {
		reasons = appendUnique(reasons, "raw_payload_present")
	}
	if input.ProviderWriteCallCount > 0 {
		reasons = appendUnique(reasons, "provider_write_not_allowed")
	}
	return reasons
}
