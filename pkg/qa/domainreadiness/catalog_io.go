package domainreadiness

import (
	"encoding/json"
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

// StarterCatalogResult reports what WriteStarterCatalog did. It mirrors the
// created/skipped contract `auto qa init` already uses so repeated provisioning
// runs are a no-op instead of an error.
type StarterCatalogResult struct {
	Status string `json:"status"`
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

// StarterCatalogResult.Status values.
const (
	StarterCatalogCreated = "created"
	StarterCatalogSkipped = "skipped"
)

// WriteStarterCatalog seeds a project-local catalog and is idempotent: an
// existing catalog is reported as skipped and never overwritten, because it may
// have been authored by hand since the first run.
func WriteStarterCatalog(projectDir, catalogPath string) (StarterCatalogResult, error) {
	path := ResolveCatalogPath(projectDir, catalogPath)
	if err := ValidateCatalogSource(path); err != nil {
		return StarterCatalogResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return StarterCatalogResult{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return StarterCatalogResult{
			Status: StarterCatalogSkipped,
			Path:   path,
			Reason: "existing project-local file preserved",
		}, nil
	} else if !os.IsNotExist(err) {
		return StarterCatalogResult{}, err
	}
	body, err := json.MarshalIndent(StarterCatalogForProject(projectDir), "", "  ")
	if err != nil {
		return StarterCatalogResult{}, err
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return StarterCatalogResult{}, err
	}
	return StarterCatalogResult{Status: StarterCatalogCreated, Path: path}, nil
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
		Scenarios:       []Scenario{contractStarterScenario("project-core-readiness", "core", []string{"fast"}, []string{"SPEC-QAMESH-002"})},
	}
}
