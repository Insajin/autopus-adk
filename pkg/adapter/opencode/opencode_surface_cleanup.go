package opencode

import (
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) cleanupStaleManagedSurfaces(oldManifest *adapter.Manifest, files []adapter.FileMapping, backupDir *string) error {
	if oldManifest == nil {
		return nil
	}
	diff := adapter.BuildManifestDiff(oldManifest, files, opencodePruneRoots())
	return adapter.PruneManagedPaths(a.root, diff.Prune, backupDir)
}

func opencodePruneRoots() []string {
	return []string{
		filepath.ToSlash(filepath.Join(".agents", "skills")),
		filepath.ToSlash(filepath.Join(".opencode", "skills")),
	}
}
