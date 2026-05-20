package domainreadiness

func aiOnly(input ScenarioEvidenceInput) bool {
	if input.PassFailSupport == PassFailSupportAIOnly {
		return true
	}
	return input.Result == ScenarioResultPassed &&
		len(nonEmptyStrings(input.SourceRefs)) == 0 &&
		len(nonEmptyStrings(input.DeterministicOracleRefs)) == 0 &&
		len(nonEmptyStrings(input.QAMESHRefs)) == 0 &&
		len(nonEmptyStrings(input.CanaryRefs)) == 0 &&
		len(nonEmptyStrings(input.BackendContractRefs)) == 0 &&
		len(nonEmptyStrings(input.AuditRefs)) == 0
}

func worseFreshness(current, candidate EvidenceFreshness) EvidenceFreshness {
	rank := map[EvidenceFreshness]int{
		EvidenceFreshnessCurrent:               0,
		EvidenceFreshnessMissing:               1,
		EvidenceFreshnessStale:                 2,
		EvidenceFreshnessExpired:               3,
		EvidenceFreshnessCrossWorkspaceBlocked: 4,
		EvidenceFreshnessUnsafe:                5,
		EvidenceFreshnessRedactionFailed:       6,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func exclusionReason(row DomainReadinessEvidence) string {
	if containsReason(row.UnsafeReasons, UnsafeReasonRawPayloadNotAllowed) {
		return string(UnsafeReasonRawPayloadNotAllowed)
	}
	if containsReason(row.UnsafeReasons, UnsafeReasonProviderWriteNotAllowed) {
		return string(UnsafeReasonProviderWriteNotAllowed)
	}
	if containsReason(row.UnsafeReasons, UnsafeReasonRedactionFailed) {
		return string(UnsafeReasonRedactionFailed)
	}
	if containsReason(row.UnsafeReasons, UnsafeReasonSourceEvidenceMissing) {
		return string(UnsafeReasonSourceEvidenceMissing)
	}
	for _, blocker := range row.Blockers {
		if blocker != "" {
			return blocker
		}
	}
	for _, gap := range row.SetupGaps {
		if gap != "" {
			return gap
		}
	}
	if row.Freshness != EvidenceFreshnessCurrent {
		return string(row.Freshness)
	}
	return string(row.DomainReadinessState)
}

func containsReason(values []UnsafeReason, target UnsafeReason) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendUniqueStrings(values []string, additions ...string) []string {
	for _, addition := range additions {
		values = appendUniqueString(values, addition)
	}
	return values
}
