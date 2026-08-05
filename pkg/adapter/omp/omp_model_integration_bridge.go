package omp

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/config"
)

func validateOMPIntegrationOverrides(profile config.RoleModelProfileConf) error {
	for agent, override := range profile.Agents {
		role, err := config.OMPAgentRole(agent)
		if err != nil {
			return err
		}
		capability, err := config.OMPNativeRoleCapability(role)
		if err != nil {
			return err
		}
		if override.Role != role || override.Capability != capability {
			return fmt.Errorf("role_capability_mismatch: agent=%s", agent)
		}
	}
	return nil
}

func bridgeOMPIntegrationRoutes(
	profile config.RoleModelProfileConf,
) (map[string]OMPModelRouteRequest, error) {
	routes := make(map[string]OMPModelRouteRequest, len(profile.Capabilities))
	diverseRoles := make(map[string]struct{}, len(profile.FamilyDiversity.Roles))
	if profile.FamilyDiversity.Enabled {
		for _, role := range profile.FamilyDiversity.Roles {
			diverseRoles[role] = struct{}{}
		}
	}
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		route, ok := profile.Capabilities[capability]
		if !ok {
			return nil, fmt.Errorf("capability_missing: %s", capability)
		}
		role, err := config.CanonicalOMPRoleForCapability(capability)
		if err != nil {
			return nil, err
		}
		_, preferDistinctFamily := diverseRoles[role]
		request := OMPModelRouteRequest{
			Role: role, Capability: capability, Required: route.Required,
			DegradedAction:               route.DegradedAction,
			PreferDistinctExecutorFamily: preferDistinctFamily,
			Candidates:                   make([]OMPRoutingCandidate, 0, len(route.Candidates)),
		}
		switch capability {
		case config.CapabilityCodingToolUse:
			request.Agent = "executor"
		case config.CapabilityIndependentDissent:
			request.Agent = "reviewer"
		}
		for _, candidate := range route.Candidates {
			request.Candidates = append(request.Candidates, OMPRoutingCandidate{
				Selector: candidate.Selector, Thinking: candidate.Thinking, Family: candidate.Family,
			})
		}
		routes[capability] = request
	}
	return routes, nil
}

func projectOMPIntegrationCapabilities(
	catalog OMPModelCatalog,
	routes map[string]OMPModelRouteRequest,
	compilation OMPModelRoutingCompilation,
) ([]OMPProjectionCapability, error) {
	resolved := make(map[string]OMPModelRouteResolution, len(compilation.Resolutions))
	for _, resolution := range compilation.Resolutions {
		route, exists := routes[resolution.Capability]
		if !exists {
			return nil, fmt.Errorf("route_missing: %s", resolution.Capability)
		}
		if resolution.Status == "selected" && resolution.EffectiveSelector != "" {
			resolved[resolution.Capability] = resolution
			continue
		}
		if route.Required && route.DegradedAction != "runtime_default" {
			return nil, fmt.Errorf("required_route_unresolved: %s: %s", resolution.Capability, resolution.Reason)
		}
	}
	result := make([]OMPProjectionCapability, 0, len(config.OMPProviderNeutralCapabilities()))
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		resolution, ok := resolved[capability]
		if !ok {
			continue
		}
		projection := OMPProjectionCapability{
			Capability: capability, Selector: resolution.EffectiveProvider + "/" + resolution.EffectiveModel,
			Thinking: resolution.Thinking,
		}
		for _, candidate := range routes[capability].Candidates {
			if formatOMPRoutingSelector(candidate) == resolution.EffectiveSelector {
				continue
			}
			if _, reason := matchOMPModelCandidate(catalog.Models, capability, candidate); reason == "compatible" {
				projection.Fallbacks = append(projection.Fallbacks, OMPProjectionCandidate{
					Selector: candidate.Selector, Thinking: candidate.Thinking,
				})
			}
		}
		result = append(result, projection)
	}
	return result, nil
}
