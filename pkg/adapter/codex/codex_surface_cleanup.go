package codex

import (
	"fmt"
	"os"
	"path/filepath"
)

func (a *Adapter) cleanupDeprecatedSurface() error {
	return a.cleanupPluginWorkflowShims()
}

func (a *Adapter) cleanupPluginWorkflowShims() error {
	for _, spec := range workflowSpecs {
		if spec.Name == "auto" {
			continue
		}
		target := filepath.Join(a.root, ".autopus", "plugins", "auto", "skills", spec.Name)
		if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deprecated Codex plugin shim 제거 실패 %s: %w", target, err)
		}
	}
	return nil
}
