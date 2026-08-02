package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRoleModelPolicy_ZeroValue_OmitsYAMLAndPreservesValidation(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("zero-policy")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal default config: %v", err)
	}
	if strings.Contains(string(data), "role_model_policy:") {
		t.Fatalf("zero-value policy changed default YAML:\n%s", data)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("zero-value policy invalidated default config: %v", err)
	}
}

func TestRoleModelPolicy_SelectedProfile_RoundTrips(t *testing.T) {
	t.Parallel()

	want := validRoleModelPolicyFixture()
	cfg := DefaultFullConfig("routing")
	cfg.RoleModelPolicy = want
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var got HarnessConfig
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if !reflect.DeepEqual(got.RoleModelPolicy, want) {
		t.Fatalf("policy round-trip mismatch\ngot:  %#v\nwant: %#v", got.RoleModelPolicy, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("round-tripped config invalid: %v", err)
	}
	name, profile, ok := got.RoleModelPolicy.SelectedRoleModelProfile()
	if !ok || name != "p1" || profile.ConfigMode != RoleModelConfigModeOverlay {
		t.Fatalf("selected profile = %q %#v %v", name, profile, ok)
	}
}

func TestRoleModelPolicy_ProjectionMatrix_IsExact(t *testing.T) {
	t.Parallel()

	wantCapabilities := map[string]string{
		CapabilityDeepReasoning:          OMPRolePlan,
		CapabilityCodingToolUse:          OMPRoleTask,
		CapabilityFastValidation:         OMPRoleSmol,
		CapabilityVisionDesign:           OMPRoleVision,
		CapabilityIndependentDissent:     OMPRoleAdvisor,
		CapabilityDeterministicTransform: OMPRoleTiny,
	}
	for capability, wantRole := range wantCapabilities {
		gotRole, err := CanonicalOMPRoleForCapability(capability)
		if err != nil || gotRole != wantRole {
			t.Errorf("canonical role for %q = %q, %v; want %q", capability, gotRole, err, wantRole)
		}
	}

	wantAgents := map[string]string{
		"annotator": OMPRoleSmol, "explorer": OMPRoleSmol,
		"architect": OMPRoleSlow,
		"planner":   OMPRolePlan, "spec-writer": OMPRolePlan,
		"ux-validator":        OMPRoleVision,
		"frontend-specialist": OMPRoleDesigner,
		"validator":           OMPRoleTiny,
		"debugger":            OMPRoleTask, "deep-worker": OMPRoleTask, "devops": OMPRoleTask,
		"executor": OMPRoleTask, "perf-engineer": OMPRoleTask, "tester": OMPRoleTask,
		"reviewer": OMPRoleAdvisor, "security-auditor": OMPRoleAdvisor,
	}
	if got := OMPAgentRoleMapping(); !reflect.DeepEqual(got, wantAgents) {
		t.Fatalf("agent role mapping mismatch\ngot:  %#v\nwant: %#v", got, wantAgents)
	}
	if err := ValidateOMPAgentRoleSet(mapKeys(wantAgents)); err != nil {
		t.Fatalf("exact agent set rejected: %v", err)
	}
	if err := ValidateOMPAgentRoleSet(append(mapKeys(wantAgents), "future-agent")); err == nil || !strings.Contains(err.Error(), "agent_role_unmapped") {
		t.Fatalf("unmapped agent error = %v", err)
	}
	if err := ValidateRoleCapabilityPair(OMPRoleAdvisor, CapabilityCodingToolUse); err == nil || !strings.Contains(err.Error(), "role_capability_mismatch") {
		t.Fatalf("mismatched role/capability error = %v", err)
	}
}

func TestLegacyTierRoute_V1_FailsClosedForUnknownTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tier       string
		capability string
		role       string
	}{
		{"opus", CapabilityDeepReasoning, OMPRolePlan},
		{"sonnet", CapabilityCodingToolUse, OMPRoleTask},
		{"haiku", CapabilityDeterministicTransform, OMPRoleTiny},
	}
	for _, tt := range tests {
		got, err := LegacyTierRoute(RoleModelPolicyVersionV1, tt.tier)
		if err != nil {
			t.Fatalf("route %q: %v", tt.tier, err)
		}
		if got.Capability != tt.capability || got.Role != tt.role || got.LegacySource != tt.tier {
			t.Errorf("route %q = %#v", tt.tier, got)
		}
	}
	if _, err := LegacyTierRoute(RoleModelPolicyVersionV1, "turbo"); err == nil || !strings.Contains(err.Error(), "legacy_tier_unknown") {
		t.Fatalf("unknown legacy tier error = %v", err)
	}
	if _, err := LegacyTierRoute("v2", "opus"); err == nil || !strings.Contains(err.Error(), "policy_version_unknown") {
		t.Fatalf("unknown policy version error = %v", err)
	}
}

