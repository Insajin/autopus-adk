package omp

import (
	"context"
	"fmt"
	"sort"

	"github.com/insajin/autopus-adk/pkg/config"
)

func (a *Adapter) probeIntegratedModelCatalog(
	ctx context.Context,
	profile config.RoleModelProfileConf,
) (OMPModelCatalogProbeResult, error) {
	settings := []string{"modelRoles", "retry.fallbackChains", "retry.modelFallback"}
	if profile.Safety.ApprovalMode != "" {
		settings = append(settings, "tools.approvalMode")
	}
	if profile.Safety.IsolationMode != "" {
		settings = append(settings, "task.isolation.mode")
	}
	sort.Strings(settings)
	opts := OMPModelCatalogProbeOptions{
		Executable: cliBinary, Runner: a.modelIntegrationRunner, Settings: settings,
	}
	probe := ProbeOMPModelCatalog(ctx, opts)
	if probe.Reason == "catalog_metadata_insufficient" {
		normalized := normalizeOMPModelCatalogProbeOptions(opts)
		raw, reason := runOMPModelCatalogProbe(ctx, normalized, "models", "--json")
		if reason == "" {
			probe.Catalog, probe.Reason = NormalizeOMPAvailableCatalog(
				raw, normalized.MaxOutput, ompIntegrationCatalogDeclarations(profile),
			)
			if probe.Reason == "catalog_ready" {
				probe.Status = "ready"
			}
		}
	}
	if probe.Status != "ready" || probe.Reason != "catalog_ready" {
		return OMPModelCatalogProbeResult{}, fmt.Errorf("model_catalog_unavailable: %s", probe.Reason)
	}
	supported := make(map[string]bool, len(probe.Settings))
	for _, setting := range probe.Settings {
		supported[setting.Key] = setting.Supported
	}
	for _, setting := range settings {
		if !supported[setting] {
			return OMPModelCatalogProbeResult{}, fmt.Errorf("model_setting_unsupported: %s", setting)
		}
	}
	return probe, nil
}

func ompIntegrationCatalogDeclarations(
	profile config.RoleModelProfileConf,
) []OMPModelCatalogDeclaration {
	declarations := make([]OMPModelCatalogDeclaration, 0)
	for capability, route := range profile.Capabilities {
		for _, candidate := range route.Candidates {
			declarations = append(declarations, OMPModelCatalogDeclaration{
				Selector: candidate.Selector, Family: candidate.Family, Capability: capability,
			})
		}
	}
	sort.Slice(declarations, func(i, j int) bool {
		if declarations[i].Selector != declarations[j].Selector {
			return declarations[i].Selector < declarations[j].Selector
		}
		return declarations[i].Capability < declarations[j].Capability
	})
	return declarations
}
