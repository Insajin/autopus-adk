package domainreadiness

import "strings"

func ValidateCatalog(catalog Catalog) CatalogValidationReport {
	report := CatalogValidationReport{
		Valid:             true,
		ScenarioCount:     len(catalog.Scenarios),
		CoveredDomains:    []string{},
		MissingDomains:    []string{},
		ValidationResults: []ScenarioValidationResult{},
	}
	if strings.TrimSpace(catalog.SchemaVersion) != CatalogSchemaVersion {
		report.Valid = false
	}
	seenDomains := map[string]bool{}
	for _, scenario := range catalog.Scenarios {
		result := ValidateScenario(scenario)
		report.ValidationResults = append(report.ValidationResults, result)
		if !result.Valid {
			report.Valid = false
		}
		if strings.TrimSpace(scenario.Domain) != "" {
			seenDomains[scenario.Domain] = true
		}
	}
	if len(catalog.Scenarios) == 0 {
		report.Valid = false
		report.MissingDomains = append(report.MissingDomains, "scenario")
		return report
	}
	requiredDomains := nonEmptyStrings(catalog.RequiredDomains)
	if len(requiredDomains) == 0 {
		for domain := range seenDomains {
			report.CoveredDomains = appendUniqueString(report.CoveredDomains, domain)
		}
		report.CoveredDomains = sortStrings(report.CoveredDomains)
		return report
	}
	for _, domain := range requiredDomains {
		if seenDomains[domain] {
			report.CoveredDomains = append(report.CoveredDomains, domain)
		} else {
			report.Valid = false
			report.MissingDomains = append(report.MissingDomains, domain)
		}
	}
	return report
}

func ValidateScenario(scenario Scenario) ScenarioValidationResult {
	result := ScenarioValidationResult{
		ScenarioID:     scenario.ScenarioID,
		Domain:         scenario.Domain,
		Valid:          true,
		ScenarioResult: ScenarioResultPassed,
	}

	addFinding := func(value string) {
		result.Findings = appendUniqueString(result.Findings, value)
		result.Valid = false
	}
	addBlocker := func(value string) {
		result.Blockers = appendUniqueString(result.Blockers, value)
		result.Valid = false
	}
	addGap := func(value string) {
		result.SetupGaps = appendUniqueString(result.SetupGaps, value)
		result.Valid = false
	}
	addReason := func(reason UnsafeReason) {
		result.RejectReasons = appendUniqueReason(result.RejectReasons, reason)
		result.Valid = false
	}

	if strings.TrimSpace(scenario.SchemaVersion) != ScenarioSchemaVersion {
		addFinding("invalid_schema_version")
	}
	if strings.TrimSpace(scenario.ScenarioID) == "" {
		addFinding("missing_scenario_id")
	}
	if strings.TrimSpace(scenario.Domain) == "" {
		addFinding("missing_domain")
	}
	if strings.TrimSpace(scenario.Owner) == "" {
		addFinding("missing_owner")
		addBlocker("missing_owner")
	}
	if strings.TrimSpace(scenario.OwningRepo) == "" {
		addFinding("missing_owning_repo")
		addBlocker("missing_owning_repo")
	}
	if len(nonEmptyStrings(scenario.SourceSpecRefs)) == 0 {
		addFinding("missing_source_spec_refs")
		addGap("missing_source_spec_refs")
		addReason(UnsafeReasonSourceEvidenceMissing)
	}
	if strings.TrimSpace(string(scenario.ScenarioMode)) == "" {
		addFinding("missing_scenario_mode")
	}
	if strings.TrimSpace(string(scenario.MutationBoundary)) == "" {
		addFinding("missing_mutation_boundary")
		addBlocker("missing_mutation_boundary")
	}
	if len(nonEmptyStrings(scenario.FixtureOrSourceNeed)) == 0 {
		addFinding("missing_fixture_or_source_need")
		addGap("missing_fixture_or_source_need")
		addReason(UnsafeReasonSourceEvidenceMissing)
	}
	if len(nonEmptyStrings(scenario.ExpectedEvidence)) == 0 {
		addFinding("missing_expected_evidence")
		addGap("missing_expected_evidence")
		addReason(UnsafeReasonSourceEvidenceMissing)
	}
	if len(nonEmptyStrings(scenario.PassFailOracle)) == 0 {
		addFinding("missing_pass_fail_oracle")
		addBlocker("missing_pass_fail_oracle")
	}
	if scenario.FreshnessWindowHours <= 0 {
		addFinding("missing_freshness_window_hours")
		addBlocker("missing_freshness_window_hours")
	}
	if len(nonEmptyStrings(scenario.ForbiddenActions)) == 0 {
		addFinding("missing_forbidden_actions")
		addBlocker("missing_forbidden_actions")
	}
	if strings.TrimSpace(scenario.SafeExecutionEnvironment.Kind) == "" {
		addFinding("missing_safe_execution_environment")
		addBlocker("missing_safe_execution_environment")
	}
	if strings.TrimSpace(scenario.LaunchQualityDomain) == "" {
		addFinding("missing_launch_quality_domain")
		addBlocker("missing_launch_quality_domain")
	}

	for _, action := range scenario.RequestedActions {
		class := classifyUnsafeAction(action)
		switch class {
		case UnsafeReasonBroadScrapingNotAllowed:
			addReason(UnsafeReasonBroadScrapingNotAllowed)
			addFinding("unsafe_action:broad_scraping")
		case UnsafeReasonProductionMutationForbidden:
			addReason(UnsafeReasonProductionMutationForbidden)
			addReason(UnsafeReasonProviderWriteNotAllowed)
			addFinding("unsafe_action:" + actionFinding(action))
		}
	}

	command := effectiveCommand(scenario.SafeExecutionEnvironment)
	if command != nil {
		for _, finding := range validateCommandShape(*command) {
			addFinding(finding)
			if finding == "invented_command" {
				addReason(UnsafeReasonInventedCommand)
				continue
			}
			addReason(UnsafeReasonUnsafeCommand)
		}
	}
	if scenario.ScenarioMode == ScenarioModeGUISafeShell {
		if len(nonEmptyStrings(scenario.SafeExecutionEnvironment.AllowedOrigins)) == 0 {
			addFinding("missing_allowed_origins")
			addBlocker("missing_allowed_origins")
		}
		selector := strings.ToLower(strings.TrimSpace(scenario.SafeExecutionEnvironment.SelectorStrategy))
		if selector != "role-first" && selector != "accessibility-first" {
			addFinding("missing_role_or_accessibility_first_selector_strategy")
			addBlocker("missing_role_or_accessibility_first_selector_strategy")
		}
	}

	result.Findings = sortStrings(result.Findings)
	result.Blockers = sortStrings(result.Blockers)
	result.SetupGaps = sortStrings(result.SetupGaps)

	if len(result.RejectReasons) > 0 && containsReject(result.RejectReasons, UnsafeReasonUnsafeCommand, UnsafeReasonInventedCommand, UnsafeReasonProductionMutationForbidden, UnsafeReasonProviderWriteNotAllowed, UnsafeReasonBroadScrapingNotAllowed) {
		result.ScenarioResult = ScenarioResultRejected
	} else if len(result.SetupGaps) > 0 {
		result.ScenarioResult = ScenarioResultSetupGap
	} else if len(result.Blockers) > 0 || !result.Valid {
		result.ScenarioResult = ScenarioResultBlocked
	}
	return result
}
