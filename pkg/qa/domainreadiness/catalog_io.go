package domainreadiness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadCatalogFile(path string) (Catalog, error) {
	if err := ValidateCatalogSource(path); err != nil {
		return Catalog{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func WriteStarterCatalog(projectDir, catalogPath string) (string, error) {
	path := ResolveCatalogPath(projectDir, catalogPath)
	if err := ValidateCatalogSource(path); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("catalog already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	body, err := json.MarshalIndent(StarterCatalog(), "", "  ")
	if err != nil {
		return "", err
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func ResolveCatalogPath(projectDir, catalogPath string) string {
	if strings.TrimSpace(catalogPath) == "" {
		catalogPath = DefaultCatalogPath
	}
	if filepath.IsAbs(catalogPath) {
		return filepath.Clean(catalogPath)
	}
	if strings.TrimSpace(projectDir) == "" {
		projectDir = "."
	}
	return filepath.Clean(filepath.Join(projectDir, filepath.FromSlash(catalogPath)))
}

func StarterCatalog() Catalog {
	return Catalog{
		SchemaVersion:   CatalogSchemaVersion,
		SuiteID:         "project-domain-readiness",
		RequiredDomains: []string{"core"},
		Scenarios: []Scenario{{
			SchemaVersion:           ScenarioSchemaVersion,
			ScenarioID:              "project-core-readiness",
			Domain:                  "core",
			Owner:                   "project-owner",
			OwningRepo:              ".",
			SourceSpecRefs:          []string{"SPEC-QAMESH-002"},
			ScenarioMode:            ScenarioModeContractTest,
			MutationBoundary:        MutationBoundaryReadOnly,
			FixtureOrSourceNeed:     []string{"project-owned deterministic test evidence"},
			JourneyPackRefs:         []string{"fast"},
			QAMESHLaneRefs:          []string{"fast"},
			ExpectedEvidence:        []string{"deterministic_check_result"},
			PassFailOracle:          []string{"exit_code == 0"},
			FreshnessWindowHours:    24,
			ForbiddenActions:        []string{"production_mutation", "provider_write", "raw_payload_retention"},
			LaunchQualityDomain:     "core",
			BackendContractTestRefs: []string{},
			SafeExecutionEnvironment: SafeExecutionEnvironment{
				Kind:        "local_safe_shell",
				Environment: "local",
				CWD:         ".",
				Timeout:     "5m",
			},
		}},
	}
}
