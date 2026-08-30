package config

import "testing"

func TestRoleModelProfileCatalogTrust_DefaultsToStrictAndValidatesKnownValues(t *testing.T) {
	t.Parallel()

	policy := validRoleModelPolicyFixture()
	profile := policy.Profiles[policy.Profile]
	if got := profile.EffectiveCatalogTrust(); got != RoleModelCatalogTrustStrict {
		t.Fatalf("EffectiveCatalogTrust() = %q, want %q", got, RoleModelCatalogTrustStrict)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() with omitted catalog trust: %v", err)
	}

	profile.CatalogTrust = RoleModelCatalogTrustStrict
	policy.Profiles[policy.Profile] = profile
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() with explicit strict trust: %v", err)
	}

	profile.CatalogTrust = "future-trust"
	policy.Profiles[policy.Profile] = profile
	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() accepted an unknown catalog trust")
	}
}

func TestRoleModelProfileCatalogTrust_OperatorAttestedRequiresClosedRoutesAndConsistentFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*RoleModelProfileConf)
	}{
		{
			name: "optional route",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.Required = false
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
		},
		{
			name: "runtime default degradation",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.DegradedAction = "runtime_default"
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
		},
		{
			name: "empty family",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.Candidates[0].Family = ""
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
		},
		{
			name: "conflicting family for one selector",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityVisionDesign]
				route.Candidates[0].Family = "other-family"
				profile.Capabilities[CapabilityVisionDesign] = route
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy := validRoleModelPolicyFixture()
			profile := policy.Profiles[policy.Profile]
			profile.CatalogTrust = RoleModelCatalogTrustOperatorAttested
			test.mutate(&profile)
			policy.Profiles[policy.Profile] = profile
			if err := policy.Validate(); err == nil {
				t.Fatal("Validate() accepted an invalid operator-attested profile")
			}
		})
	}
}

func TestRoleModelProfileCatalogTrust_OperatorAttestedAcceptsCompleteDeclarations(t *testing.T) {
	t.Parallel()

	policy := validRoleModelPolicyFixture()
	profile := policy.Profiles[policy.Profile]
	profile.CatalogTrust = RoleModelCatalogTrustOperatorAttested
	policy.Profiles[policy.Profile] = profile
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() rejected a complete operator-attested profile: %v", err)
	}
}
