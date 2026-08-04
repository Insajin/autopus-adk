package config

const (
	RoleModelPolicyVersionV1 = "v1"

	RoleModelConfigModeOverlay        = "overlay"
	RoleModelConfigModeProjectManaged = "project-managed"
)

// RoleModelPolicyConf is an opt-in, provider-neutral OMP role routing policy.
type RoleModelPolicyConf struct {
	Version  string                          `yaml:"version,omitempty"`
	Profile  string                          `yaml:"profile,omitempty"`
	Profiles map[string]RoleModelProfileConf `yaml:"profiles,omitempty"`
}

// RoleModelProfileConf owns capability routes and optional OMP projection policy.
type RoleModelProfileConf struct {
	ConfigMode      string                             `yaml:"config_mode,omitempty"`
	Capabilities    map[string]RoleCapabilityRouteConf `yaml:"capabilities,omitempty"`
	Agents          map[string]RoleAgentOverrideConf   `yaml:"agents,omitempty"`
	ManagedKeys     map[string]RoleManagedKeyClaimConf `yaml:"managed_keys,omitempty"`
	FamilyDiversity FamilyDiversityPolicyConf          `yaml:"family_diversity,omitempty"`
	Safety          RoleSafetyPolicyConf               `yaml:"safety,omitempty"`
}

// RoleManagedKeyClaimConf proves complete ownership of one project config key.
type RoleManagedKeyClaimConf struct {
	PriorFingerprint   string `yaml:"prior_fingerprint"`
	Complete           bool   `yaml:"complete"`
	FullArrayOwnership bool   `yaml:"full_array_ownership,omitempty"`
}

// RoleCapabilityRouteConf declares ordered candidates for one semantic capability.
type RoleCapabilityRouteConf struct {
	Candidates     []RoleModelCandidateConf `yaml:"candidates,omitempty"`
	Required       bool                     `yaml:"required,omitempty"`
	DegradedAction string                   `yaml:"degraded_action,omitempty"`
}

// RoleModelCandidateConf contains only non-secret selector metadata.
type RoleModelCandidateConf struct {
	Selector string `yaml:"selector"`
	Thinking string `yaml:"thinking,omitempty"`
	Family   string `yaml:"family,omitempty"`
}

// RoleAgentOverrideConf pins an agent to a valid native role/capability pair.
type RoleAgentOverrideConf struct {
	Role       string `yaml:"role"`
	Capability string `yaml:"capability"`
}

// FamilyDiversityPolicyConf selects roles that prefer a distinct model family.
type FamilyDiversityPolicyConf struct {
	Enabled bool     `yaml:"enabled,omitempty"`
	Roles   []string `yaml:"roles,omitempty"`
}

// RoleSafetyPolicyConf claims OMP safety keys only when a value is explicit.
type RoleSafetyPolicyConf struct {
	ApprovalMode  string `yaml:"approval_mode,omitempty"`
	IsolationMode string `yaml:"isolation_mode,omitempty"`
}

// LegacyRoleRoute is a versioned semantic translation of a legacy quality tier.
type LegacyRoleRoute struct {
	Capability   string
	Role         string
	LegacySource string
}

// EffectiveVersion returns the backward-compatible v1 policy version.
func (c RoleModelPolicyConf) EffectiveVersion() string {
	if c.Version == "" {
		return RoleModelPolicyVersionV1
	}
	return c.Version
}

// SelectedRoleModelProfile returns the explicitly selected profile.
func (c RoleModelPolicyConf) SelectedRoleModelProfile() (string, RoleModelProfileConf, bool) {
	if c.Profile == "" {
		return "", RoleModelProfileConf{}, false
	}
	profile, ok := c.Profiles[c.Profile]
	return c.Profile, profile, ok
}
