package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
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

func (a *Adapter) cleanupStaleManagedSurfaces(oldManifest *adapter.Manifest, files []adapter.FileMapping, backupDir *string) error {
	if oldManifest == nil {
		return nil
	}
	diff := adapter.BuildManifestDiff(oldManifest, files, codexPruneRoots())
	return adapter.PruneManagedPaths(a.root, diff.Prune, backupDir)
}

func codexPruneRoots() []string {
	return []string{
		filepath.ToSlash(filepath.Join(".codex", "skills")),
		filepath.ToSlash(filepath.Join(".autopus", "plugins", "auto", "skills")),
	}
}
