package omp

import (
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompProjectionRoleSpec struct {
	role       string
	capability string
}

var ompProjectionRoleSpecs = []ompProjectionRoleSpec{
	{role: config.OMPRoleDefault, capability: config.CapabilityCodingToolUse},
	{role: config.OMPRoleSmol, capability: config.CapabilityFastValidation},
	{role: config.OMPRoleSlow, capability: config.CapabilityDeepReasoning},
	{role: config.OMPRolePlan, capability: config.CapabilityDeepReasoning},
	{role: config.OMPRoleVision, capability: config.CapabilityVisionDesign},
	{role: config.OMPRoleDesigner, capability: config.CapabilityVisionDesign},
	{role: config.OMPRoleCommit, capability: config.CapabilityDeterministicTransform},
	{role: config.OMPRoleTiny, capability: config.CapabilityDeterministicTransform},
	{role: config.OMPRoleTask, capability: config.CapabilityCodingToolUse},
	{role: config.OMPRoleAdvisor, capability: config.CapabilityIndependentDissent},
}

var ompProjectionCapabilities = func() map[string]struct{} {
	result := make(map[string]struct{})
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		result[capability] = struct{}{}
	}
	return result
}()

var ompProjectionThinkingLevels = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":   true,
}

func validateOMPProjectionAgentSet(agentNames []string) error {
	return config.ValidateOMPAgentRoleSet(agentNames)
}
