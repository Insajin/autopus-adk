package omp

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/config"
)

// validateOMPIntegrationOverrides re-checks the optional role/capability pins
// on agent overrides. config.Validate already rejects unmapped agents and
// candidate shape; only agreement with the agent matrix is confirmed here.
func validateOMPIntegrationOverrides(profile config.RoleModelProfileConf) error {
	for agent, override := range profile.Agents {
		role, err := config.OMPAgentRole(agent)
		if err != nil {
			return err
		}
		capability, err := config.OMPAgentCapability(agent)
		if err != nil {
			return err
		}
		if override.Role != "" && override.Role != role ||
			override.Capability != "" && override.Capability != capability {
			return fmt.Errorf("role_capability_mismatch: agent=%s", agent)
		}
	}
	return nil
}

// bridgeOMPIntegrationRoutes builds one route request per canonical agent,
// keyed by agent name. Candidates come from the agent override when present
// and otherwise from the agent's capability route.
func bridgeOMPIntegrationRoutes(
	profile config.RoleModelProfileConf,
) (map[string]OMPModelRouteRequest, error) {
	agents := config.CanonicalAgentNames()
	routes := make(map[string]OMPModelRouteRequest, len(agents))
	diverseRoles := make(map[string]struct{}, len(profile.FamilyDiversity.Roles))
	if profile.FamilyDiversity.Enabled {
		for _, role := range profile.FamilyDiversity.Roles {
			diverseRoles[role] = struct{}{}
		}
	}
	for _, agent := range agents {
		role, err := config.OMPAgentRole(agent)
		if err != nil {
			return nil, err
		}
		capability, err := config.OMPAgentCapability(agent)
		if err != nil {
			return nil, err
		}
		route, err := profile.AgentRoute(agent)
		if err != nil {
			return nil, err
		}
		_, preferDistinctFamily := diverseRoles[role]
		request := OMPModelRouteRequest{
			Agent: agent, Role: role, Capability: capability,
			Required: route.Required, DegradedAction: route.DegradedAction,
			PreferDistinctExecutorFamily: preferDistinctFamily,
			Candidates:                   make([]OMPRoutingCandidate, 0, len(route.Candidates)),
		}
		for _, candidate := range route.Candidates {
			request.Candidates = append(request.Candidates, OMPRoutingCandidate{
				Selector: candidate.Selector, Thinking: candidate.Thinking, Family: candidate.Family,
			})
		}
		routes[agent] = request
	}
	return routes, nil
}

// projectOMPIntegrationAgents keeps only agents whose route resolved to an
// exact selector; a required route that stays unresolved fails closed.
func projectOMPIntegrationAgents(
	catalog OMPModelCatalog,
	routes map[string]OMPModelRouteRequest,
	compilation OMPModelRoutingCompilation,
) ([]OMPProjectionAgent, error) {
	resolved := make(map[string]OMPModelRouteResolution, len(compilation.Resolutions))
	for _, resolution := range compilation.Resolutions {
		route, exists := routes[resolution.Agent]
		if !exists {
			return nil, fmt.Errorf("route_missing: %s", resolution.Agent)
		}
		if resolution.Status == "selected" && resolution.EffectiveSelector != "" {
			resolved[resolution.Agent] = resolution
			continue
		}
		if route.Required && route.DegradedAction != "runtime_default" {
			return nil, fmt.Errorf("required_route_unresolved: %s: %s", resolution.Agent, resolution.Reason)
		}
	}
	result := make([]OMPProjectionAgent, 0, len(resolved))
	for _, agent := range config.CanonicalAgentNames() {
		resolution, ok := resolved[agent]
		if !ok {
			continue
		}
		projection := OMPProjectionAgent{
			Agent: agent, Role: resolution.RequestedRole, Capability: resolution.Capability,
			Selector: resolution.EffectiveProvider + "/" + resolution.EffectiveModel,
			Thinking: resolution.Thinking,
		}
		for _, candidate := range routes[agent].Candidates {
			if formatOMPRoutingSelector(candidate) == resolution.EffectiveSelector {
				continue
			}
			if _, reason := matchOMPModelCandidate(catalog.Models, resolution.Capability, candidate); reason == "compatible" {
				projection.Fallbacks = append(projection.Fallbacks, OMPProjectionCandidate{
					Selector: candidate.Selector, Thinking: candidate.Thinking,
				})
			}
		}
		result = append(result, projection)
	}
	return result, nil
}
