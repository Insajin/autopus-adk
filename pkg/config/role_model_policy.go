package config

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	RoleModelPolicyVersionV1 = "v1"

	RoleModelConfigModeOverlay        = "overlay"
	RoleModelConfigModeProjectManaged = "project-managed"

	RoleModelCatalogTrustStrict           = "strict"
	RoleModelCatalogTrustOperatorAttested = "operator-attested"
)

// RoleModelPolicyConf is an opt-in, provider-neutral OMP role routing policy.
// Family and ConfigMode apply only when a selected built-in profile is derived.
type RoleModelPolicyConf struct {
	Version    string                          `yaml:"version,omitempty"`
	Profile    string                          `yaml:"profile,omitempty"`
	Family     string                          `yaml:"family,omitempty"`
	ConfigMode string                          `yaml:"config_mode,omitempty"`
	Profiles   map[string]RoleModelProfileConf `yaml:"profiles,omitempty"`
}

// RoleModelProfileConf owns capability routes and optional OMP projection policy.
type RoleModelProfileConf struct {
	ConfigMode      string                             `yaml:"config_mode,omitempty"`
	CatalogTrust    string                             `yaml:"catalog_trust,omitempty"`
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

// EffectiveCatalogTrust returns the strict default for catalog normalization.
func (c RoleModelProfileConf) EffectiveCatalogTrust() string {
	if c.CatalogTrust == "" {
		return RoleModelCatalogTrustStrict
	}
	return c.CatalogTrust
}

// OMPMissingManagedValueFingerprint identifies a managed key that was absent.
func OMPMissingManagedValueFingerprint() string {
	sum := sha256.Sum256([]byte("autopus.omp.managed.missing.v1"))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SelectedRoleModelProfile returns the explicitly selected profile.
func (c RoleModelPolicyConf) SelectedRoleModelProfile() (string, RoleModelProfileConf, bool) {
	if c.Profile == "" {
		return "", RoleModelProfileConf{}, false
	}
	profile, ok := c.Profiles[c.Profile]
	return c.Profile, profile, ok
}

// SelectedRoleModelProfileForQuality resolves the selected profile, falling
// back to the quality-derived built-in profile of the same name when the
// config defines none. An explicit definition always wins.
func (c RoleModelPolicyConf) SelectedRoleModelProfileForQuality(quality QualityConf) (string, RoleModelProfileConf, bool) {
	name, profile, ok := c.SelectedRoleModelProfile()
	if ok || name == "" {
		return name, profile, ok
	}
	profile, ok = BuiltinRoleModelProfile(name, quality, c.Family, c.ConfigMode)
	return name, profile, ok
}
