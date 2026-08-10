package cost_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/cost"
)

func TestDefaultPricingTable_Opus5UsesCanonicalPricing(t *testing.T) {
	t.Parallel()

	table := cost.DefaultPricingTable()
	pricing, ok := table["claude-opus-5"]
	if !ok {
		t.Fatal("pricing table missing claude-opus-5")
	}
	if pricing.InputPricePerMillion != 5.0 {
		t.Errorf("input price = %.2f, want 5.00", pricing.InputPricePerMillion)
	}
	if pricing.OutputPricePerMillion != 25.0 {
		t.Errorf("output price = %.2f, want 25.00", pricing.OutputPricePerMillion)
	}
	if _, ok := table["opus"]; ok {
		t.Error("dynamic alias opus must not have deterministic pricing")
	}
}

func TestQualityModeToModels_Opus5IsDefaultForUltraAndStrategicRoles(t *testing.T) {
	t.Parallel()

	ultra := cost.QualityModeToModels("ultra")
	for _, role := range []string{
		"planner", "architect", "executor", "tester", "reviewer", "validator",
		"test_scaffold", "annotator", "security_auditor",
	} {
		if got := ultra[role]; got != "claude-opus-5" {
			t.Errorf("ultra/%s = %q, want claude-opus-5", role, got)
		}
	}

	// balanced executor and security_auditor are opus now. Both were pinned to
	// sonnet here while cost carried its own tier table; the balanced quality
	// preset promoted them (executor on the PR #151 measurement), and cost now
	// derives from that preset, so the promotion finally reaches cost.
	wantBalanced := map[string]string{
		"planner":          "claude-opus-5",
		"architect":        "claude-opus-5",
		"executor":         "claude-opus-5",
		"security_auditor": "claude-opus-5",
		"tester":           "claude-sonnet-5",
		"reviewer":         "claude-sonnet-5",
		"validator":        "claude-sonnet-5",
		"test_scaffold":    "claude-sonnet-5",
		"annotator":        "claude-sonnet-5",
	}
	balanced := cost.QualityModeToModels("balanced")
	for role, want := range wantBalanced {
		if got := balanced[role]; got != want {
			t.Errorf("balanced/%s = %q, want %q", role, got, want)
		}
	}
}

func TestDefaultPricingTable_Opus48RemainsSelectable(t *testing.T) {
	t.Parallel()

	if _, ok := cost.DefaultPricingTable()["claude-opus-4-8"]; !ok {
		t.Error("legacy claude-opus-4-8 pricing must remain available")
	}
}
