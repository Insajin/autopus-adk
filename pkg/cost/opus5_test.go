package cost_test

import (
	"reflect"
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
	wantUltra := map[string]string{
		"planner":          "claude-opus-5",
		"architect":        "claude-opus-5",
		"executor":         "claude-opus-5",
		"tester":           "claude-opus-5",
		"reviewer":         "claude-opus-5",
		"validator":        "claude-opus-5",
		"test_scaffold":    "claude-opus-5",
		"annotator":        "claude-opus-5",
		"security_auditor": "claude-opus-5",
	}
	if len(ultra) != 9 || !reflect.DeepEqual(ultra, wantUltra) {
		t.Fatalf("ultra model map = %#v, want exact 9-role map %#v", ultra, wantUltra)
	}

	balanced := cost.QualityModeToModels("balanced")
	wantBalanced := map[string]string{
		"planner":          "claude-opus-5",
		"architect":        "claude-opus-5",
		"executor":         "claude-sonnet-5",
		"tester":           "claude-sonnet-5",
		"reviewer":         "claude-sonnet-5",
		"validator":        "claude-sonnet-5",
		"test_scaffold":    "claude-sonnet-5",
		"annotator":        "claude-sonnet-5",
		"security_auditor": "claude-sonnet-5",
	}
	if len(balanced) != 9 || !reflect.DeepEqual(balanced, wantBalanced) {
		t.Fatalf("balanced model map = %#v, want exact 9-role map %#v", balanced, wantBalanced)
	}
}

func TestDefaultPricingTable_Opus48RemainsSelectable(t *testing.T) {
	t.Parallel()

	if _, ok := cost.DefaultPricingTable()["claude-opus-4-8"]; !ok {
		t.Error("legacy claude-opus-4-8 pricing must remain available")
	}
}
