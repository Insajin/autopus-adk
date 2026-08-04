package config

import (
	"fmt"
	"strings"
)

func (c *HarnessConfig) validateModelSelectionRoleAndOMPContextPolicy() error {
	if err := c.validateModelSelectionAndRolePolicy(); err != nil {
		return err
	}
	return c.OMPContextPolicy.Validate()
}

// ValidateOMPContextPolicy is the canonical adapter and CLI validation entrypoint.
func ValidateOMPContextPolicy(policy OMPContextPolicyConf) error {
	return policy.Validate()
}

// Validate rejects ambiguous modes and any user-owned or global mutation claim.
func (c OMPContextPolicyConf) Validate() error {
	if c.Profile == "" && len(c.Profiles) == 0 {
		return nil
	}
	if c.Profile != "" {
		if _, ok := c.Profiles[c.Profile]; !ok {
			return fmt.Errorf("omp_context_policy.profile_unknown: %q", c.Profile)
		}
	}
	for name, profile := range c.Profiles {
		if !IsValidQualityPresetName(name) {
			return fmt.Errorf("omp_context_policy.profile_name_invalid: %q", name)
		}
		if err := validateOMPContextProfile(name, profile); err != nil {
			return err
		}
	}
	return nil
}

// @AX:WARN [AUTO]: context-profile validation has cyclomatic complexity 23.
// @AX:REASON [AUTO]: gocyclo reports 23 across history, memory, fallback, token-budget, TTL, and promotion-policy invariants.
func validateOMPContextProfile(name string, raw OMPContextProfileConf) error {
	if raw.MemoryMode == "active" {
		return contextProfileError(name, "memory_active_forbidden", raw.MemoryMode)
	}
	effective := effectiveOMPContextProfile(raw)
	if effective.HistoryMode != OMPContextHistoryOff && effective.HistoryMode != OMPContextHistoryShadow && effective.HistoryMode != OMPContextHistoryActive {
		return contextProfileError(name, "history_mode_invalid", raw.HistoryMode)
	}
	if effective.MemoryMode != OMPContextMemoryOff && effective.MemoryMode != OMPContextMemoryShadow {
		return contextProfileError(name, "memory_mode_invalid", raw.MemoryMode)
	}
	if raw.HistoryMode == OMPContextHistoryActive {
		if raw.Fallback == "" {
			return contextProfileError(name, "active_fallback_required", "")
		}
		if raw.CapabilityPolicy == "" {
			return contextProfileError(name, "active_capability_policy_required", "")
		}
		if raw.RuntimeRootPolicy == "" {
			return contextProfileError(name, "active_runtime_root_required", "")
		}
	}
	if effective.HistoryTargetTokens < MinOMPContextHistoryTargetTokens || effective.HistoryTargetTokens > MaxOMPContextHistoryTargetTokens {
		return contextProfileError(name, "history_target_out_of_bounds", raw.HistoryTargetTokens)
	}
	if effective.MemoryTTLSeconds < MinOMPContextMemoryTTLSeconds || effective.MemoryTTLSeconds > MaxOMPContextMemoryTTLSeconds {
		return contextProfileError(name, "memory_ttl_out_of_bounds", raw.MemoryTTLSeconds)
	}
	if raw.MemoryNamespace != "" && !validOMPContextNamespace(raw.MemoryNamespace) {
		return contextProfileError(name, "memory_namespace_invalid", raw.MemoryNamespace)
	}
	if effective.MemoryMode == OMPContextMemoryShadow && raw.MemoryNamespace == "" {
		return contextProfileError(name, "memory_namespace_required", "")
	}
	if effective.Fallback != OMPContextFallbackBlock && effective.Fallback != OMPContextFallbackCanonicalFull {
		return contextProfileError(name, "fallback_invalid", raw.Fallback)
	}
	if effective.CapabilityPolicy != OMPContextCapabilityProbeRequired {
		return contextProfileError(name, "capability_policy_invalid", raw.CapabilityPolicy)
	}
	if err := validateOMPContextOwnership(name, effective); err != nil {
		return err
	}
	return nil
}

func validateOMPContextOwnership(name string, profile OMPContextProfileConf) error {
	for _, value := range []string{profile.RuntimeRootPolicy, profile.MutationScope} {
		switch value {
		case "user", "user_owned", "user_root":
			return contextProfileError(name, "user_root_claim_forbidden", value)
		case "global", "user_global", "project_global":
			return contextProfileError(name, "global_mutation_claim_forbidden", value)
		}
	}
	if profile.RuntimeRootPolicy != OMPContextRuntimeNoSession && profile.RuntimeRootPolicy != OMPContextRuntimeIsolatedTaskOwned {
		return contextProfileError(name, "runtime_root_policy_invalid", profile.RuntimeRootPolicy)
	}
	if profile.MutationScope != OMPContextMutationSessionOverlay {
		return contextProfileError(name, "mutation_scope_invalid", profile.MutationScope)
	}
	return nil
}

func validOMPContextNamespace(namespace string) bool {
	if len(namespace) == 0 || len(namespace) > MaxOMPContextMemoryNamespaceLength {
		return false
	}
	for index, char := range namespace {
		alphanumeric := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if alphanumeric || index > 0 && strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return true
}

func contextProfileError(profile, code string, value any) error {
	return fmt.Errorf("omp_context_policy.profiles[%s].%s: %v", profile, code, value)
}
