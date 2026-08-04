package config

import (
	"strings"
	"testing"
)

func TestOMPContextPolicy_ValidationReasonCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*OMPContextPolicyConf)
		code   string
	}{
		{"missing profile", func(p *OMPContextPolicyConf) { p.Profile = "missing" }, "profile_unknown"},
		{"invalid history", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.HistoryMode = "turbo" }), "history_mode_invalid"},
		{"invalid memory", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MemoryMode = "cache" }), "memory_mode_invalid"},
		{"memory active", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MemoryMode = "active" }), "memory_active_forbidden"},
		{"active fallback", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.Fallback = "" }), "active_fallback_required"},
		{"active capability", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.CapabilityPolicy = "" }), "active_capability_policy_required"},
		{"active runtime root", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.RuntimeRootPolicy = "" }), "active_runtime_root_required"},
		{"target lower bound", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.HistoryTargetTokens = 127 }), "history_target_out_of_bounds"},
		{"target upper bound", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.HistoryTargetTokens = 32769 }), "history_target_out_of_bounds"},
		{"ttl lower bound", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MemoryTTLSeconds = 59 }), "memory_ttl_out_of_bounds"},
		{"ttl upper bound", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MemoryTTLSeconds = 604801 }), "memory_ttl_out_of_bounds"},
		{"namespace", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MemoryNamespace = "../user" }), "memory_namespace_invalid"},
		{"namespace upper bound", mutateOMPContextProfile(func(p *OMPContextProfileConf) {
			p.MemoryNamespace = "a" + strings.Repeat("x", MaxOMPContextMemoryNamespaceLength)
		}), "memory_namespace_invalid"},
		{"fallback", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.Fallback = "continue" }), "fallback_invalid"},
		{"capability", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.CapabilityPolicy = "version_only" }), "capability_policy_invalid"},
		{"user root", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.RuntimeRootPolicy = "user_owned" }), "user_root_claim_forbidden"},
		{"global runtime root", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.RuntimeRootPolicy = "global" }), "global_mutation_claim_forbidden"},
		{"global mutation", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MutationScope = "global" }), "global_mutation_claim_forbidden"},
		{"user mutation", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MutationScope = "user_root" }), "user_root_claim_forbidden"},
		{"runtime root policy", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.RuntimeRootPolicy = "shared" }), "runtime_root_policy_invalid"},
		{"mutation scope", mutateOMPContextProfile(func(p *OMPContextProfileConf) { p.MutationScope = "project" }), "mutation_scope_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validActiveOMPContextPolicy()
			tt.mutate(&policy)
			if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.code)
			}
		})
	}
}

func TestValidateOMPContextPolicy_CanonicalEntryPoint(t *testing.T) {
	t.Parallel()

	if err := ValidateOMPContextPolicy(validActiveOMPContextPolicy()); err != nil {
		t.Fatalf("canonical validator rejected valid policy: %v", err)
	}
}

func TestOMPContextPolicy_MemoryShadow_RequiresNamespace(t *testing.T) {
	t.Parallel()

	policy := OMPContextPolicyConf{
		Profile: "memory",
		Profiles: map[string]OMPContextProfileConf{
			"memory": {MemoryMode: OMPContextMemoryShadow},
		},
	}
	if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), "memory_namespace_required") {
		t.Fatalf("Validate() error = %v", err)
	}
	profile := policy.Profiles["memory"]
	profile.MemoryNamespace = "workspace.spec.role"
	policy.Profiles["memory"] = profile
	if err := policy.Validate(); err != nil {
		t.Fatalf("valid memory shadow profile rejected: %v", err)
	}
	_, effective, ok, err := policy.SelectedOMPContextProfile()
	if err != nil || !ok || effective.MemoryTTLSeconds != DefaultOMPContextMemoryTTLSeconds {
		t.Fatalf("selected memory profile = %#v, ok %v, err %v", effective, ok, err)
	}
}

func mutateOMPContextProfile(mutate func(*OMPContextProfileConf)) func(*OMPContextPolicyConf) {
	return func(policy *OMPContextPolicyConf) {
		profile := policy.Profiles[policy.Profile]
		mutate(&profile)
		policy.Profiles[policy.Profile] = profile
	}
}
