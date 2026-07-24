package routing

import "testing"

func TestDefaultConfig_ClaudeComplexUsesOpus5(t *testing.T) {
	t.Parallel()

	models := DefaultConfig().Models["claude"]
	if models.Complex != "claude-opus-5" {
		t.Errorf("complex model = %q, want claude-opus-5", models.Complex)
	}
	if models.Simple != "claude-sonnet-5" {
		t.Errorf("simple model = %q, want claude-sonnet-5", models.Simple)
	}
	if models.Medium != "claude-sonnet-5" {
		t.Errorf("medium model = %q, want claude-sonnet-5", models.Medium)
	}
}
