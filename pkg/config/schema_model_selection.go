package config

import "fmt"

func (c *HarnessConfig) validateModelSelectionConfig() error {
	if !c.Quality.IsValidSupervisorModelPolicy() {
		return fmt.Errorf(
			"quality.supervisor_model_policy %q is invalid: must be inherit or quality",
			c.Quality.SupervisorModelPolicy,
		)
	}
	validModelTiers := map[string]bool{"opus": true, "sonnet": true, "haiku": true}
	for presetName, preset := range c.Quality.Presets {
		for agentName, tier := range preset.Agents {
			if !validModelTiers[tier] {
				return fmt.Errorf("quality.presets[%s].agents[%s]: unknown model tier %q", presetName, agentName, tier)
			}
		}
	}
	for providerName, provider := range c.Orchestra.Providers {
		if provider.ModelPolicy != "" && provider.ModelPolicy != ProviderModelPolicyQuality && provider.ModelPolicy != ProviderModelPolicyPinned {
			return fmt.Errorf("orchestra.providers[%s].model_policy %q is invalid: must be quality or pinned", providerName, provider.ModelPolicy)
		}
	}
	return nil
}
