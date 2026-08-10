package cost_test

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/cost"
)

// TestQualityModeToModels_DerivesFromConfigPresets pins the cost model map to
// the quality presets in pkg/config. Expectations are read out of the presets
// instead of restated as slug literals on purpose: cost used to carry its own
// tier table and the two silently diverged (balanced executor was sonnet here
// while the preset said opus). This test fails the moment they diverge again.
func TestQualityModeToModels_DerivesFromConfigPresets(t *testing.T) {
	t.Parallel()

	presets := config.DefaultFullConfig("").Quality.Presets
	if len(presets) == 0 {
		t.Fatal("default quality presets are empty")
	}

	for mode, preset := range presets {
		models := cost.QualityModeToModels(mode)
		if models == nil {
			t.Fatalf("%s: preset exists but QualityModeToModels returned nil", mode)
		}

		for _, agent := range config.CanonicalAgentNames() {
			tier, assigned := preset.Agents[agent]
			if !assigned {
				t.Errorf("%s: preset assigns no tier to canonical agent %q", mode, agent)
				continue
			}
			got, present := models[agent]
			if !present {
				t.Errorf("%s: canonical agent %q missing from the model map", mode, agent)
				continue
			}
			if want := config.ClaudeModelForTier(tier); got != want {
				t.Errorf("%s/%s = %q, want %q (preset tier %q)", mode, agent, got, want, tier)
			}

			// Workflow phase ids spell multi-word roles with underscores; both
			// spellings must land on the same tier.
			alias := strings.ReplaceAll(agent, "-", "_")
			if alias == agent {
				continue
			}
			if got, want := models[alias], models[agent]; got != want {
				t.Errorf("%s/%s (underscore alias) = %q, want %q", mode, alias, got, want)
			}
		}
	}
}

// TestQualityModeToModels_WorkflowOnlyRolesResolve guards the phase roles that
// are not canonical source agents. An empty model is rejected by the generated
// workflow JS whitelist, so every phase role must resolve to a slug.
func TestQualityModeToModels_WorkflowOnlyRolesResolve(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"ultra", "balanced"} {
		models := cost.QualityModeToModels(mode)
		// test_scaffold is the test-scaffolding phase, so it rides the tester tier.
		got, want := models["test_scaffold"], models["tester"]
		if want == "" {
			t.Fatalf("%s: tester resolved to an empty model", mode)
		}
		if got != want {
			t.Errorf("%s/test_scaffold = %q, want the tester tier %q", mode, got, want)
		}
	}
}
