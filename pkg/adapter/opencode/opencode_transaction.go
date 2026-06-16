package opencode

import (
	"fmt"
	"os"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) buildUpdateTransactionPlan(
	oldManifest *adapter.Manifest,
	files []adapter.FileMapping,
) (adapter.TransactionPlan, *adapter.PlatformFiles) {
	finalFiles := make([]adapter.FileMapping, 0, len(files))
	for _, file := range files {
		action := adapter.ResolveAction(a.root, file.TargetPath, file.OverwritePolicy, oldManifest)
		if action == adapter.ActionSkip {
			continue
		}
		finalFiles = append(finalFiles, file)
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: adapter.Checksum(fmt.Sprintf("%d", len(finalFiles))),
	}
	diff := adapter.BuildManifestDiff(oldManifest, files, opencodePruneRoots())
	removes := adapter.TransactionRemovesFromManifestDiff(diff, false)

	return adapter.TransactionPlan{
		Writes:   adapter.TransactionWritesFromFiles(finalFiles, opencodeFileMode),
		Removes:  adapter.FilterUnsupportedRootGitHookRemoves(a.root, removes),
		Manifest: adapter.ManifestFromFiles(adapterName, pf),
	}, pf
}

func opencodeFileMode(path string) os.FileMode {
	if adapter.IsRootGitHookPath(path) || isExecutablePath(path) {
		return 0755
	}
	return 0644
}
