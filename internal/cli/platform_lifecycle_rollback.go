package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

type platformLifecycleSnapshot struct {
	config                harnessConfigSnapshot
	manifest              harnessConfigSnapshot
	preimages             map[string]harnessConfigSnapshot
	committedTransactions map[string]struct{}
}

// Antigravity direct writes and OMP delta-backed config need broad preimages; manifests cover other native adapters.
var platformLifecycleRootsByPlatform = map[string][]string{
	"antigravity-cli": {".agents", ".gemini"},
	"claude-code":     {},
	"codex":           {},
	"omp":             {".omp"},
	"opencode":        {},
}

var platformLifecycleAllSurfaceRoots = []string{".agents", ".autopus/plugins", ".claude", ".codex", ".gemini", ".omp", ".opencode"}
var platformLifecycleRootFiles = [...]string{".mcp.json", "AGENTS.md", "CLAUDE.md", "GEMINI.md", "opencode.json"}

func capturePlatformLifecycleSnapshot(root, platform string) (platformLifecycleSnapshot, error) {
	configSnapshot, err := captureHarnessConfigSnapshot(root)
	if err != nil {
		return platformLifecycleSnapshot{}, fmt.Errorf("autopus.yaml snapshot failed: %w", err)
	}
	manifestRel := platformManifestPath(platform)
	manifestSnapshot, err := captureLifecyclePathSnapshot(root, manifestRel)
	if err != nil {
		return platformLifecycleSnapshot{}, fmt.Errorf("%s snapshot failed: %w", manifestRel, err)
	}

	preimages, err := captureLifecycleSurfacePreimages(root, platform)
	if err != nil {
		return platformLifecycleSnapshot{}, err
	}
	snapshot := platformLifecycleSnapshot{
		config:                configSnapshot,
		manifest:              manifestSnapshot,
		preimages:             preimages,
		committedTransactions: make(map[string]struct{}),
	}
	manifest, err := adapter.LoadManifest(root, platform)
	if err != nil {
		return platformLifecycleSnapshot{}, err
	}
	if manifest != nil {
		for path := range manifest.Files {
			rel, normalizeErr := normalizeLifecyclePath(root, path)
			if normalizeErr != nil {
				return platformLifecycleSnapshot{}, normalizeErr
			}
			fileSnapshot, captureErr := captureLifecyclePathSnapshot(root, rel)
			if captureErr != nil {
				return platformLifecycleSnapshot{}, fmt.Errorf("%s snapshot failed: %w", rel, captureErr)
			}
			snapshot.preimages[rel] = fileSnapshot
		}
	}
	transactions, err := adapter.ListCommittedTransactions(root)
	if err != nil {
		return platformLifecycleSnapshot{}, err
	}
	for _, transaction := range transactions {
		if transaction.Platform == platform {
			snapshot.committedTransactions[lifecycleTransactionKey(transaction)] = struct{}{}
		}
	}
	return snapshot, nil
}

func restorePlatformLifecycleSnapshot(
	root string,
	platform string,
	snapshot platformLifecycleSnapshot,
	operationPaths []string,
) error {
	manifestRel := platformManifestPath(platform)
	paths := make(map[string]struct{}, len(snapshot.preimages)+len(operationPaths))
	for path := range snapshot.preimages {
		paths[path] = struct{}{}
	}
	currentManifest, manifestErr := adapter.LoadManifest(root, platform)
	if currentManifest != nil {
		for path := range currentManifest.Files {
			operationPaths = append(operationPaths, path)
		}
	}
	for _, path := range operationPaths {
		rel, err := normalizeLifecyclePath(root, path)
		if err != nil {
			manifestErr = errors.Join(manifestErr, err)
			continue
		}
		if rel != manifestRel {
			paths[rel] = struct{}{}
		}
	}

	transactionPaths, transactionErr := rollbackLifecycleTransactions(root, platform, snapshot)
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	var surfaceErr error
	for _, path := range ordered {
		fileSnapshot, existedBefore := snapshot.preimages[path]
		if !existedBefore {
			if transactionPaths[path] {
				continue
			}
			fileSnapshot = harnessConfigSnapshot{}
		}
		if err := restoreLifecyclePathSnapshot(root, path, fileSnapshot); err != nil {
			surfaceErr = errors.Join(surfaceErr, fmt.Errorf("restore %s: %w", path, err))
		}
	}
	manifestRestoreErr := restoreLifecyclePathSnapshot(root, manifestRel, snapshot.manifest)
	configRestoreErr := restoreLifecyclePathSnapshot(root, "autopus.yaml", snapshot.config)
	return errors.Join(manifestErr, transactionErr, surfaceErr, manifestRestoreErr, configRestoreErr)
}

