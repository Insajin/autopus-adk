package domainreadiness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type starterPackage struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func StarterCatalogForProject(projectDir string) Catalog {
	scenarios := []Scenario{coreStarterScenario()}
	pkg := readStarterPackage(projectDir)
	if hasStarterBrowserSignals(projectDir, pkg) {
		scenarios = appendStarterScenario(scenarios, browserStarterScenario())
	}
	if hasStarterAuthSignals(projectDir, pkg) {
		scenarios = appendStarterScenario(scenarios, authStarterScenario())
	}
	if hasStarterDesktopSignals(projectDir) {
		scenarios = appendStarterScenario(scenarios, desktopStarterScenario())
	}
	if hasStarterBuildSignals(projectDir, pkg) {
		scenarios = appendStarterScenario(scenarios, buildStarterScenario())
	}
	return Catalog{
		SchemaVersion:   CatalogSchemaVersion,
		SuiteID:         "project-domain-readiness",
		RequiredDomains: starterRequiredDomains(scenarios),
		Scenarios:       scenarios,
	}
}

func coreStarterScenario() Scenario {
	return contractStarterScenario("project-core-readiness", "core", []string{"fast", "full"}, []string{"fast"}, []string{"SPEC-QAMESH-002"})
}

func buildStarterScenario() Scenario {
	return contractStarterScenario("project-build-readiness", "build", []string{"fast", "full"}, []string{"node-build-fast", "fast"}, []string{"SPEC-QAMESH-002", "SPEC-QAMESH-004"})
}

func contractStarterScenario(id, domain string, lanes, journeys, specs []string) Scenario {
	return Scenario{
		SchemaVersion:           ScenarioSchemaVersion,
		ScenarioID:              id,
		Domain:                  domain,
		Owner:                   "project-owner",
		OwningRepo:              ".",
		SourceSpecRefs:          specs,
		ScenarioMode:            ScenarioModeContractTest,
		MutationBoundary:        MutationBoundaryReadOnly,
		FixtureOrSourceNeed:     []string{"project-owned deterministic test evidence"},
		JourneyPackRefs:         journeys,
		QAMESHLaneRefs:          lanes,
		ExpectedEvidence:        []string{"deterministic_check_result"},
		PassFailOracle:          []string{"exit_code == 0"},
		FreshnessWindowHours:    24,
		ForbiddenActions:        starterForbiddenActions(),
		LaunchQualityDomain:     domain,
		BackendContractTestRefs: []string{},
		SafeExecutionEnvironment: SafeExecutionEnvironment{
			Kind:        "local_safe_shell",
			Environment: "local",
			CWD:         ".",
			Timeout:     "5m",
		},
	}
}

func browserStarterScenario() Scenario {
	return guiStarterScenario("project-browser-gui-readiness", "browser", []string{"browser-staging", "gui-explore", "full"}, []string{"browser-staging-playwright", "gui-explore"})
}

func authStarterScenario() Scenario {
	scenario := guiStarterScenario("project-auth-safe-shell-readiness", "auth", []string{"browser-staging", "gui-explore", "full"}, []string{"browser-auth-safe-shell-gui-explore", "gui-explore"})
	scenario.FixtureOrSourceNeed = []string{"seeded non-production auth state or safe-shell auth route evidence"}
	scenario.ForbiddenActions = appendUniqueString(scenario.ForbiddenActions, "account_creation")
	scenario.ForbiddenActions = appendUniqueString(scenario.ForbiddenActions, "email_send")
	return scenario
}

func desktopStarterScenario() Scenario {
	scenario := guiStarterScenario("project-desktop-gui-readiness", "desktop", []string{"desktop-native", "gui-explore", "full"}, []string{"desktop-gui-explore"})
	scenario.SafeExecutionEnvironment.AllowedOrigins = []string{"http://127.0.0.1:1420", "http://localhost:1420"}
	scenario.DesktopTypedCardTestRefs = []string{"desktop-card:app-shell"}
	return scenario
}