func TestRoleModelPolicy_Validation_FailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*HarnessConfig)
		code   string
	}{
		{"missing profile", func(c *HarnessConfig) { c.RoleModelPolicy.Profile = "missing" }, "profile_unknown"},
		{"unknown capability", func(c *HarnessConfig) {
			p := c.RoleModelPolicy.Profiles["p1"]
			p.Capabilities["vendor_magic"] = RoleCapabilityRouteConf{Candidates: []RoleModelCandidateConf{{Selector: "acme/model"}}}
			c.RoleModelPolicy.Profiles["p1"] = p
		}, "capability_unknown"},
		{"empty candidate", func(c *HarnessConfig) {
			p := c.RoleModelPolicy.Profiles["p1"]
			p.Capabilities[CapabilityDeepReasoning] = RoleCapabilityRouteConf{Candidates: []RoleModelCandidateConf{{}}}
			c.RoleModelPolicy.Profiles["p1"] = p
		}, "selector_invalid"},
		{"unknown agent", func(c *HarnessConfig) {
			p := c.RoleModelPolicy.Profiles["p1"]
			p.Agents["future-agent"] = RoleAgentOverrideConf{Role: OMPRoleTask, Capability: CapabilityCodingToolUse}
			c.RoleModelPolicy.Profiles["p1"] = p
		}, "agent_role_unmapped"},
		{"mismatched override", func(c *HarnessConfig) {
			p := c.RoleModelPolicy.Profiles["p1"]
			p.Agents["reviewer"] = RoleAgentOverrideConf{Role: OMPRoleAdvisor, Capability: CapabilityCodingToolUse}
			c.RoleModelPolicy.Profiles["p1"] = p
		}, "role_capability_mismatch"},
		{"omp quality provider", func(c *HarnessConfig) {
			c.Quality.Providers = map[string]string{"omp": "balanced"}
		}, "quality.providers provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultFullConfig("invalid-policy")
			cfg.RoleModelPolicy = validRoleModelPolicyFixture()
			tt.mutate(cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("Validate() error = %v, want code %q", err, tt.code)
			}
		})
	}
}

func validRoleModelPolicyFixture() RoleModelPolicyConf {
	routes := make(map[string]RoleCapabilityRouteConf, len(OMPProviderNeutralCapabilities()))
	for _, capability := range OMPProviderNeutralCapabilities() {
		routes[capability] = RoleCapabilityRouteConf{
			Required:   true,
			Candidates: []RoleModelCandidateConf{{Selector: "acme/model", Thinking: "high", Family: "acme"}},
		}
	}
	return RoleModelPolicyConf{
		Version: RoleModelPolicyVersionV1,
		Profile: "p1",
		Profiles: map[string]RoleModelProfileConf{
			"p1": {
				ConfigMode:   RoleModelConfigModeOverlay,
				Capabilities: routes,
				Agents: map[string]RoleAgentOverrideConf{
					"reviewer": {Role: OMPRoleAdvisor, Capability: CapabilityIndependentDissent},
				},
				FamilyDiversity: FamilyDiversityPolicyConf{Enabled: true, Roles: []string{OMPRoleAdvisor}},
				Safety:          RoleSafetyPolicyConf{ApprovalMode: "write", IsolationMode: "auto"},
			},
		},
	}
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
