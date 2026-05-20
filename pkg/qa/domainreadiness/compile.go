package domainreadiness

import "strings"

func CompileCatalog(catalog Catalog, opts CompileOptions) (CompileSummary, error) {
	lane := strings.TrimSpace(opts.Lane)
	if lane == "" {
		lane = "fast"
	}
	validation := ValidateCatalog(catalog)
	summary := CompileSummary{
		SchemaVersion:    PlanSchemaVersion,
		ScenarioCount:    len(catalog.Scenarios),
		CommandsExecuted: false,
		SelectedLane:     lane,
		Validation:       validation,
		ScenarioPlans:    []ScenarioPlan{},
		CoveredDomains:   validation.CoveredDomains,
		MissingDomains:   validation.MissingDomains,
	}
	resultsByID := map[string]ScenarioValidationResult{}
	for _, result := range validation.ValidationResults {
		resultsByID[result.ScenarioID] = result
	}
	for _, scenario := range catalog.Scenarios {
		result := resultsByID[scenario.ScenarioID]
		plan := ScenarioPlan{
			ScenarioID:       scenario.ScenarioID,
			Domain:           scenario.Domain,
			Owner:            scenario.Owner,
			OwningRepo:       scenario.OwningRepo,
			ScenarioMode:     scenario.ScenarioMode,
			MutationBoundary: scenario.MutationBoundary,
			Command:          effectiveCommand(scenario.SafeExecutionEnvironment),
			JourneyRefs:      append([]string(nil), scenario.JourneyPackRefs...),
			LaneRefs:         append([]string(nil), scenario.QAMESHLaneRefs...),
			ArtifactRefs:     artifactRefs(scenario),
			AcceptanceRefs:   append([]string(nil), scenario.SourceSpecRefs...),
			SourceNeeds:      append([]string(nil), scenario.FixtureOrSourceNeed...),
			ExpectedEvidence: append([]string(nil), scenario.ExpectedEvidence...),
			PassFailOracle:   append([]string(nil), scenario.PassFailOracle...),
			CanaryRefs:       append([]string(nil), scenario.CanaryRefs...),
			SetupGaps:        append([]string(nil), result.SetupGaps...),
			RejectReasons:    append([]UnsafeReason(nil), result.RejectReasons...),
		}
		if plan.Command != nil {
			plan.Adapter = plan.Command.Adapter
		}
		if len(result.RejectReasons) > 0 {
			summary.RejectedScenarios = append(summary.RejectedScenarios, result)
		}
		if len(plan.LaneRefs) == 0 || containsString(plan.LaneRefs, lane) {
			summary.ScenarioPlans = append(summary.ScenarioPlans, plan)
		}
	}
	return summary, nil
}

func artifactRefs(scenario Scenario) []string {
	refs := []string{}
	for _, value := range scenario.ExpectedEvidence {
		refs = appendUniqueString(refs, "evidence:"+scenario.ScenarioID+":"+value)
	}
	for _, value := range scenario.BackendContractTestRefs {
		refs = appendUniqueString(refs, value)
	}
	for _, value := range scenario.FrontendTypedCardTestRefs {
		refs = appendUniqueString(refs, value)
	}
	for _, value := range scenario.DesktopTypedCardTestRefs {
		refs = appendUniqueString(refs, value)
	}
	return refs
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
