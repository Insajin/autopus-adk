package domainreadiness

import "strings"

func BuildSetupGapReport(catalog Catalog, opts ReportOptions) (Report, error) {
	if strings.TrimSpace(opts.SuiteID) == "" {
		opts.SuiteID = catalog.SuiteID
	}
	if strings.TrimSpace(opts.SuiteID) == "" {
		opts.SuiteID = "domain-readiness"
	}
	if strings.TrimSpace(opts.RunID) == "" {
		opts.RunID = "domain-readiness-plan"
	}
	if strings.TrimSpace(opts.WorkspaceID) == "" {
		opts.WorkspaceID = "00000000-0000-4000-8000-000000000001"
	}
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		SuiteID:       opts.SuiteID,
		RunID:         opts.RunID,
		WorkspaceID:   opts.WorkspaceID,
		Evidence:      []DomainReadinessEvidence{},
	}
	for _, scenario := range catalog.Scenarios {
		row, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
			SuiteID:     opts.SuiteID,
			RunID:       opts.RunID,
			WorkspaceID: opts.WorkspaceID,
			Scenarios: []ScenarioEvidenceInput{{
				Scenario:               scenario,
				Result:                 ScenarioResultSetupGap,
				Freshness:              EvidenceFreshnessMissing,
				ProviderReadCallCount:  0,
				ProviderWriteCallCount: 0,
				RedactionStatus:        RedactionStatusPassed,
				RetentionClass:         RetentionClassMetadataOnly,
				RawPayloadPresent:      false,
				SetupGaps:              []string{"source_evidence_missing"},
				AuditRefs:              []string{"audit:domain_readiness:" + scenario.ScenarioID},
			}},
		})
		if err != nil {
			return Report{}, err
		}
		report.Evidence = append(report.Evidence, row)
	}
	report.EvidenceCount = len(report.Evidence)
	return report, nil
}