func rollbackLifecycleTransactions(
	root string,
	platform string,
	snapshot platformLifecycleSnapshot,
) (map[string]bool, error) {
	transactions, err := adapter.ListCommittedTransactions(root)
	if err != nil {
		return nil, err
	}
	pending := make([]*adapter.TransactionJournal, 0)
	for _, transaction := range transactions {
		if transaction.Platform != platform {
			continue
		}
		if _, existed := snapshot.committedTransactions[lifecycleTransactionKey(transaction)]; !existed {
			pending = append(pending, transaction)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].CreatedAt == pending[j].CreatedAt {
			return pending[i].ID > pending[j].ID
		}
		return pending[i].CreatedAt > pending[j].CreatedAt
	})

	restored := make(map[string]bool)
	var rollbackErr error
	for _, transaction := range pending {
		if err := adapter.RollbackTransactionJournal(root, transaction); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		for _, entry := range transaction.Entries {
			rel, normalizeErr := normalizeLifecyclePath(root, entry.Path)
			if normalizeErr != nil {
				rollbackErr = errors.Join(rollbackErr, normalizeErr)
				continue
			}
			restored[rel] = true
		}
	}
	return restored, rollbackErr
}

func captureLifecycleSurfacePreimages(root, platform string) (map[string]harnessConfigSnapshot, error) {
	preimages := make(map[string]harnessConfigSnapshot)
	surfaceRoots, knownPlatform := platformLifecycleRootsByPlatform[platform]
	if !knownPlatform {
		surfaceRoots = platformLifecycleAllSurfaceRoots
	}
	for _, rel := range platformLifecycleRootFiles {
		snapshot, err := captureLifecyclePathSnapshot(root, rel)
		if err != nil {
			return nil, fmt.Errorf("%s snapshot failed: %w", rel, err)
		}
		if snapshot.exists {
			preimages[rel] = snapshot
		}
	}
	for _, relRoot := range surfaceRoots {
		pathRoot := filepath.Join(root, filepath.FromSlash(relRoot))
		err := filepath.Walk(pathRoot, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() || !info.Mode().IsRegular() {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			snapshot, err := captureLifecyclePathSnapshot(root, rel)
			if err == nil {
				preimages[filepath.ToSlash(rel)] = snapshot
			}
			return err
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("%s surface snapshot failed: %w", relRoot, err)
		}
	}
	return preimages, nil
}

func captureLifecyclePathSnapshot(root, rel string) (harnessConfigSnapshot, error) {
	path, err := adapter.SafeWorkspacePath(root, rel)
	if err != nil {
		return harnessConfigSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return harnessConfigSnapshot{}, nil
	}
	if err != nil {
		return harnessConfigSnapshot{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return harnessConfigSnapshot{}, err
	}
	return harnessConfigSnapshot{data: data, mode: info.Mode().Perm(), exists: true}, nil
}

func restoreLifecyclePathSnapshot(root, rel string, snapshot harnessConfigSnapshot) error {
	current, err := captureLifecyclePathSnapshot(root, rel)
	if err != nil {
		return err
	}
	if current.exists == snapshot.exists && current.mode == snapshot.mode &&
		bytes.Equal(current.data, snapshot.data) {
		return nil
	}
	path, err := adapter.SafeWorkspacePath(root, rel)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if !snapshot.exists {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, snapshot.data, snapshot.mode)
}

func normalizeLifecyclePath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		path = rel
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if _, err := adapter.SafeWorkspacePath(root, clean); err != nil {
		return "", err
	}
	return filepath.ToSlash(clean), nil
}

func platformManifestPath(platform string) string {
	return filepath.ToSlash(filepath.Join(".autopus", platform+"-manifest.json"))
}

func lifecycleTransactionKey(transaction *adapter.TransactionJournal) string {
	return transaction.Platform + "\x00" + transaction.ID
}

func platformFilePaths(files *adapter.PlatformFiles) []string {
	if files == nil {
		return nil
	}
	paths := make([]string, 0, len(files.Files))
	for _, file := range files.Files {
		paths = append(paths, file.TargetPath)
	}
	return paths
}

func lifecycleSaveError(saveErr, rollbackErr error) error {
	if rollbackErr == nil {
		return saveErr
	}
	return fmt.Errorf("%w; platform lifecycle rollback failed: %v", saveErr, rollbackErr)
}
