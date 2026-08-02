package omp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

// OMPModelProjectionInput is a provider-neutral bridge from the routing
// resolver into OMP-native config and agent rendering.
type OMPModelProjectionInput struct {
	Capabilities []OMPProjectionCapability
	AgentNames   []string
}

// OMPProjectionCapability is one already-resolved semantic capability.
type OMPProjectionCapability struct {
	Capability string
	Selector   string
	Thinking   string
	Fallbacks  []OMPProjectionCandidate
}

// OMPProjectionCandidate is one ordered, exact availability fallback.
type OMPProjectionCandidate struct {
	Selector string
	Thinking string
}

// OMPModelProjection is the canonical OMP-native representation consumed by
// config activation and agent generation.
type OMPModelProjection struct {
	ModelRoles     []OMPModelRoleProjection
	FallbackChains []OMPFallbackChainProjection
	Agents         []OMPAgentModelProjection
}

type OMPModelRoleProjection struct {
	Role       string
	Capability string
	Selector   string
}

type OMPFallbackChainProjection struct {
	Selector   string
	Candidates []string
}

type OMPAgentModelProjection struct {
	Agent             string
	Role              string
	Model             string
	Thinking          string
	EffectiveSelector string
}

// CompileOMPModelProjection deterministically expands six semantic capability
// resolutions into all ten OMP roles and the exact current Autopus agent set.
func CompileOMPModelProjection(input OMPModelProjectionInput) (OMPModelProjection, error) {
	capabilities, err := validateOMPProjectionCapabilities(input.Capabilities)
	if err != nil {
		return OMPModelProjection{}, err
	}
	if err := validateOMPProjectionAgentSet(input.AgentNames); err != nil {
		return OMPModelProjection{}, err
	}

	projection := OMPModelProjection{
		ModelRoles: make([]OMPModelRoleProjection, 0, len(ompProjectionRoleSpecs)),
		Agents:     make([]OMPAgentModelProjection, 0, len(config.OMPAgentRoleMapping())),
	}
	roleResults := make(map[string]OMPProjectionCapability, len(ompProjectionRoleSpecs))
	fallbacksBySelector := make(map[string][]string)
	for _, spec := range ompProjectionRoleSpecs {
		resolved := capabilities[spec.capability]
		selector := formatOMPProjectedSelector(resolved.Selector, resolved.Thinking)
		projection.ModelRoles = append(projection.ModelRoles, OMPModelRoleProjection{
			Role: spec.role, Capability: spec.capability, Selector: selector,
		})
		roleResults[spec.role] = resolved
		if len(resolved.Fallbacks) > 0 {
			chain := OMPFallbackChainProjection{Selector: selector}
			for _, fallback := range resolved.Fallbacks {
				chain.Candidates = append(chain.Candidates,
					formatOMPProjectedSelector(fallback.Selector, fallback.Thinking))
			}
			if previous, exists := fallbacksBySelector[selector]; exists {
				if !equalOMPProjectedStrings(previous, chain.Candidates) {
					return OMPModelProjection{}, fmt.Errorf("fallback_chain_conflict: %s", selector)
				}
			} else {
				fallbacksBySelector[selector] = append([]string(nil), chain.Candidates...)
				projection.FallbackChains = append(projection.FallbackChains, chain)
			}
		}
	}

	agents := append([]string(nil), input.AgentNames...)
	sort.Strings(agents)
	for _, agent := range agents {
		role, roleErr := config.OMPAgentRole(agent)
		if roleErr != nil {
			return OMPModelProjection{}, roleErr
		}
		resolved := roleResults[role]
		projection.Agents = append(projection.Agents, OMPAgentModelProjection{
			Agent:             agent,
			Role:              role,
			Model:             "@" + role,
			Thinking:          resolved.Thinking,
			EffectiveSelector: formatOMPProjectedSelector(resolved.Selector, resolved.Thinking),
		})
	}
	return projection, nil
}

func validateOMPProjectionCapabilities(
	inputs []OMPProjectionCapability,
) (map[string]OMPProjectionCapability, error) {
	capabilities := make(map[string]OMPProjectionCapability, len(inputs))
	for _, input := range inputs {
		if _, ok := ompProjectionCapabilities[input.Capability]; !ok {
			return nil, fmt.Errorf("capability_unknown: %q", input.Capability)
		}
		if _, exists := capabilities[input.Capability]; exists {
			return nil, fmt.Errorf("capability_duplicate: %q", input.Capability)
		}
		if err := validateOMPProjectedSelector(input.Selector, input.Thinking); err != nil {
			return nil, fmt.Errorf("capability %s: %w", input.Capability, err)
		}
		for index, fallback := range input.Fallbacks {
			if err := validateOMPProjectedSelector(fallback.Selector, fallback.Thinking); err != nil {
				return nil, fmt.Errorf("capability %s fallback[%d]: %w", input.Capability, index, err)
			}
		}
		capabilities[input.Capability] = input
	}
	for capability := range ompProjectionCapabilities {
		if _, ok := capabilities[capability]; !ok {
			return nil, fmt.Errorf("capability_missing: %s", capability)
		}
	}
	return capabilities, nil
}

func validateOMPProjectedSelector(selector, thinking string) error {
	parts := strings.Split(selector, "/")
	if len(parts) != 2 || !isOMPProjectionIdentifier(parts[0]) || !isOMPProjectionIdentifier(parts[1]) {
		return fmt.Errorf("selector_invalid: %q", selector)
	}
	if !ompProjectionThinkingLevels[thinking] {
		return fmt.Errorf("thinking_unsupported: %q", thinking)
	}
	return nil
}

func isOMPProjectionIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || (index > 0 && strings.ContainsRune("._-", char)) {
			continue
		}
		return false
	}
	return true
}

func formatOMPProjectedSelector(selector, thinking string) string {
	return selector + ":" + thinking
}

func equalOMPProjectedStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func splitOMPProjectedSelector(value string) (string, string, error) {
	index := strings.LastIndexByte(value, ':')
	if index <= 0 || index == len(value)-1 {
		return "", "", fmt.Errorf("selector_invalid: %q", value)
	}
	return value[:index], value[index+1:], nil
}
