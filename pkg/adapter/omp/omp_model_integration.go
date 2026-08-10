package omp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompModelIntegration struct {
	profileName string
	profile     config.RoleModelProfileConf
	probe       OMPModelCatalogProbeResult
	routing     OMPModelRoutingCompilation
	projection  OMPModelProjection
	agents      []adapter.FileMapping
}

// WithModelIntegrationRunner injects metadata and config-readback execution.
func (a *Adapter) WithModelIntegrationRunner(runner OMPModelCatalogRunner) *Adapter {
	a.modelIntegrationRunner = runner
	return a
}

// WithModelIntegrationClock makes generated_at testable without changing its digest.
func (a *Adapter) WithModelIntegrationClock(clock func() time.Time) *Adapter {
	a.modelIntegrationClock = clock
	return a
}

// @AX:WARN [AUTO]: model integration preparation contains 10 if branches.
// @AX:REASON [AUTO]: policy opt-in, probe evidence, catalog normalization, routing, projection, and receipt preparation are fail-closed.
func (a *Adapter) prepareModelIntegration(
	ctx context.Context,
	cfg *config.HarnessConfig,
) (*ompModelIntegration, error) {
	if cfg.RoleModelPolicy.Profile == "" {
		return nil, nil
	}
	if err := cfg.RoleModelPolicy.Validate(); err != nil {
		return nil, err
	}
	profileName, profile, ok := cfg.RoleModelPolicy.SelectedRoleModelProfileForQuality(cfg.Quality)
	if !ok {
		return nil, fmt.Errorf("role_model_policy.profile_unknown: %q", profileName)
	}
	if !profile.FamilyDiversity.Enabled {
		return nil, fmt.Errorf("family_diversity_required")
	}
	if err := validateOMPIntegrationOverrides(profile); err != nil {
		return nil, err
	}
	probe, err := a.probeIntegratedModelCatalog(ctx, profile)
	if err != nil {
		return nil, err
	}
	routes, err := bridgeOMPIntegrationRoutes(profile)
	if err != nil {
		return nil, err
	}
	routing := CompileOMPModelRouting(OMPModelRoutingInput{
		Catalog: probe.Catalog, CatalogReason: probe.Reason, Routes: routes,
	})
	capabilities, err := projectOMPIntegrationCapabilities(probe.Catalog, routes, routing)
	if err != nil {
		return nil, err
	}
	agentNames := make([]string, 0, len(config.OMPAgentRoleMapping()))
	for name := range config.OMPAgentRoleMapping() {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)
	projection, err := CompileOMPModelProjection(OMPModelProjectionInput{
		Capabilities: capabilities, AgentNames: agentNames,
	})
	if err != nil {
		return nil, err
	}
	agents, err := a.prepareAgentMappingsWithProjection(projection)
	if err != nil {
		return nil, err
	}
	return &ompModelIntegration{
		profileName: profileName, profile: profile, probe: probe,
		routing: routing, projection: projection, agents: agents,
	}, nil
}

func (i *ompModelIntegration) prepareAgentMappings() ([]adapter.FileMapping, error) {
	return append([]adapter.FileMapping(nil), i.agents...), nil
}

func isOwnerOnlyOMPModelPath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(path))
	return normalized == configFile || normalized == DefaultOMPModelOverlayPath ||
		normalized == OMPModelReceiptRelativePath ||
		normalized == OMPModelProjectOwnershipRelativePath
}