func guiStarterScenario(id, domain string, lanes, journeys []string) Scenario {
	return Scenario{
		SchemaVersion:             ScenarioSchemaVersion,
		ScenarioID:                id,
		Domain:                    domain,
		Owner:                     "project-owner",
		OwningRepo:                ".",
		SourceSpecRefs:            []string{"SPEC-QAMESH-003", "SPEC-QAMESH-005"},
		ScenarioMode:              ScenarioModeGUISafeShell,
		MutationBoundary:          MutationBoundaryReadOnly,
		FixtureOrSourceNeed:       []string{"project-owned route map and redacted GUI journey evidence"},
		JourneyPackRefs:           journeys,
		QAMESHLaneRefs:            lanes,
		FrontendTypedCardTestRefs: []string{"frontend-card:app-shell"},
		ExpectedEvidence:          []string{"qamesh_manifest_ref", "gui_journey_graph_ref", "redacted_screenshot_ref"},
		PassFailOracle:            []string{"deterministic journey pack checks pass", "no console or network blockers above threshold"},
		FreshnessWindowHours:      24,
		ForbiddenActions:          starterForbiddenActions(),
		LaunchQualityDomain:       domain,
		SafeExecutionEnvironment: SafeExecutionEnvironment{
			Kind:             "gui_safe_shell",
			Environment:      "local-or-staging",
			CWD:              ".",
			Timeout:          "10m",
			AllowedOrigins:   []string{"http://127.0.0.1:*", "http://localhost:*"},
			SelectorStrategy: "role-first",
		},
	}
}

func starterForbiddenActions() []string {
	return []string{"production_mutation", "provider_write", "raw_payload_retention", "payment"}
}

func appendStarterScenario(scenarios []Scenario, next Scenario) []Scenario {
	for _, existing := range scenarios {
		if existing.Domain == next.Domain {
			return scenarios
		}
	}
	return append(scenarios, next)
}

func starterRequiredDomains(scenarios []Scenario) []string {
	domains := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		domains = appendUniqueString(domains, scenario.Domain)
	}
	return domains
}

func readStarterPackage(projectDir string) starterPackage {
	body, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		return starterPackage{}
	}
	var pkg starterPackage
	if err := json.Unmarshal(body, &pkg); err != nil {
		return starterPackage{}
	}
	return pkg
}

func hasStarterBrowserSignals(projectDir string, pkg starterPackage) bool {
	for _, rel := range []string{"playwright.config.ts", "playwright.config.js", "vite.config.ts", "vite.config.js", "next.config.ts", "next.config.js", "src/app", "src/pages", "app", "pages"} {
		if starterExists(projectDir, rel) {
			return true
		}
	}
	for _, name := range []string{"@playwright/test", "next", "vite", "react"} {
		if starterHasPackage(pkg, name) {
			return true
		}
	}
	return false
}

func hasStarterAuthSignals(projectDir string, pkg starterPackage) bool {
	for _, rel := range []string{"src/app/login", "src/app/auth", "src/pages/login", "app/login", "pages/login", "src/features/auth", "src/lib/auth", "auth.config.ts", "middleware.ts"} {
		if starterExists(projectDir, rel) {
			return true
		}
	}
	for _, name := range []string{"next-auth", "@auth/core", "better-auth", "lucia", "@supabase/supabase-js"} {
		if starterHasPackage(pkg, name) {
			return true
		}
	}
	return false
}

func hasStarterDesktopSignals(projectDir string) bool {
	for _, rel := range []string{"src-tauri/Cargo.toml", "src-tauri/tauri.conf.json", "tauri.conf.json"} {
		if starterExists(projectDir, rel) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(filepath.Base(projectDir)), "desktop")
}

func hasStarterBuildSignals(projectDir string, pkg starterPackage) bool {
	return starterHasScript(pkg, "build") || starterExists(projectDir, "go.mod") || starterExists(projectDir, "Cargo.toml")
}

func starterExists(projectDir, rel string) bool {
	_, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel)))
	return err == nil
}

func starterHasPackage(pkg starterPackage, name string) bool {
	_, ok := pkg.Dependencies[name]
	if ok {
		return true
	}
	_, ok = pkg.DevDependencies[name]
	return ok
}

func starterHasScript(pkg starterPackage, name string) bool {
	_, ok := pkg.Scripts[name]
	return ok
}
