package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOMPContextPolicy_ZeroValue_DoesNotOptInOrChangeDefaultYAML(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("zero-context")
	before := *cfg
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal default config: %v", err)
	}
	if strings.Contains(string(data), "omp_context_policy:") {
		t.Fatalf("zero context policy changed default YAML:\n%s", data)
	}
	if cfg.OMPContextPolicy.IsOptedIn() {
		t.Fatal("zero context policy unexpectedly opted in")
	}
	if _, _, ok, err := cfg.OMPContextPolicy.SelectedOMPContextProfile(); err != nil || ok {
		t.Fatalf("zero selected profile = ok %v, err %v", ok, err)
	}
	if !reflect.DeepEqual(*cfg, before) {
		t.Fatal("effective lookup mutated default config")
	}
}

func TestOMPContextPolicy_ExplicitProfile_AppliesEffectiveDefaultsWithoutMutation(t *testing.T) {
	t.Parallel()

	policy := OMPContextPolicyConf{
		Profile:  "safe",
		Profiles: map[string]OMPContextProfileConf{"safe": {}},
	}
	before := policy.Profiles["safe"]
	name, got, ok, err := policy.SelectedOMPContextProfile()
	if err != nil {
		t.Fatalf("select profile: %v", err)
	}
	if !ok || name != "safe" {
		t.Fatalf("selected profile = %q, %v", name, ok)
	}
	want := OMPContextProfileConf{
		HistoryMode:         OMPContextHistoryShadow,
		MemoryMode:          OMPContextMemoryOff,
		HistoryTargetTokens: DefaultOMPContextHistoryTargetTokens,
		MemoryTTLSeconds:    DefaultOMPContextMemoryTTLSeconds,
		Fallback:            OMPContextFallbackCanonicalFull,
		CapabilityPolicy:    OMPContextCapabilityProbeRequired,
		RuntimeRootPolicy:   OMPContextRuntimeIsolatedTaskOwned,
		MutationScope:       OMPContextMutationSessionOverlay,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effective profile mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if raw := policy.Profiles["safe"]; !reflect.DeepEqual(raw, before) {
		t.Fatalf("selection mutated raw profile: %#v", raw)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("defaulted profile invalid: %v", err)
	}
}

func TestOMPContextPolicy_HistoryAndMemoryModes_AreIndependent(t *testing.T) {
	t.Parallel()

	policy := validActiveOMPContextPolicy()
	_, got, ok, err := policy.SelectedOMPContextProfile()
	if err != nil || !ok {
		t.Fatalf("select active profile: ok %v, err %v", ok, err)
	}
	if got.HistoryMode != OMPContextHistoryActive || got.MemoryMode != OMPContextMemoryOff {
		t.Fatalf("effective modes = history %q, memory %q", got.HistoryMode, got.MemoryMode)
	}
	data, err := yaml.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	var roundTrip OMPContextPolicyConf
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	if !reflect.DeepEqual(roundTrip, policy) {
		t.Fatalf("policy round-trip mismatch\ngot:  %#v\nwant: %#v", roundTrip, policy)
	}
}

func validActiveOMPContextPolicy() OMPContextPolicyConf {
	return OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]OMPContextProfileConf{
			"active": {
				HistoryMode:         OMPContextHistoryActive,
				MemoryMode:          OMPContextMemoryOff,
				HistoryTargetTokens: 1000,
				Fallback:            OMPContextFallbackCanonicalFull,
				CapabilityPolicy:    OMPContextCapabilityProbeRequired,
				RuntimeRootPolicy:   OMPContextRuntimeIsolatedTaskOwned,
				MutationScope:       OMPContextMutationSessionOverlay,
			},
		},
	}
}
