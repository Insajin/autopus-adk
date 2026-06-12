package domainreadiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateScenarioPassesForFullyValidScenario asserts the happy path keeps
// a complete scenario valid and exercises the safe command shape path.
func TestValidateScenarioPassesForFullyValidScenario(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-readiness", "core")
	scenario.SafeExecutionEnvironment.Command = &CommandShape{
		Adapter: "go-test",
		Argv:    []string{"go", "test", "./..."},
		Timeout: "5m",
	}

	result := ValidateScenario(scenario)

	require.True(t, result.Valid)
	assert.Equal(t, ScenarioResultPassed, result.ScenarioResult)
	assert.Empty(t, result.Findings)
	assert.Empty(t, result.Blockers)
	assert.Empty(t, result.RejectReasons)
}

// TestValidateScenarioFlagsAllMissingFields asserts each required field maps to a
// specific finding and the scenario becomes blocked.
func TestValidateScenarioFlagsAllMissingFields(t *testing.T) {
	t.Parallel()

	result := ValidateScenario(Scenario{})

	assert.False(t, result.Valid)
	for _, expected := range []string{
		"invalid_schema_version",
		"missing_scenario_id",
		"missing_domain",
		"missing_owner",
		"missing_owning_repo",
		"missing_source_spec_refs",
		"missing_scenario_mode",
		"missing_mutation_boundary",
		"missing_fixture_or_source_need",
		"missing_expected_evidence",
		"missing_pass_fail_oracle",
		"missing_freshness_window_hours",
		"missing_forbidden_actions",
		"missing_safe_execution_environment",
		"missing_launch_quality_domain",
	} {
		assert.Contains(t, result.Findings, expected)
	}
	// Source evidence gaps push reject reasons; the result becomes rejected only
	// for command/mutation reasons, here it is setup_gap (gaps present, no reject).
	assert.Equal(t, ScenarioResultSetupGap, result.ScenarioResult)
	assert.Contains(t, result.SetupGaps, "missing_expected_evidence")
	assert.Contains(t, result.RejectReasons, UnsafeReasonSourceEvidenceMissing)
}

// TestValidateScenarioRejectsMutationAction asserts a production-mutation requested
// action yields rejected scenario result with provider-write reason.
func TestValidateScenarioRejectsMutationAction(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-readiness", "core")
	scenario.RequestedActions = []string{"production_deploy"}

	result := ValidateScenario(scenario)

	assert.Equal(t, ScenarioResultRejected, result.ScenarioResult)
	assert.Contains(t, result.RejectReasons, UnsafeReasonProductionMutationForbidden)
	assert.Contains(t, result.RejectReasons, UnsafeReasonProviderWriteNotAllowed)
	assert.Contains(t, result.Findings, "unsafe_action:production_deploy")
}

// TestValidateScenarioRejectsBroadScraping asserts broad scraping classification.
func TestValidateScenarioRejectsBroadScraping(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-readiness", "core")
	scenario.RequestedActions = []string{"scrape_all_pages"}

	result := ValidateScenario(scenario)

	assert.Equal(t, ScenarioResultRejected, result.ScenarioResult)
	assert.Contains(t, result.RejectReasons, UnsafeReasonBroadScrapingNotAllowed)
	assert.Contains(t, result.Findings, "unsafe_action:broad_scraping")
}

// TestValidateScenarioFlagsInventedCommand asserts an unknown adapter command is
// flagged as invented and rejected.
func TestValidateScenarioFlagsInventedCommand(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-readiness", "core")
	scenario.SafeExecutionEnvironment.Command = &CommandShape{
		Adapter: "go-test",
		Argv:    []string{"rm", "-rf", "/"},
	}

	result := ValidateScenario(scenario)

	assert.Equal(t, ScenarioResultRejected, result.ScenarioResult)
	assert.Contains(t, result.Findings, "invented_command")
	assert.Contains(t, result.RejectReasons, UnsafeReasonInventedCommand)
}

// TestValidateScenarioFlagsUnsafeShellAndCwd asserts shell metacharacters, abs cwd,
// bad timeout, and secret env allowlist all surface as findings.
func TestValidateScenarioFlagsUnsafeShellAndCwd(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("core-readiness", "core")
	scenario.SafeExecutionEnvironment.Command = &CommandShape{
		Adapter:      "custom-command",
		Run:          "go test && rm -rf",
		CWD:          "/etc",
		Timeout:      "90m",
		EnvAllowlist: []string{"SECRET_TOKEN"},
	}

	result := ValidateScenario(scenario)

	assert.Equal(t, ScenarioResultRejected, result.ScenarioResult)
	assert.Contains(t, result.Findings, "unsafe_shell")
	assert.Contains(t, result.Findings, "unsafe_cwd")
	assert.Contains(t, result.Findings, "unsafe_timeout")
	assert.Contains(t, result.Findings, "unsafe_env")
	assert.Contains(t, result.RejectReasons, UnsafeReasonUnsafeCommand)
}

// TestValidateScenarioGUISafeShellRequiresOriginsAndSelector asserts GUI mode needs
// allowed origins and a role/accessibility-first selector strategy.
func TestValidateScenarioGUISafeShellRequiresOriginsAndSelector(t *testing.T) {
	t.Parallel()

	scenario := sampleScenario("gui-readiness", "browser")
	scenario.ScenarioMode = ScenarioModeGUISafeShell

	result := ValidateScenario(scenario)

	assert.Contains(t, result.Findings, "missing_allowed_origins")
	assert.Contains(t, result.Findings, "missing_role_or_accessibility_first_selector_strategy")
	assert.Contains(t, result.Blockers, "missing_allowed_origins")

	scenario.SafeExecutionEnvironment.AllowedOrigins = []string{"https://staging.example.com"}
	scenario.SafeExecutionEnvironment.SelectorStrategy = "role-first"
	ok := ValidateScenario(scenario)
	assert.NotContains(t, ok.Findings, "missing_allowed_origins")
	assert.NotContains(t, ok.Findings, "missing_role_or_accessibility_first_selector_strategy")
}

// TestValidateCatalogReportsCoveredAndMissingDomains drives ValidateCatalog through
// the seen-domains, required-domains, and empty branches.
func TestValidateCatalogReportsCoveredAndMissingDomains(t *testing.T) {
	t.Parallel()

	// Empty catalog: invalid with scenario missing-domain.
	empty := ValidateCatalog(Catalog{SchemaVersion: CatalogSchemaVersion})
	assert.False(t, empty.Valid)
	assert.Equal(t, []string{"scenario"}, empty.MissingDomains)

	// No required domains: covered domains derive from scenarios, sorted.
	noReq := ValidateCatalog(Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Scenarios: []Scenario{
			sampleScenario("core-1", "core"),
			sampleScenario("auth-1", "auth"),
		},
	})
	assert.True(t, noReq.Valid)
	assert.Equal(t, []string{"auth", "core"}, noReq.CoveredDomains)

	// Bad schema version flips validity.
	badSchema := ValidateCatalog(Catalog{
		SchemaVersion: "wrong",
		Scenarios:     []Scenario{sampleScenario("core-1", "core")},
	})
	assert.False(t, badSchema.Valid)
}
