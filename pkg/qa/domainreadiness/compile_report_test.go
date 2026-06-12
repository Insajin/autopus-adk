package domainreadiness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompileCatalogLaneFilterExcludesNonMatchingScenarios asserts the lane filter
// keeps only scenarios whose lane refs include the requested lane.
func TestCompileCatalogLaneFilterExcludesNonMatchingScenarios(t *testing.T) {
	t.Parallel()

	fastScenario := sampleScenario("fast-1", "core")
	fastScenario.QAMESHLaneRefs = []string{"fast"}

	fullScenario := sampleScenario("full-1", "payments")
	fullScenario.QAMESHLaneRefs = []string{"full"}

	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		SuiteID:       "suite",
		Scenarios:     []Scenario{fastScenario, fullScenario},
	}

	plan, err := CompileCatalog(catalog, CompileOptions{Lane: "fast"})
	require.NoError(t, err)

	require.Len(t, plan.ScenarioPlans, 1)
	assert.Equal(t, "fast-1", plan.ScenarioPlans[0].ScenarioID)

	// With "full" lane only the full scenario is included.
	plan2, err := CompileCatalog(catalog, CompileOptions{Lane: "full"})
	require.NoError(t, err)
	require.Len(t, plan2.ScenarioPlans, 1)
	assert.Equal(t, "full-1", plan2.ScenarioPlans[0].ScenarioID)
}

// TestCompileCatalogDefaultsLaneToFast asserts empty Lane option defaults to fast.
func TestCompileCatalogDefaultsLaneToFast(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-1", "core")
	scenario.QAMESHLaneRefs = []string{"fast"}

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{scenario},
	}, CompileOptions{})
	require.NoError(t, err)

	assert.Equal(t, "fast", plan.SelectedLane)
	require.Len(t, plan.ScenarioPlans, 1)
}

// TestCompileCatalogIncludesRejectedScenariosInRejectedList asserts scenarios with
// reject reasons appear in the rejected list, not the plan list.
func TestCompileCatalogIncludesRejectedScenariosInRejectedList(t *testing.T) {
	t.Parallel()

	rejected := sampleScenario("bad-action", "core")
	rejected.RequestedActions = []string{"production_deploy"}
	rejected.QAMESHLaneRefs = []string{"fast"}

	clean := sampleScenario("clean", "core")
	clean.ScenarioID = "clean"
	clean.QAMESHLaneRefs = []string{"fast"}

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{rejected, clean},
	}, CompileOptions{Lane: "fast"})
	require.NoError(t, err)

	require.Len(t, plan.RejectedScenarios, 1)
	assert.Equal(t, "bad-action", plan.RejectedScenarios[0].ScenarioID)
}

// TestCompileCatalogAdapterPopulatedFromCommand asserts the plan's Adapter field is
// populated from the effective command's adapter.
func TestCompileCatalogAdapterPopulatedFromCommand(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-cmd", "core")
	scenario.SafeExecutionEnvironment.Command = &CommandShape{
		Adapter: "go-test",
		Argv:    []string{"go", "test", "./..."},
	}
	scenario.QAMESHLaneRefs = []string{"fast"}

	plan, err := CompileCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{scenario},
	}, CompileOptions{Lane: "fast"})
	require.NoError(t, err)

	require.Len(t, plan.ScenarioPlans, 1)
	assert.Equal(t, "go-test", plan.ScenarioPlans[0].Adapter)
}

// TestBuildSetupGapReportPopulatesDefaultIdentifiers asserts the report fills in
// default suite/run/workspace IDs when opts are empty.
func TestBuildSetupGapReportPopulatesDefaultIdentifiers(t *testing.T) {
	t.Parallel()

	report, err := BuildSetupGapReport(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios:     []Scenario{sampleScenario("core-1", "core")},
	}, ReportOptions{})
	require.NoError(t, err)

	assert.Equal(t, ReportSchemaVersion, report.SchemaVersion)
	assert.Equal(t, "domain-readiness", report.SuiteID)
	assert.Equal(t, "domain-readiness-plan", report.RunID)
	assert.NotEmpty(t, report.WorkspaceID)
	assert.Equal(t, 1, report.EvidenceCount)
	assert.Len(t, report.Evidence, 1)
	assert.Equal(t, DomainReadinessStateSetupGap, report.Evidence[0].DomainReadinessState)
}

