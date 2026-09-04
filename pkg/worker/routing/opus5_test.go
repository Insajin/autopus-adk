package routing

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestDefaultConfig_ClaudeUsesFourTierRouting(t *testing.T) {
	t.Parallel()

	models := DefaultConfig().Models["claude"]
	if models.Complex != config.ClaudeFableModel {
		t.Errorf("complex model = %q, want %s", models.Complex, config.ClaudeFableModel)
	}
	if models.Medium != config.ClaudeOpusModel {
		t.Errorf("medium model = %q, want %s", models.Medium, config.ClaudeOpusModel)
	}
	if models.Simple != config.ClaudeSonnetModel {
		t.Errorf("simple model = %q, want %s", models.Simple, config.ClaudeSonnetModel)
	}
}
