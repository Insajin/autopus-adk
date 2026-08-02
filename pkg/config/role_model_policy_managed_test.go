package config

import (
	"strings"
	"testing"
)

const missingManagedFingerprintFixture = "sha256:47a5f5efebd01b44f543a37adf34b22a3591c72fe46098069bc8a307cf490b04"

func TestRoleModelPolicy_ProjectManaged_RequiresExplicitCompleteClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims map[string]RoleManagedKeyClaimConf
		code   string
	}{
		{"missing", nil, "managed_key_claim_required"},
		{"unknown", map[string]RoleManagedKeyClaimConf{"unknown.key": validManagedClaim(false)}, "managed_key_claim_invalid"},
		{"incomplete", map[string]RoleManagedKeyClaimConf{"modelRoles": {PriorFingerprint: missingManagedFingerprintFixture}}, "managed_key_claim_invalid"},
		{"invalid fingerprint", map[string]RoleManagedKeyClaimConf{"modelRoles": {PriorFingerprint: "sha256:bad", Complete: true}}, "managed_key_claim_invalid"},
		{"array incomplete", map[string]RoleManagedKeyClaimConf{"retry.fallbackChains": validManagedClaim(false)}, "array_ownership_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validRoleModelPolicyFixture()
			profile := policy.Profiles["p1"]
			profile.ConfigMode = RoleModelConfigModeProjectManaged
			profile.ManagedKeys = tt.claims
			policy.Profiles["p1"] = profile
			if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.code)
			}
		})
	}
}

func TestRoleModelPolicy_ProjectManaged_AcceptsExactClaims(t *testing.T) {
	t.Parallel()

	policy := validRoleModelPolicyFixture()
	profile := policy.Profiles["p1"]
	profile.ConfigMode = RoleModelConfigModeProjectManaged
	profile.ManagedKeys = map[string]RoleManagedKeyClaimConf{
		"modelRoles":           validManagedClaim(false),
		"retry.fallbackChains": validManagedClaim(true),
		"retry.modelFallback":  validManagedClaim(false),
	}
	policy.Profiles["p1"] = profile
	if err := policy.Validate(); err != nil {
		t.Fatalf("valid managed claims rejected: %v", err)
	}
}

func validManagedClaim(array bool) RoleManagedKeyClaimConf {
	return RoleManagedKeyClaimConf{
		PriorFingerprint: missingManagedFingerprintFixture,
		Complete:         true, FullArrayOwnership: array,
	}
}
