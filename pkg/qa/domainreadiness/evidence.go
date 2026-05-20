package domainreadiness

import (
	"fmt"
	"strings"
)

func BuildDomainReadinessEvidence(input EvidenceBuildInput) (DomainReadinessEvidence, error) {
	if strings.TrimSpace(input.SuiteID) == "" {
		return DomainReadinessEvidence{}, fmt.Errorf("missing suite id")
	}
	if strings.TrimSpace(input.RunID) == "" {
		return DomainReadinessEvidence{}, fmt.Errorf("missing run id")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" {
		return DomainReadinessEvidence{}, fmt.Errorf("missing workspace id")
	}
	if len(input.Scenarios) == 0 {
		return DomainReadinessEvidence{}, fmt.Errorf("missing scenarios")
	}

	first := input.Scenarios[0].Scenario
	row := DomainReadinessEvidence{
		SchemaVersion:          EvidenceSchemaVersion,
		SuiteID:                input.SuiteID,
		RunID:                  input.RunID,
		WorkspaceID:            input.WorkspaceID,
		Domain:                 first.Domain,
		ScenarioIDs:            []string{},
		DomainReadinessState:   DomainReadinessStateReady,
		ScenarioResults:        []ScenarioResultEntry{},
		SourceNeeds:            []string{},
		SetupGaps:              []string{},
		Blockers:               []string{},
		ExpectedEvidenceRefs:   []string{},
		ActualEvidenceRefs:     []string{},
		QAMESHRefs:             []string{},
		CanaryRefs:             []string{},
		BackendContractRefs:    []string{},
		FrontendCardRefs:       []string{},
		DesktopCardRefs:        []string{},
		Freshness:              EvidenceFreshnessCurrent,
		DenominatorIncluded:    true,
		Owner:                  first.Owner,
		OwningRepo:             first.OwningRepo,
		AuditRefs:              []string{},
		ProviderReadCallCount:  0,
		ProviderWriteCallCount: 0,
		RedactionStatus:        RedactionStatusPassed,
		RetentionClass:         RetentionClassMetadataOnly,
		RawPayloadPresent:      false,
		UnsafeReasons:          []UnsafeReason{},
	}

	hasReady := false
	hasBlocked := false
	hasSetupGap := false
	hasStale := false
	hasUnsafe := false
	hasFailed := false

	for _, scenarioInput := range input.Scenarios {
		scenario := scenarioInput.Scenario
		if scenario.Domain != row.Domain {
			return DomainReadinessEvidence{}, fmt.Errorf("mixed domains are not supported: %s and %s", row.Domain, scenario.Domain)
		}
		row.ScenarioIDs = appendUniqueString(row.ScenarioIDs, scenario.ScenarioID)
		row.SourceNeeds = appendUniqueStrings(row.SourceNeeds, scenario.FixtureOrSourceNeed...)
		row.ExpectedEvidenceRefs = appendUniqueStrings(row.ExpectedEvidenceRefs, scenario.ExpectedEvidence...)
		row.ExpectedEvidenceRefs = appendUniqueStrings(row.ExpectedEvidenceRefs, scenarioInput.ExpectedEvidenceRefs...)
		row.ActualEvidenceRefs = appendUniqueStrings(row.ActualEvidenceRefs, scenarioInput.ActualEvidenceRefs...)
		row.QAMESHRefs = appendUniqueStrings(row.QAMESHRefs, scenarioInput.QAMESHRefs...)
		row.CanaryRefs = appendUniqueStrings(row.CanaryRefs, scenarioInput.CanaryRefs...)
		row.BackendContractRefs = appendUniqueStrings(row.BackendContractRefs, scenario.BackendContractTestRefs...)
		row.BackendContractRefs = appendUniqueStrings(row.BackendContractRefs, scenarioInput.BackendContractRefs...)
		row.FrontendCardRefs = appendUniqueStrings(row.FrontendCardRefs, scenario.FrontendTypedCardTestRefs...)
		row.FrontendCardRefs = appendUniqueStrings(row.FrontendCardRefs, scenarioInput.FrontendCardRefs...)
		row.DesktopCardRefs = appendUniqueStrings(row.DesktopCardRefs, scenario.DesktopTypedCardTestRefs...)
		row.DesktopCardRefs = appendUniqueStrings(row.DesktopCardRefs, scenarioInput.DesktopCardRefs...)
		row.SetupGaps = appendUniqueStrings(row.SetupGaps, scenarioInput.SetupGaps...)
		row.Blockers = appendUniqueStrings(row.Blockers, scenarioInput.Blockers...)
		row.AuditRefs = appendUniqueStrings(row.AuditRefs, scenarioInput.AuditRefs...)
		row.ProviderReadCallCount += scenarioInput.ProviderReadCallCount
		row.ProviderWriteCallCount += scenarioInput.ProviderWriteCallCount
		row.RawPayloadPresent = row.RawPayloadPresent || scenarioInput.RawPayloadPresent
		for _, reason := range scenarioInput.UnsafeReasons {
			row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, reason)
		}

		result := scenarioInput.Result
		if result == "" {
			result = ScenarioResultSetupGap
		}
		freshness := scenarioInput.Freshness
		if freshness == "" {
			freshness = EvidenceFreshnessMissing
		}
		row.Freshness = worseFreshness(row.Freshness, freshness)
		redaction := strings.TrimSpace(scenarioInput.RedactionStatus)
		if redaction == "" {
			redaction = RedactionStatusFailed
		}
		if row.RedactionStatus == RedactionStatusPassed && redaction != RedactionStatusPassed {
			row.RedactionStatus = redaction
		}
		retention := strings.TrimSpace(scenarioInput.RetentionClass)
		if retention != "" {
			row.RetentionClass = retention
		}

		if scenarioInput.RawPayloadPresent {
			result = ScenarioResultRejected
			hasUnsafe = true
			row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, UnsafeReasonRawPayloadNotAllowed)
			row.Blockers = appendUniqueString(row.Blockers, string(UnsafeReasonRawPayloadNotAllowed))
		}
		if scenarioInput.ProviderWriteCallCount > 0 {
			result = ScenarioResultRejected
			hasUnsafe = true
			row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, UnsafeReasonProviderWriteNotAllowed)
			row.Blockers = appendUniqueString(row.Blockers, string(UnsafeReasonProviderWriteNotAllowed))
		}
		if redaction != RedactionStatusPassed {
			result = ScenarioResultRejected
			hasUnsafe = true
			row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, UnsafeReasonRedactionFailed)
			row.Blockers = appendUniqueString(row.Blockers, string(UnsafeReasonRedactionFailed))
			row.Freshness = EvidenceFreshnessRedactionFailed
		}
		if len(nonEmptyStrings(scenarioInput.SourceRefs)) == 0 {
			if !hasUnsafe {
				result = ScenarioResultSetupGap
			}
			hasSetupGap = true
			row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, UnsafeReasonSourceEvidenceMissing)
			row.SetupGaps = appendUniqueString(row.SetupGaps, string(UnsafeReasonSourceEvidenceMissing))
		}
		switch freshness {
		case EvidenceFreshnessStale, EvidenceFreshnessExpired:
			if !hasUnsafe && result == ScenarioResultPassed {
				result = ScenarioResultStale
			}
			hasStale = true
			row.Blockers = appendUniqueString(row.Blockers, string(UnsafeReasonStaleEvidence))
		case EvidenceFreshnessMissing:
			if !hasUnsafe {
				result = ScenarioResultSetupGap
			}
			hasSetupGap = true
		case EvidenceFreshnessUnsafe, EvidenceFreshnessCrossWorkspaceBlocked:
			result = ScenarioResultRejected
			hasUnsafe = true
			if freshness == EvidenceFreshnessCrossWorkspaceBlocked {
				row.UnsafeReasons = appendUniqueReason(row.UnsafeReasons, UnsafeReasonCrossWorkspaceRef)
				row.Blockers = appendUniqueString(row.Blockers, string(UnsafeReasonCrossWorkspaceRef))
			}
		}
		if aiOnly(scenarioInput) {
			if !hasUnsafe {
				result = ScenarioResultBlocked
			}
			hasBlocked = true
			row.Blockers = appendUniqueString(row.Blockers, "ai_only_pass_fail")
		}

		switch result {
		case ScenarioResultPassed:
			hasReady = true
		case ScenarioResultSetupGap:
			hasSetupGap = true
		case ScenarioResultStale:
			hasStale = true
		case ScenarioResultRejected:
			hasUnsafe = true
		case ScenarioResultBlocked:
			hasBlocked = true
		case ScenarioResultFailed:
			hasFailed = true
		}
		row.ScenarioResults = upsertScenarioResult(row.ScenarioResults, ScenarioResultEntry{
			ScenarioID:             scenario.ScenarioID,
			ScenarioResult:         result,
			ReasonCode:             string(result),
			DeterministicOracleRef: firstNonEmptyString(scenarioInput.DeterministicOracleRefs),
			EvidenceRefIDs:         appendUniqueStrings(append([]string{}, scenarioInput.ActualEvidenceRefs...), scenarioInput.QAMESHRefs...),
			ProviderWriteCallCount: scenarioInput.ProviderWriteCallCount,
			RedactionStatus:        redaction,
			RawPayloadPresent:      scenarioInput.RawPayloadPresent,
		})
	}

	switch {
	case hasUnsafe:
		row.DomainReadinessState = DomainReadinessStateUnsafe
	case hasSetupGap && !hasReady:
		row.DomainReadinessState = DomainReadinessStateSetupGap
	case hasStale && !hasReady:
		row.DomainReadinessState = DomainReadinessStateStale
	case hasBlocked || hasFailed:
		if hasReady {
			row.DomainReadinessState = DomainReadinessStatePartial
		} else {
			row.DomainReadinessState = DomainReadinessStateBlocked
		}
	case hasSetupGap || hasStale:
		row.DomainReadinessState = DomainReadinessStatePartial
	default:
		row.DomainReadinessState = DomainReadinessStateReady
	}
	row.DenominatorIncluded = row.DomainReadinessState == DomainReadinessStateReady
	if !row.DenominatorIncluded {
		row.ExclusionReason = exclusionReason(row)
	}
	return row, nil
}

func upsertScenarioResult(results []ScenarioResultEntry, next ScenarioResultEntry) []ScenarioResultEntry {
	for i, result := range results {
		if result.ScenarioID == next.ScenarioID {
			results[i] = next
			return results
		}
	}
	return append(results, next)
}

func firstNonEmptyString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
