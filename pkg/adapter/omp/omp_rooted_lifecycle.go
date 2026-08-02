package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// @AX:WARN [AUTO]: missing-root creation contains 13 fail-closed identity and lifecycle branches.
// @AX:REASON [AUTO]: the existing ancestor, created child, pathname binding, and every opened root must remain coupled.
func openOrCreateOMPRootedWorkspace(
	path string,
	afterCreate func(),
) (*ompRootedWorkspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve OMP workspace: %w", err)
	}
	if _, err := os.Lstat(abs); err == nil {
		return openOMPRootedWorkspace(abs)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect OMP workspace root: %w", err)
	}

	ancestor := filepath.Dir(abs)
	for {
		info, inspectErr := os.Lstat(ancestor)
		if inspectErr == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return nil, fmt.Errorf("OMP workspace ancestor must be a real directory")
			}
			break
		}
		if !errors.Is(inspectErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect OMP workspace ancestor: %w", inspectErr)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return nil, fmt.Errorf("OMP workspace has no real ancestor")
		}
		ancestor = parent
	}

	parentWorkspace, err := openOMPRootedWorkspace(ancestor)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(ancestor, abs)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("resolve OMP workspace relative path: %w", err), parentWorkspace.Close())
	}
	if _, err := cleanOMPRootedPath(relative); err != nil {
		return nil, errors.Join(err, parentWorkspace.Close())
	}
	if err := parentWorkspace.root.MkdirAll(relative, 0o755); err != nil {
		return nil, errors.Join(fmt.Errorf("create OMP workspace root: %w", err), parentWorkspace.Close())
	}
	if afterCreate != nil {
		afterCreate()
	}
	root, err := parentWorkspace.root.OpenRoot(relative)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open created OMP workspace: %w", err), parentWorkspace.Close())
	}
	opened, openErr := root.Stat(".")
	bound, bindErr := os.Lstat(abs)
	closeErr := parentWorkspace.Close()
	if openErr != nil || bindErr != nil || bound.Mode()&os.ModeSymlink != 0 ||
		!bound.IsDir() || !os.SameFile(opened, bound) || closeErr != nil {
		return nil, errors.Join(fmt.Errorf("OMP workspace changed while creating"), root.Close(), closeErr)
	}
	return &ompRootedWorkspace{root: root, path: abs, info: opened}, nil
}

func (a *Adapter) prepareConfigMappingAt(
	workspace *ompRootedWorkspace,
	_ *config.HarnessConfig,
) (adapter.FileMapping, error) {
	data, _, err := workspace.readFile(configFile, 4<<20)
	if errors.Is(err, fs.ErrNotExist) {
		data = nil
	} else if err != nil {
		return adapter.FileMapping{}, fmt.Errorf("%s 읽기 실패: %w", configFile, err)
	}
	configDoc, err := mergeOMPConfigDocument(string(data))
	if err != nil {
		return adapter.FileMapping{}, err
	}
	return adapter.FileMapping{
		TargetPath: configFile, OverwritePolicy: adapter.OverwriteMarker,
		Checksum: adapter.Checksum(configDoc), Content: []byte(configDoc),
	}, nil
}

func (a *Adapter) fileModeResolverAt(workspace *ompRootedWorkspace) func(string) os.FileMode {
	return func(path string) os.FileMode {
		if !isOwnerOnlyOMPModelPath(path) {
			return 0o644
		}
		if path != configFile {
			return 0o600
		}
		if info, err := workspace.lstat(path); err == nil {
			return info.Mode().Perm()
		}
		return ompConfigFileMode
	}
}

func resolveOMPUpdateActionAt(
	workspace *ompRootedWorkspace,
	file adapter.FileMapping,
	old *adapter.Manifest,
) (adapter.UpdateAction, error) {
	if file.OverwritePolicy == adapter.OverwriteMarker {
		return adapter.ActionOverwrite, nil
	}
	_, err := workspace.lstat(file.TargetPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect OMP update target %s: %w", file.TargetPath, err)
	}
	exists := err == nil
	if old == nil {
		if exists {
			return adapter.ActionOverwrite, nil
		}
		return adapter.ActionCreate, nil
	}
	previous, managed := old.Files[file.TargetPath]
	if !exists {
		if managed {
			return adapter.ActionSkip, nil
		}
		return adapter.ActionCreate, nil
	}
	if managed {
		data, _, readErr := workspace.readFile(file.TargetPath, 4<<20)
		if readErr != nil {
			return "", fmt.Errorf("read OMP update target %s: %w", file.TargetPath, readErr)
		}
		if adapter.Checksum(string(data)) == previous.Checksum {
			return adapter.ActionOverwrite, nil
		}
	}
	return adapter.ActionBackup, nil
}

func (a *Adapter) buildUpdateTransactionPlanAt(
	workspace *ompRootedWorkspace,
	oldManifest *adapter.Manifest,
	files []adapter.FileMapping,
	cfg *config.HarnessConfig,
) (adapter.TransactionPlan, *adapter.PlatformFiles, error) {
	finalFiles := make([]adapter.FileMapping, 0, len(files))
	for _, file := range files {
		action, err := resolveOMPUpdateActionAt(workspace, file, oldManifest)
		if err != nil {
			return adapter.TransactionPlan{}, nil, err
		}
		if action != adapter.ActionSkip {
			finalFiles = append(finalFiles, file)
		}
	}
	pf := &adapter.PlatformFiles{
		Files: finalFiles, Checksum: adapter.Checksum(fmt.Sprintf("%d", len(finalFiles))),
	}
	diff := adapter.BuildManifestDiff(oldManifest, files, PruneRoots(cfg))
	return adapter.TransactionPlan{
		Writes:   adapter.TransactionWritesFromFiles(finalFiles, a.fileModeResolverAt(workspace)),
		Removes:  adapter.TransactionRemovesFromManifestDiff(diff, false),
		Manifest: adapter.ManifestFromFiles(adapterName, pf),
	}, pf, nil
}
