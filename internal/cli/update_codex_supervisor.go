package cli

import (
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

const legacyCodexSupervisorPreviewReason = "legacy Codex supervisor model would migrate to user-default inheritance"

func migrateLegacyCodexSupervisorPolicy(dir string, cfg *config.HarnessConfig) (bool, error) {
	inspection, err := codex.InspectLegacySupervisorModel(dir, cfg)
	if err != nil {
		return false, err
	}
	if !inspection.Migratable {
		return false, nil
	}
	cfg.Quality.SupervisorModelPolicy = config.SupervisorModelPolicyInherit
	return true, nil
}
