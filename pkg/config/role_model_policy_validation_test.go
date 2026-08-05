package config

import (
	"strings"
	"testing"
)

func TestRoleModelPolicy_Helpers_FailClosedBranches(t *testing.T) {
	t.Parallel()

	if got := (RoleModelPolicyConf{}).EffectiveVersion(); got != RoleModelPolicyVersionV1 {
		t.Fatalf("zero effective version = %q", got)
	}
	if _, _, ok := (RoleModelPolicyConf{}).SelectedRoleModelProfile(); ok {
		t.Fatal("zero policy unexpectedly selected a profile")
	}
	missing := RoleModelPolicyConf{Profile: "missing", Profiles: map[string]RoleModelProfileConf{}}
	if name, _, ok := missing.SelectedRoleModelProfile(); ok || name != "missing" {
		t.Fatalf("missing selected profile = %q, %v", name, ok)
	}
	if _, err := OMPNativeRoleCapability("future"); err == nil || !strings.Contains(err.Error(), "role_unknown") {
		t.Fatalf("unknown native role error = %v", err)
	}
	if err := ValidateRoleCapabilityPair("future", CapabilityCodingToolUse); err == nil || !strings.Contains(err.Error(), "role_unknown") {
		t.Fatalf("unknown pair role error = %v", err)
	}
	exactAgents := mapKeys(OMPAgentRoleMapping())
	if err := ValidateOMPAgentRoleSet(exactAgents[:len(exactAgents)-1]); err == nil || !strings.Contains(err.Error(), "agent_role_missing") {
		t.Fatalf("missing agent error = %v", err)
	}
	if err := ValidateOMPAgentRoleSet(append(exactAgents, exactAgents[0])); err == nil || !strings.Contains(err.Error(), "agent_role_duplicate") {
		t.Fatalf("duplicate agent error = %v", err)
	}
	for _, thinking := range []string{"off", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto"} {
		if !IsOMPNativeThinkingLevel(thinking) {
			t.Fatalf("native thinking level rejected: %s", thinking)
		}
	}
	if IsOMPNativeThinkingLevel("turbo") {
		t.Fatal("unknown thinking level accepted")
	}
}

func TestRoleModelPolicy_Validation_RejectsInvalidProfileFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*RoleModelPolicyConf)
		code   string
	}{
		{"version", func(p *RoleModelPolicyConf) { p.Version = "v2" }, "policy_version_unknown"},
		{"profile name", func(p *RoleModelPolicyConf) {
			profile := p.Profiles["p1"]
			delete(p.Profiles, "p1")
			p.Profile = "bad name"
			p.Profiles[p.Profile] = profile
		}, "profile_name_invalid"},
		{"config mode", mutateProfile(func(p *RoleModelProfileConf) { p.ConfigMode = "merge" }), "config_mode_invalid"},
		{"missing capability", mutateProfile(func(p *RoleModelProfileConf) { delete(p.Capabilities, CapabilityVisionDesign) }), "capability_missing"},
		{"no candidates", mutateProfile(func(p *RoleModelProfileConf) {
			p.Capabilities[CapabilityVisionDesign] = RoleCapabilityRouteConf{Required: true}
		}), "candidates_required"},
		{"bad degraded action", mutateProfile(func(p *RoleModelProfileConf) {
			route := p.Capabilities[CapabilityVisionDesign]
			route.DegradedAction = "guess"
			p.Capabilities[CapabilityVisionDesign] = route
		}), "degraded_action_invalid"},
		{"empty selector components", mutateProfile(func(p *RoleModelProfileConf) {
			route := p.Capabilities[CapabilityVisionDesign]
			route.Candidates[0].Selector = "/"
			p.Capabilities[CapabilityVisionDesign] = route
		}), "selector_invalid"},
		{"bad candidate metadata", mutateProfile(func(p *RoleModelProfileConf) {
			route := p.Capabilities[CapabilityVisionDesign]
			route.Candidates[0].Thinking = "high value"
			p.Capabilities[CapabilityVisionDesign] = route
		}), "metadata_invalid"},
		{"unknown thinking", mutateProfile(func(p *RoleModelProfileConf) {
			route := p.Capabilities[CapabilityVisionDesign]
			route.Candidates[0].Thinking = "turbo"
			p.Capabilities[CapabilityVisionDesign] = route
		}), "metadata_invalid"},
		{"bad diversity role", mutateProfile(func(p *RoleModelProfileConf) {
			p.FamilyDiversity.Roles = []string{"future"}
		}), "role_unknown"},
		{"duplicate diversity role", mutateProfile(func(p *RoleModelProfileConf) {
			p.FamilyDiversity.Roles = []string{OMPRoleAdvisor, OMPRoleAdvisor}
		}), "role_duplicate"},
		{"bad approval", mutateProfile(func(p *RoleModelProfileConf) { p.Safety.ApprovalMode = "bad value" }), "approval_mode_invalid"},
		{"bad isolation", mutateProfile(func(p *RoleModelProfileConf) { p.Safety.IsolationMode = "bad value" }), "isolation_mode_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validRoleModelPolicyFixture()
			tt.mutate(&policy)
			if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.code)
			}
		})
	}
}

func mutateProfile(mutate func(*RoleModelProfileConf)) func(*RoleModelPolicyConf) {
	return func(policy *RoleModelPolicyConf) {
		profile := policy.Profiles[policy.Profile]
		mutate(&profile)
		policy.Profiles[policy.Profile] = profile
	}
}
