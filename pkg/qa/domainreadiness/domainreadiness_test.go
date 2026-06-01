package domainreadiness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStarterCatalogCompilesWithoutProductSpecificBaseline(t *testing.T) {
	t.Parallel()

	catalog := StarterCatalog()
	plan, err := CompileCatalog(catalog, CompileOptions{Lane: "fast"})
	require.NoError(t, err)

	assert.Equal(t, CatalogSchemaVersion, catalog.SchemaVersion)
	assert.Equal(t, PlanSchemaVersion, plan.SchemaVersion)
	assert.True(t, plan.Validation.Valid)
	assert.Equal(t, 1, plan.ScenarioCount)
	assert.Equal(t, []string{"core"}, plan.CoveredDomains)
	require.Len(t, plan.ScenarioPlans, 1)
	assert.Equal(t, "project-core-readiness", plan.ScenarioPlans[0].ScenarioID)
	assert.Empty(t, plan.ScenarioPlans[0].Adapter)
}

func TestStarterCatalogForProjectAddsBrowserAuthAndBuildDomains(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {"build": "next build"},
  "dependencies": {"next": "^16.0.0", "next-auth": "^5.0.0"}
}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "app", "login"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "next.config.ts"), []byte("export default {}\n"), 0o644))

	catalog := StarterCatalogForProject(dir)
	plan, err := CompileCatalog(catalog, CompileOptions{ProjectDir: dir, Lane: "full"})
	require.NoError(t, err)

	assert.True(t, plan.Validation.Valid)
	assert.ElementsMatch(t, []string{"core", "browser", "auth", "build"}, catalog.RequiredDomains)
	assert.Contains(t, plan.CoveredDomains, "browser")
	assert.Contains(t, plan.CoveredDomains, "auth")
	assert.Contains(t, plan.CoveredDomains, "build")
	assert.GreaterOrEqual(t, len(plan.ScenarioPlans), 4)
}

func TestStarterCatalogForProjectAddsDesktopDomain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname = \"desktop\"\n"), 0o644))

	catalog := StarterCatalogForProject(dir)
	plan, err := CompileCatalog(catalog, CompileOptions{ProjectDir: dir, Lane: "full"})
	require.NoError(t, err)

	assert.True(t, plan.Validation.Valid)
	assert.Contains(t, catalog.RequiredDomains, "desktop")
	assert.Contains(t, plan.CoveredDomains, "desktop")
}

func TestLoadCatalogFileRejectsGeneratedSurface(t *testing.T) {
	t.Parallel()

	_, err := LoadCatalogFile(filepath.Join(".autopus", "qa", "runs", "catalog.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated surface")
}

func TestValidateCatalogUsesCatalogRequiredDomains(t *testing.T) {
	t.Parallel()

	catalog := Catalog{
		SchemaVersion:   CatalogSchemaVersion,
		RequiredDomains: []string{"core", "payments"},
		Scenarios:       []Scenario{sampleScenario("core-readiness", "core")},
	}

	report := ValidateCatalog(catalog)

	assert.False(t, report.Valid)
	assert.Equal(t, []string{"core"}, report.CoveredDomains)
	assert.Equal(t, []string{"payments"}, report.MissingDomains)
}

func TestBuildDomainReadinessEvidenceRejectsUnsafeInputs(t *testing.T) {
	t.Parallel()

	row, err := BuildDomainReadinessEvidence(EvidenceBuildInput{
		SuiteID:     "suite",
		RunID:       "run",
		WorkspaceID: "workspace",
		Scenarios: []ScenarioEvidenceInput{{
			Scenario:               sampleScenario("core-readiness", "core"),
			Result:                 ScenarioResultPassed,
			SourceRefs:             []string{"source:core:summary"},
			ActualEvidenceRefs:     []string{"evidence:core:summary"},
			Freshness:              EvidenceFreshnessCurrent,
			EvidenceCapturedAt:     time.Now(),
			ProviderWriteCallCount: 1,
			RedactionStatus:        RedactionStatusPassed,
			RetentionClass:         RetentionClassMetadataOnly,
			RawPayloadPresent:      true,
		}},
	})
	require.NoError(t, err)

	assert.Equal(t, DomainReadinessStateUnsafe, row.DomainReadinessState)
	assert.False(t, row.DenominatorIncluded)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonRawPayloadNotAllowed)
	assert.Contains(t, row.UnsafeReasons, UnsafeReasonProviderWriteNotAllowed)
	assert.Equal(t, 1, row.ProviderWriteCallCount)
	require.Len(t, row.ScenarioResults, 1)
	assert.Equal(t, ScenarioResultRejected, row.ScenarioResults[0].ScenarioResult)
}

func TestBuildSetupGapReportUsesCatalogSuiteAndGenericAuditRefs(t *testing.T) {
	t.Parallel()

	report, err := BuildSetupGapReport(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		SuiteID:       "suite-project",
		Scenarios:     []Scenario{sampleScenario("core-readiness", "core")},
	}, ReportOptions{RunID: "run", WorkspaceID: "workspace"})
	require.NoError(t, err)

	assert.Equal(t, ReportSchemaVersion, report.SchemaVersion)
	assert.Equal(t, "suite-project", report.SuiteID)
	require.Len(t, report.Evidence, 1)
	row := report.Evidence[0]
	assert.Equal(t, "audit:domain_readiness:core-readiness", row.AuditRefs[0])
	assert.Equal(t, RetentionClassMetadataOnly, row.RetentionClass)
	assert.False(t, row.DenominatorIncluded)
}

func sampleScenario(id, domain string) Scenario {
	return Scenario{
		SchemaVersion:           ScenarioSchemaVersion,
		ScenarioID:              id,
		Domain:                  domain,
		Owner:                   "qa-owner",
		OwningRepo:              ".",
		SourceSpecRefs:          []string{"SPEC-QAMESH-002"},
		ScenarioMode:            ScenarioModeContractTest,
		MutationBoundary:        MutationBoundaryReadOnly,
		FixtureOrSourceNeed:     []string{"deterministic evidence"},
		JourneyPackRefs:         []string{"fast"},
		QAMESHLaneRefs:          []string{"fast"},
		ExpectedEvidence:        []string{"deterministic_check_result"},
		PassFailOracle:          []string{"exit_code == 0"},
		FreshnessWindowHours:    24,
		ForbiddenActions:        []string{"production_mutation", "provider_write"},
		LaunchQualityDomain:     domain,
		BackendContractTestRefs: []string{"backend-contract:" + id},
		SafeExecutionEnvironment: SafeExecutionEnvironment{
			Kind:        "local_safe_shell",
			Environment: "local",
			CWD:         ".",
			Timeout:     "5m",
		},
	}
}
