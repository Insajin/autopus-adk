package setup

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/sigmap"
)

const signaturesDir = ".autopus/context"
const signaturesFile = "signatures.md"

func renderSignatureMapContent(projectDir string, cfg *config.HarnessConfig) ([]byte, bool, error) {
	if cfg != nil && !cfg.Context.SignatureMap {
		return nil, false, nil
	}

	sm, err := sigmap.Extract(projectDir)
	if err != nil {
		return nil, false, fmt.Errorf("extract signatures: %w", err)
	}
	if err := sigmap.CalculateFanIn(projectDir, sm); err != nil {
		return nil, false, fmt.Errorf("calculate fan-in: %w", err)
	}
	return []byte(sigmap.Render(sm)), true, nil
}
