package config

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func (c *HarnessConfig) validateModelSelectionAndRolePolicy() error {
	if err := c.validateModelSelectionConfig(); err != nil {
		return err
	}
	return c.RoleModelPolicy.validateForQuality(c.Quality)
}

// Validate rejects partial or ambiguous opt-in role-model policies.
func (c RoleModelPolicyConf) Validate() error {
	return c.validateForQuality(QualityConf{})
}

func (c RoleModelPolicyConf) validateForQuality(quality QualityConf) error {
	if c.Version == "" && c.Profile == "" && c.Family == "" &&
		c.ConfigMode == "" && len(c.Profiles) == 0 {
		return nil
	}
	if c.EffectiveVersion() != RoleModelPolicyVersionV1 {
		return fmt.Errorf("role_model_policy.policy_version_unknown: %q", c.Version)
	}
	if _, ok := effectiveBuiltinRoleModelFamily(c.Family); !ok {
		return fmt.Errorf("role_model_policy.family_invalid: %q", c.Family)
	}
	if _, ok := effectiveBuiltinRoleModelConfigMode(c.ConfigMode); !ok {
		return fmt.Errorf("role_model_policy.config_mode_invalid: %q", c.ConfigMode)
	}
	if c.Profile != "" {
		if _, ok := c.Profiles[c.Profile]; !ok {
			if !IsBuiltinRoleModelProfileName(c.Profile) {
				return fmt.Errorf("role_model_policy.profile_unknown: %q", c.Profile)
			}
			profile, derived := BuiltinRoleModelProfile(c.Profile, quality, c.Family, c.ConfigMode)
			if !derived {
				return fmt.Errorf("role_model_policy.profile_unknown: %q", c.Profile)
			}
			if err := validateRoleModelProfile(c.Profile, profile); err != nil {
				return err
			}
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

// @AX:WARN [AUTO]: role-model profile validation contains 11 if branches.
// @AX:REASON [AUTO]: version, capability, agent override, trust, diversity, and managed-key constraints must all agree.
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
	for _, agent := range sortedAgentOverrides(profile) {
		if err := validateAgentOverride(name, agent, profile.Agents[agent]); err != nil {
			return err
		}
	}
	if trust == RoleModelCatalogTrustOperatorAttested {
		if err := validateOperatorAttestedRoutes(name, profile); err != nil {
			return err
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

// sortedAgentOverrides orders override validation so error reports are stable.
func sortedAgentOverrides(profile RoleModelProfileConf) []string {
	agents := make([]string, 0, len(profile.Agents))
	for agent := range profile.Agents {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	return agents
}

// validateAgentOverride requires a matrix agent, rejects role or capability
// assertions that disagree with the matrix, and applies the capability
// candidate rules to override candidates.
func validateAgentOverride(profile, agent string, override RoleAgentOverrideConf) error {
	capability, err := OMPAgentCapability(agent)
	if err != nil {
		return fmt.Errorf("role_model_policy.profiles[%s].%w", profile, err)
	}
	scope := fmt.Sprintf("role_model_policy.profiles[%s].agents[%s]", profile, agent)
	if role := OMPAgentRoleName(agent); override.Role != "" && override.Role != role {
		return fmt.Errorf("%s.role_capability_mismatch: role %q, want %q", scope, override.Role, role)
	}
	if override.Capability != "" && override.Capability != capability {
		return fmt.Errorf("%s.role_capability_mismatch: capability %q, want %q", scope, override.Capability, capability)
	}
	return validateRouteCandidates(scope, override.Candidates)
}

// validateOperatorAttestedRoutes requires closed capability routes and applies
// the attestation candidate rules to capability and agent override candidates
// alike, sharing one selector-to-family ledger across all of them.
func validateOperatorAttestedRoutes(name string, profile RoleModelProfileConf) error {
	families := make(map[string]string)
	for _, capability := range providerNeutralCapabilities {
		route := profile.Capabilities[capability]
		if !route.Required || route.DegradedAction != "" {
			return fmt.Errorf("role_model_policy.profiles[%s].operator_attestation_route_not_closed: %s", name, capability)
		}
		scope := fmt.Sprintf("role_model_policy.profiles[%s].capabilities[%s]", name, capability)
		if err := validateOperatorAttestedCandidates(scope, route.Candidates, families); err != nil {
			return err
		}
	}
	for _, agent := range sortedAgentOverrides(profile) {
		scope := fmt.Sprintf("role_model_policy.profiles[%s].agents[%s]", name, agent)
		if err := validateOperatorAttestedCandidates(scope, profile.Agents[agent].Candidates, families); err != nil {
			return err
		}
	}
	return nil
}

func validateOperatorAttestedCandidates(
	scope string,
	candidates []RoleModelCandidateConf,
	families map[string]string,
) error {
	seen := make(map[string]struct{}, len(candidates))
	for index, candidate := range candidates {
		if candidate.Family == "" {
			return fmt.Errorf("%s.candidates[%d].operator_attestation_family_required", scope, index)
		}
		if family, ok := families[candidate.Selector]; ok && family != candidate.Family {
			return fmt.Errorf("%s.operator_attestation_family_conflict: %s", scope, candidate.Selector)
		}
		families[candidate.Selector] = candidate.Family
		declaration := candidate.Selector + "\x00" + candidate.Thinking
		if _, duplicate := seen[declaration]; duplicate {
			return fmt.Errorf("%s.operator_attestation_declaration_duplicate: %s", scope, candidate.Selector)
		}
		seen[declaration] = struct{}{}
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
	scope := fmt.Sprintf("role_model_policy.profiles[%s].capabilities[%s]", profile, capability)
	if route.DegradedAction != "" && route.DegradedAction != "runtime_default" {
		return fmt.Errorf("%s.degraded_action_invalid: %q", scope, route.DegradedAction)
	}
	if len(route.Candidates) == 0 {
		return fmt.Errorf("%s.candidates_required", scope)
	}
	return validateRouteCandidates(scope, route.Candidates)
}

// validateRouteCandidates applies the selector, thinking, and family rules
// shared by capability routes and agent overrides.
func validateRouteCandidates(scope string, candidates []RoleModelCandidateConf) error {
	for index, candidate := range candidates {
		if !validSelector(candidate.Selector) {
			return fmt.Errorf("%s.candidates[%d].selector_invalid: %q", scope, index, candidate.Selector)
		}
		if !IsOMPNativeThinkingLevel(candidate.Thinking) || !validPolicyToken(candidate.Family) {
			return fmt.Errorf("%s.candidates[%d].metadata_invalid", scope, index)
		}
	}
	return nil
}

// validateFamilyDiversity accepts only Policy Contract agent roles.
func validateFamilyDiversity(profile string, policy FamilyDiversityPolicyConf) error {
	seen := make(map[string]struct{}, len(policy.Roles))
	for _, role := range policy.Roles {
		if _, err := OMPRoleCapability(role); err != nil {
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
