package config

import (
	"strings"
	"testing"
)

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
		code   string
	}{
		{
			name: "optional route",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.Required = false
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
			code: "operator_attestation_route_not_closed: coding_tool_use",
		},
		{
			name: "runtime default degradation",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.DegradedAction = "runtime_default"
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
			code: "operator_attestation_route_not_closed: coding_tool_use",
		},
		{
			name: "empty family",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityCodingToolUse]
				route.Candidates[0].Family = ""
				profile.Capabilities[CapabilityCodingToolUse] = route
			},
			code: "capabilities[coding_tool_use].candidates[0].operator_attestation_family_required",
		},
		{
			name: "conflicting family for one selector",
			mutate: func(profile *RoleModelProfileConf) {
				route := profile.Capabilities[CapabilityVisionDesign]
				route.Candidates[0].Family = "other-family"
				profile.Capabilities[CapabilityVisionDesign] = route
			},
			code: "capabilities[vision_design].operator_attestation_family_conflict: acme/model",
		},
		{
			name: "override candidate without family",
			mutate: func(profile *RoleModelProfileConf) {
				profile.Agents["executor"] = RoleAgentOverrideConf{
					Candidates: []RoleModelCandidateConf{{Selector: "acme/coder", Thinking: "high"}},
				}
			},
			code: "agents[executor].candidates[0].operator_attestation_family_required",
		},
		{
			name: "override family conflicts with capability declaration",
			mutate: func(profile *RoleModelProfileConf) {
				profile.Agents["executor"] = RoleAgentOverrideConf{
					Candidates: []RoleModelCandidateConf{{Selector: "acme/model", Thinking: "high", Family: "other-family"}},
				}
			},
			code: "agents[executor].operator_attestation_family_conflict: acme/model",
		},
		{
			name: "override duplicate declaration",
			mutate: func(profile *RoleModelProfileConf) {
				profile.Agents["executor"] = RoleAgentOverrideConf{
					Candidates: []RoleModelCandidateConf{
						{Selector: "acme/coder", Thinking: "high", Family: "acme"},
						{Selector: "acme/coder", Thinking: "high", Family: "acme"},
					},
				}
			},
			code: "agents[executor].operator_attestation_declaration_duplicate: acme/coder",
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
			err := policy.Validate()
			if err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("Validate() error = %v, want %q", err, test.code)
			}
		})
	}
}

func TestRoleModelProfileCatalogTrust_OperatorAttestedAcceptsCompleteDeclarations(t *testing.T) {
	t.Parallel()

	policy := validRoleModelPolicyFixture()
	profile := policy.Profiles[policy.Profile]
	profile.CatalogTrust = RoleModelCatalogTrustOperatorAttested
	profile.Agents["executor"] = RoleAgentOverrideConf{
		Candidates: []RoleModelCandidateConf{
			{Selector: "acme/coder", Thinking: "high", Family: "acme"},
			{Selector: "acme/model", Thinking: "medium", Family: "acme"},
		},
	}
	policy.Profiles[policy.Profile] = profile
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() rejected a complete operator-attested profile: %v", err)
	}
}