// TestBuildSetupGapReportUsesCatalogSuiteIDWhenOptsEmpty asserts SuiteID from
// catalog is preferred over the default fallback.
func TestBuildSetupGapReportUsesCatalogSuiteIDWhenOptsEmpty(t *testing.T) {
	t.Parallel()

	report, err := BuildSetupGapReport(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		SuiteID:       "my-suite",
		Scenarios:     []Scenario{sampleScenario("core-1", "core")},
	}, ReportOptions{RunID: "run-x", WorkspaceID: "ws-y"})
	require.NoError(t, err)

	assert.Equal(t, "my-suite", report.SuiteID)
	assert.Equal(t, "run-x", report.RunID)
	assert.Equal(t, "ws-y", report.WorkspaceID)
}

// TestUpsertScenarioResultUpdatesExistingEntry asserts that a second call with the
// same ScenarioID overwrites the earlier result in-place.
func TestUpsertScenarioResultUpdatesExistingEntry(t *testing.T) {
	t.Parallel()

	// Prime with a passed entry.
	first := ScenarioEvidenceInput{
		Scenario:            sampleScenario("scn-x", "core"),
		Result:              ScenarioResultPassed,
		SourceRefs:          []string{"src"},
		ActualEvidenceRefs:  []string{"evid"},
		Freshness:           EvidenceFreshnessCurrent,
		EvidenceCapturedAt:  time.Now(),
		RedactionStatus:     RedactionStatusPassed,
		RetentionClass:      RetentionClassMetadataOnly,
	}

	row, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "s",
		RunID:       "r",
		WorkspaceID: "w",
		Scenarios:   []ScenarioEvidenceInput{first},
	})
	require.NoError(t, err)
	require.Len(t, row.ScenarioResults, 1)
	assert.Equal(t, ScenarioResultPassed, row.ScenarioResults[0].ScenarioResult)

	// Now test upsertScenarioResult directly: same ID gets updated.
	results := row.ScenarioResults
	updated := upsertScenarioResult(results, ScenarioResultEntry{
		ScenarioID:     "scn-x",
		ScenarioResult: ScenarioResultFailed,
	})
	require.Len(t, updated, 1)
	assert.Equal(t, ScenarioResultFailed, updated[0].ScenarioResult)
}

// TestActionFindingNormalizesSpecialChars asserts actionFinding converts
// hyphens, spaces, slashes, colons to underscores and collapses doubles.
func TestActionFindingNormalizesSpecialChars(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"production_deploy", "production_deploy"},
		{"Production-Deploy", "production_deploy"},
		{"send email", "send_email"},
		{"a//b", "a_b"},
		{"x::y", "x_y"},
		{"__leading", "leading"},
		{"trailing__", "trailing"},
		{"double__under", "double_under"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, actionFinding(tc.input))
		})
	}
}

// TestAppendStarterScenarioSkipsDuplicateDomain asserts the helper adds a scenario
// only when no existing scenario shares the domain.
func TestAppendStarterScenarioSkipsDuplicateDomain(t *testing.T) {
	t.Parallel()

	existing := []Scenario{sampleScenario("existing", "core")}
	dup := sampleScenario("another", "core")
	newDomain := sampleScenario("auth-1", "auth")

	// Duplicate domain must not be added.
	result := appendStarterScenario(existing, dup)
	assert.Len(t, result, 1)
	assert.Equal(t, "existing", result[0].ScenarioID)

	// Different domain must be added.
	result2 := appendStarterScenario(existing, newDomain)
	assert.Len(t, result2, 2)
	assert.Equal(t, "auth-1", result2[1].ScenarioID)
}

// TestContainsStringBothBranches asserts the helper returns true for a matching
// element and false when no element matches.
func TestContainsStringBothBranches(t *testing.T) {
	t.Parallel()

	list := []string{"fast", "full", "nightly"}
	assert.True(t, containsString(list, "full"))
	assert.False(t, containsString(list, "smoke"))
	assert.False(t, containsString(nil, "fast"))
}
