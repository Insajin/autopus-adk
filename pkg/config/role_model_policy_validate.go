package config

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func (c *HarnessConfig) validateModelSelectionAndRolePolicy() error {
	if err := c.validateModelSelectionConfig(); err != nil {
		return err
	}
	return c.RoleModelPolicy.Validate()
}

// Validate rejects partial or ambiguous opt-in role-model policies.
func (c RoleModelPolicyConf) Validate() error {
	if c.Version == "" && c.Profile == "" && len(c.Profiles) == 0 {
		return nil
	}
	if c.EffectiveVersion() != RoleModelPolicyVersionV1 {
		return fmt.Errorf("role_model_policy.policy_version_unknown: %q", c.Version)
	}
	if c.Profile != "" {
		// A built-in name needs no explicit definition: it is derived from the
		// quality presets at lookup time.
		if _, ok := c.Profiles[c.Profile]; !ok && !IsBuiltinRoleModelProfileName(c.Profile) {
			return fmt.Errorf("role_model_policy.profile_unknown: %q", c.Profile)
		}
	}
	for name, profile := range c.Profiles {
		if !IsValidQualityPresetName(name) {
			return fmt.Errorf("role_model_policy.profile_name_invalid: %q", name)
		}
		if err := validateRoleModelProfile(name, profile); err != nil {
			return err
		}
	}
	return nil
}

// @AX:WARN [AUTO]: role-model profile validation contains 9 if branches.
// @AX:REASON [AUTO]: version, capability, source, alias, fallback, and per-role constraints must all agree.
func validateRoleModelProfile(name string, profile RoleModelProfileConf) error {
	if profile.ConfigMode != RoleModelConfigModeOverlay && profile.ConfigMode != RoleModelConfigModeProjectManaged {
		return fmt.Errorf("role_model_policy.profiles[%s].config_mode_invalid: %q", name, profile.ConfigMode)
	}
	trust := profile.EffectiveCatalogTrust()
	if trust != RoleModelCatalogTrustStrict && trust != RoleModelCatalogTrustOperatorAttested {
		return fmt.Errorf("role_model_policy.profiles[%s].catalog_trust_invalid: %q", name, profile.CatalogTrust)
	}
	for _, capability := range providerNeutralCapabilities {
		if _, ok := profile.Capabilities[capability]; !ok {
			return fmt.Errorf("role_model_policy.profiles[%s].capability_missing: %q", name, capability)
		}
	}
	for capability, route := range profile.Capabilities {
		if _, err := CanonicalOMPRoleForCapability(capability); err != nil {
			return fmt.Errorf("role_model_policy.profiles[%s].%w", name, err)
		}
		if err := validateCapabilityRoute(name, capability, route); err != nil {
			return err
		}
	}
	if trust == RoleModelCatalogTrustOperatorAttested {
		if err := validateOperatorAttestedRoutes(name, profile); err != nil {
			return err
		}
	}
	for agent, override := range profile.Agents {
		if _, err := OMPAgentRole(agent); err != nil {
			return fmt.Errorf("role_model_policy.profiles[%s].%w", name, err)
		}
		if err := ValidateRoleCapabilityPair(override.Role, override.Capability); err != nil {
			return fmt.Errorf("role_model_policy.profiles[%s].agents[%s].%w", name, agent, err)
		}
	}
	if err := validateFamilyDiversity(name, profile.FamilyDiversity); err != nil {
		return err
	}
	if err := validateSafety(name, profile.Safety); err != nil {
		return err
	}
	if err := validateRoleManagedKeys(name, profile); err != nil {
		return err
	}
	return nil
}

