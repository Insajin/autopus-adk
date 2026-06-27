package qualityloop

import (
	"path"
	"strings"
)

func aggregateRepeatedFailures(inputs []FailureInput, result *NormalizeResult) map[int]bool {
	groups := map[string][]int{}
	for index, input := range inputs {
		if repeatedADKFailure(input) {
			groups[input.FailureFingerprint] = append(groups[input.FailureFingerprint], index)
		}
	}
	skip := map[int]bool{}
	for _, indexes := range groups {
		if len(indexes) < 3 {
			continue
		}
		failures := make([]FailureInput, 0, len(indexes))
		for _, index := range indexes {
			failures = append(failures, inputs[index])
			skip[index] = true
		}
		result.Candidates = append(result.Candidates, repeatedSkillCandidate(failures))
	}
	return skip
}

func repeatedADKFailure(input FailureInput) bool {
	if input.FailureFingerprint == "" {
		return false
	}
	if input.RedactionStatus == "failed" || input.RawPayloadPresent {
		return false
	}
	if isGeneratedSurfacePath(input.TargetArtifact) {
		return false
	}
	for _, ref := range append(append([]string{}, input.AffectedRefs...), input.OwnedPaths...) {
		if isGeneratedSurfacePath(ref) {
			return false
		}
		if isADKOwnedPath(ref) {
			return true
		}
	}
	return isADKOwnedPath(input.TargetArtifact)
}

func isADKOwnedPath(rel string) bool {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || path.IsAbs(rel) || strings.Contains(rel, "\x00") || hasParentPathSegment(rel) {
		return false
	}
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") || isGeneratedSurfacePath(rel) {
		return false
	}
	rel = strings.TrimPrefix(rel, "autopus-adk/")
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") || isGeneratedSurfacePath(rel) {
		return false
	}
	for _, prefix := range []string{"content/", "templates/", "pkg/", ".autopus/specs/"} {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func hasParentPathSegment(rel string) bool {
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func minimalityReasonCodes() []string {
	return []string{
		"unnecessary_dependency",
		"duplicate_helper",
		"single_impl_abstraction",
		"stdlib_available",
		"native_available",
		"yagni_expansion",
		"existing_helper_available",
		"existing_dependency_available",
		"shrink_scope_available",
	}
}

func repeatedSkillCandidate(failures []FailureInput) ImprovementCandidate {
	first := failures[0]
	reasons := []string{"repeated_failure", firstNonEmpty(first.ReasonCode, "skill_instruction_gap")}
	sourceRefs := make([]string, 0, len(failures))
	hashes := make([]string, 0)
	affected := make([]string, 0)
	acceptance := make([]string, 0)
	for _, failure := range failures {
		sourceRefs = appendUniqueMany(sourceRefs, sourceFailureRefs(failure))
		hashes = appendUniqueMany(hashes, failure.SourceHashes)
		affected = appendUniqueMany(affected, failure.AffectedRefs)
		acceptance = appendUniqueMany(acceptance, failure.AffectedAcceptanceIDs)
	}
	id := stableCandidateID(first, KindSkillEvolveCandidate, reasons)
	return ImprovementCandidate{
		SchemaVersion:               SchemaVersion,
		CandidateID:                 id,
		CandidateKind:               KindSkillEvolveCandidate,
		Status:                      StatusQuarantined,
		Active:                      false,
		WorkspaceID:                 first.WorkspaceID,
		OwningRepo:                  "autopus-adk",
		Owner:                       "autopus-adk",
		FailureFingerprint:          first.FailureFingerprint,
		FailureTaxonomy:             TaxonomySkillOrPlaybookGap,
		ReasonCodes:                 reasons,
		ClassificationConfidence:    0.82,
		ConfidenceBand:              BandHigh,
		ClassificationMethod:        MethodContractMapping,
		EvidenceStrength:            EvidenceDeterministic,
		LowConfidenceReviewRequired: false,
		Severity:                    "high",
		DeterministicAuthority:      true,
		SourceFailureRefs:           sourceRefs,
		SourceArtifactType:          first.SourceArtifactType,
		SourceHashes:                hashes,
		ForbiddenWriteSurfaces:      append([]string{}, defaultForbiddenWriteSurfaces...),
		AffectedOutputs:             affected,
		AffectedRefs:                affected,
		AffectedAcceptanceIDs:       acceptance,
		RecommendedRoute:            KindSkillEvolveCandidate,
		RouteTargets:                affected,
		TargetArtifact:              firstNonEmpty(first.TargetArtifact, firstNonEmpty(affected...)),
		SourceOwnedTargetPath:       firstNonEmpty(first.TargetArtifact, firstNonEmpty(affected...)),
		GeneratedSurfaceValidation:  "source_owned",
		Risk:                        "medium",
		ExpectedValidation:          "deterministic replay or post-apply evidence",
		RollbackPath:                "archive candidate without source mutation",
		ProposedDigest:              digestStrings(affected, hashes),
		GenerationPromptDigest:      digestStrings(reasons, sourceRefs),
		RepairActionPolicy:          PolicyReplayRequired,
		MaxReplayAttempts:           2,
		ReplayPlan:                  replayPlanFor(KindSkillEvolveCandidate),
		ApprovalGate:                approvalGateFor(KindSkillEvolveCandidate),
		SafetyGate:                  "passed",
		ProviderWriteCallCount:      0,
		RedactionStatus:             RedactionMetadataOnly,
		RetentionClass:              RetentionAudit,
		RawPayloadPresent:           false,
		AuditRefs:                   []string{"audit:" + id},
	}
}
