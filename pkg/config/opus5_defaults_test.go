package config

import "testing"

func TestDefaultFullConfig_PremiumRouterUsesOpus5(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	if got := cfg.Router.Tiers["premium"]; got != "claude-opus-5" {
		t.Errorf("premium router model = %q, want claude-opus-5", got)
	}
	if got := cfg.Router.Tiers["standard"]; got != "claude-sonnet-5" {
		t.Errorf("standard router model = %q, want claude-sonnet-5", got)
	}
	if got := cfg.Router.Tiers["economy"]; got != "claude-sonnet-5" {
		t.Errorf("economy router model = %q, want claude-sonnet-5", got)
	}
}
