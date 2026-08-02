package config

import (
	"fmt"
	"sort"
)

const (
	CapabilityDeepReasoning          = "deep_reasoning"
	CapabilityCodingToolUse          = "coding_tool_use"
	CapabilityFastValidation         = "fast_validation"
	CapabilityVisionDesign           = "vision_design"
	CapabilityIndependentDissent     = "independent_dissent"
	CapabilityDeterministicTransform = "deterministic_transform"

	OMPRoleDefault  = "default"
	OMPRoleSmol     = "smol"
	OMPRoleSlow     = "slow"
	OMPRolePlan     = "plan"
	OMPRoleVision   = "vision"
	OMPRoleDesigner = "designer"
	OMPRoleCommit   = "commit"
	OMPRoleTiny     = "tiny"
	OMPRoleTask     = "task"
	OMPRoleAdvisor  = "advisor"
)

var providerNeutralCapabilities = []string{
	CapabilityDeepReasoning,
	CapabilityCodingToolUse,
	CapabilityFastValidation,
	CapabilityVisionDesign,
	CapabilityIndependentDissent,
	CapabilityDeterministicTransform,
}

var canonicalRoleByCapability = map[string]string{
	CapabilityDeepReasoning:          OMPRolePlan,
	CapabilityCodingToolUse:          OMPRoleTask,
	CapabilityFastValidation:         OMPRoleSmol,
	CapabilityVisionDesign:           OMPRoleVision,
	CapabilityIndependentDissent:     OMPRoleAdvisor,
	CapabilityDeterministicTransform: OMPRoleTiny,
}

var capabilityByNativeRole = map[string]string{
	OMPRoleDefault:  CapabilityCodingToolUse,
	OMPRoleSmol:     CapabilityFastValidation,
	OMPRoleSlow:     CapabilityDeepReasoning,
	OMPRolePlan:     CapabilityDeepReasoning,
	OMPRoleVision:   CapabilityVisionDesign,
	OMPRoleDesigner: CapabilityVisionDesign,
	OMPRoleCommit:   CapabilityDeterministicTransform,
	OMPRoleTiny:     CapabilityDeterministicTransform,
	OMPRoleTask:     CapabilityCodingToolUse,
	OMPRoleAdvisor:  CapabilityIndependentDissent,
}

var roleBySourceAgent = map[string]string{
	"annotator":           OMPRoleSmol,
	"architect":           OMPRoleSlow,
	"debugger":            OMPRoleTask,
	"deep-worker":         OMPRoleTask,
	"devops":              OMPRoleTask,
	"executor":            OMPRoleTask,
	"explorer":            OMPRoleSmol,
	"frontend-specialist": OMPRoleDesigner,
	"perf-engineer":       OMPRoleTask,
	"planner":             OMPRolePlan,
	"reviewer":            OMPRoleAdvisor,
	"security-auditor":    OMPRoleAdvisor,
	"spec-writer":         OMPRolePlan,
	"tester":              OMPRoleTask,
	"ux-validator":        OMPRoleVision,
	"validator":           OMPRoleTiny,
}

// OMPProviderNeutralCapabilities returns the canonical v1 capability order.
func OMPProviderNeutralCapabilities() []string {
	return append([]string(nil), providerNeutralCapabilities...)
}

// CanonicalOMPRoleForCapability maps a semantic capability to its default native role.
func CanonicalOMPRoleForCapability(capability string) (string, error) {
	role, ok := canonicalRoleByCapability[capability]
	if !ok {
		return "", fmt.Errorf("capability_unknown: %q", capability)
	}
	return role, nil
}

// OMPNativeRoleCapability returns the semantic capability owned by a native role.
func OMPNativeRoleCapability(role string) (string, error) {
	capability, ok := capabilityByNativeRole[role]
	if !ok {
		return "", fmt.Errorf("role_unknown: %q", role)
	}
	return capability, nil
}

// OMPAgentRole returns the exact native role for a generated source agent.
func OMPAgentRole(agent string) (string, error) {
	role, ok := roleBySourceAgent[agent]
	if !ok {
		return "", fmt.Errorf("agent_role_unmapped: %q", agent)
	}
	return role, nil
}

// OMPAgentRoleMapping returns a detached copy of the v1 source-agent matrix.
func OMPAgentRoleMapping() map[string]string {
	result := make(map[string]string, len(roleBySourceAgent))
	for agent, role := range roleBySourceAgent {
		result[agent] = role
	}
	return result
}

// ValidateRoleCapabilityPair rejects role overrides that alter the v1 matrix.
func ValidateRoleCapabilityPair(role, capability string) error {
	want, err := OMPNativeRoleCapability(role)
	if err != nil {
		return err
	}
	if want != capability {
		return fmt.Errorf("role_capability_mismatch: role %q requires %q, got %q", role, want, capability)
	}
	return nil
}

// ValidateOMPAgentRoleSet requires exact equality with the 16 generated agents.
func ValidateOMPAgentRoleSet(agents []string) error {
	seen := make(map[string]bool, len(agents))
	for _, agent := range agents {
		if _, err := OMPAgentRole(agent); err != nil {
			return err
		}
		if seen[agent] {
			return fmt.Errorf("agent_role_duplicate: %q", agent)
		}
		seen[agent] = true
	}
	missing := make([]string, 0)
	for agent := range roleBySourceAgent {
		if !seen[agent] {
			missing = append(missing, agent)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("agent_role_missing: %v", missing)
	}
	return nil
}

// LegacyTierRoute maps only the v1 opus/sonnet/haiku compatibility vocabulary.
func LegacyTierRoute(version, tier string) (LegacyRoleRoute, error) {
	if version != RoleModelPolicyVersionV1 {
		return LegacyRoleRoute{}, fmt.Errorf("policy_version_unknown: %q", version)
	}
	routes := map[string]LegacyRoleRoute{
		"opus":   {Capability: CapabilityDeepReasoning, Role: OMPRolePlan, LegacySource: "opus"},
		"sonnet": {Capability: CapabilityCodingToolUse, Role: OMPRoleTask, LegacySource: "sonnet"},
		"haiku":  {Capability: CapabilityDeterministicTransform, Role: OMPRoleTiny, LegacySource: "haiku"},
	}
	route, ok := routes[tier]
	if !ok {
		return LegacyRoleRoute{}, fmt.Errorf("legacy_tier_unknown: %q", tier)
	}
	return route, nil
}