func validateOperatorAttestedRoutes(name string, profile RoleModelProfileConf) error {
	families := make(map[string]string)
	for _, capability := range providerNeutralCapabilities {
		route := profile.Capabilities[capability]
		if !route.Required || route.DegradedAction != "" {
			return fmt.Errorf("role_model_policy.profiles[%s].operator_attestation_route_not_closed: %s", name, capability)
		}
		seen := make(map[string]struct{}, len(route.Candidates))
		for index, candidate := range route.Candidates {
			if candidate.Family == "" {
				return fmt.Errorf(
					"role_model_policy.profiles[%s].capabilities[%s].candidates[%d].operator_attestation_family_required",
					name, capability, index,
				)
			}
			if family, ok := families[candidate.Selector]; ok && family != candidate.Family {
				return fmt.Errorf(
					"role_model_policy.profiles[%s].operator_attestation_family_conflict: %s",
					name, candidate.Selector,
				)
			}
			families[candidate.Selector] = candidate.Family
			declaration := candidate.Selector + "\x00" + candidate.Thinking
			if _, duplicate := seen[declaration]; duplicate {
				return fmt.Errorf(
					"role_model_policy.profiles[%s].capabilities[%s].operator_attestation_declaration_duplicate: %s",
					name, capability, candidate.Selector,
				)
			}
			seen[declaration] = struct{}{}
		}
	}
	return nil
}

func validateRoleManagedKeys(name string, profile RoleModelProfileConf) error {
	if profile.ConfigMode == RoleModelConfigModeProjectManaged && len(profile.ManagedKeys) == 0 {
		return fmt.Errorf("role_model_policy.profiles[%s].managed_key_claim_required", name)
	}
	allowed := map[string]bool{
		"modelRoles": true, "retry.fallbackChains": true, "retry.modelFallback": true,
		"tools.approvalMode": true, "task.isolation.mode": true,
	}
	for path, claim := range profile.ManagedKeys {
		if !allowed[path] || !claim.Complete || !validRoleManagedFingerprint(claim.PriorFingerprint) {
			return fmt.Errorf("role_model_policy.profiles[%s].managed_key_claim_invalid: %s", name, path)
		}
		if path == "retry.fallbackChains" && !claim.FullArrayOwnership {
			return fmt.Errorf("role_model_policy.profiles[%s].array_ownership_required: %s", name, path)
		}
	}
	return nil
}

func validRoleManagedFingerprint(value string) bool {
	const prefix = "sha256:"
	if len(value) != len(prefix)+64 || !strings.HasPrefix(value, prefix) {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}

func validateCapabilityRoute(profile, capability string, route RoleCapabilityRouteConf) error {
	if route.DegradedAction != "" && route.DegradedAction != "runtime_default" {
		return fmt.Errorf("role_model_policy.profiles[%s].capabilities[%s].degraded_action_invalid: %q", profile, capability, route.DegradedAction)
	}
	if len(route.Candidates) == 0 {
		return fmt.Errorf("role_model_policy.profiles[%s].capabilities[%s].candidates_required", profile, capability)
	}
	for index, candidate := range route.Candidates {
		if !validSelector(candidate.Selector) {
			return fmt.Errorf("role_model_policy.profiles[%s].capabilities[%s].candidates[%d].selector_invalid: %q", profile, capability, index, candidate.Selector)
		}
		if !IsOMPNativeThinkingLevel(candidate.Thinking) || !validPolicyToken(candidate.Family) {
			return fmt.Errorf("role_model_policy.profiles[%s].capabilities[%s].candidates[%d].metadata_invalid", profile, capability, index)
		}
	}
	return nil
}

func validateFamilyDiversity(profile string, policy FamilyDiversityPolicyConf) error {
	seen := make(map[string]struct{}, len(policy.Roles))
	for _, role := range policy.Roles {
		if _, err := OMPNativeRoleCapability(role); err != nil {
			return fmt.Errorf("role_model_policy.profiles[%s].family_diversity.%w", profile, err)
		}
		if _, duplicate := seen[role]; duplicate {
			return fmt.Errorf("role_model_policy.profiles[%s].family_diversity.role_duplicate: %q", profile, role)
		}
		seen[role] = struct{}{}
	}
	return nil
}

func validateSafety(profile string, safety RoleSafetyPolicyConf) error {
	if !validPolicyToken(safety.ApprovalMode) {
		return fmt.Errorf("role_model_policy.profiles[%s].safety.approval_mode_invalid", profile)
	}
	if !validPolicyToken(safety.IsolationMode) {
		return fmt.Errorf("role_model_policy.profiles[%s].safety.isolation_mode_invalid", profile)
	}
	return nil
}

func validSelector(selector string) bool {
	parts := strings.Split(selector, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" &&
		validPolicyToken(parts[0]) && validPolicyToken(parts[1])
}

func validPolicyToken(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return true
}
