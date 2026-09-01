package cli

// Domain-readiness projection for `auto qa full`. Split out of
// qa_full_payload.go to keep both files under the 300-line source limit.

import (
	"fmt"
	"os"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
)

type qaFullDomainReadiness struct {
	Status      string                          `json:"status"`
	CatalogPath string                          `json:"catalog_path"`
	SetupGap    string                          `json:"setup_gap,omitempty"`
	Plan        *domainreadiness.CompileSummary `json:"plan,omitempty"`
}

func loadQAFullDomainReadiness(projectDir string) qaFullDomainReadiness {
	catalogPath := domainreadiness.ResolveCatalogPath(projectDir, domainreadiness.DefaultCatalogPath)
	catalog, err := domainreadiness.LoadCatalogFile(catalogPath)
	if err != nil {
		status := "error"
		setupGap := err.Error()
		if os.IsNotExist(err) {
			status = "setup_gap"
			setupGap = "domain readiness catalog is missing"
		}
		return qaFullDomainReadiness{Status: status, CatalogPath: catalogPath, SetupGap: setupGap}
	}
	plan, err := domainreadiness.CompileCatalog(catalog, domainreadiness.CompileOptions{ProjectDir: projectDir, Lane: "full"})
	if err != nil {
		return qaFullDomainReadiness{Status: "error", CatalogPath: catalogPath, SetupGap: err.Error()}
	}
	// plan.Valid already folds in the journey_pack_ref cross-check, so a catalog
	// naming Journey Packs the project lacks no longer reports "ready".
	status := "ready"
	if !plan.Valid || len(plan.MissingDomains) > 0 || len(plan.RejectedScenarios) > 0 {
		status = "setup_gap"
	}
	if status != "ready" && len(plan.JourneyRefGaps) > 0 {
		return qaFullDomainReadiness{
			Status:      status,
			CatalogPath: catalogPath,
			SetupGap:    fmt.Sprintf("%d domain readiness journey_pack_ref(s) do not resolve to a project Journey Pack", len(plan.JourneyRefGaps)),
			Plan:        &plan,
		}
	}
	return qaFullDomainReadiness{Status: status, CatalogPath: catalogPath, Plan: &plan}
}

func domainScenarioCount(domain qaFullDomainReadiness) int {
	if domain.Plan == nil {
		return 0
	}
	return domain.Plan.ScenarioCount
}
